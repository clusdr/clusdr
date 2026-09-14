// Package eventbus provides an in-process publish/subscribe event bus.
//
// Design constraints:
//   - Bounded channels per subscriber; slow subscribers drop events.
//   - Publish is non-blocking; the cluster never slows down for a lagging watcher.
//   - Thread-safe; Publish, Subscribe, and Unsubscribe can be called from any goroutine.
//   - Dropped events set a per-subscriber flag so Watch can emit watch.gap.
package eventbus

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/odurgut/clusdr/internal/events"
)

// Bus fans out published events to all active subscriptions.
// Create with New(); the zero value is not usable.
type Bus struct {
	mu   sync.RWMutex
	subs map[uint64]*subscription
	next atomic.Uint64 // subscription ID generator
	seq  atomic.Uint64 // global event sequence
}

type subscription struct {
	ch      chan events.Event
	done    chan struct{} // closed by Unsubscribe to signal the consumer
	dropped atomic.Bool   // set when Publish skipped this subscriber
}

// New creates a ready-to-use Bus.
func New() *Bus {
	return &Bus{subs: make(map[uint64]*subscription)}
}

// LastSeq returns the last sequence number assigned by Publish.
// 0 means no events have been published yet.
func (b *Bus) LastSeq() uint64 {
	return b.seq.Load()
}

// Publish sends e to all active subscribers. If a subscriber's channel is full
// the event is dropped for that subscriber — the bus never blocks.
// Seq and Timestamp are set by Publish if not already provided.
func (b *Bus) Publish(e events.Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	e.Seq = b.seq.Add(1)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subs {
		select {
		case sub.ch <- e:
		default:
			sub.dropped.Store(true)
		}
	}
}

// Subscription is returned by Bus.Subscribe. Read events from C; call
// Unsubscribe when done to free resources.
type Subscription struct {
	// C receives published events.
	C <-chan events.Event
	// Done is closed when Unsubscribe is called.
	// Select on it alongside ctx.Done() to detect teardown.
	Done <-chan struct{}

	id    uint64
	inner *subscription
	bus   *Bus
	once  sync.Once
	done  chan struct{} // same channel exposed as Done
}

// Subscribe registers a new subscriber. bufSize controls the depth of the
// event channel; recommended values are 16–64. Events are dropped (not
// queued) when the buffer is full.
func (b *Bus) Subscribe(bufSize int) *Subscription {
	if bufSize <= 0 {
		bufSize = 16
	}
	ch := make(chan events.Event, bufSize)
	done := make(chan struct{})
	id := b.next.Add(1)
	inner := &subscription{ch: ch, done: done}

	b.mu.Lock()
	b.subs[id] = inner
	b.mu.Unlock()

	return &Subscription{C: ch, Done: done, id: id, inner: inner, bus: b, done: done}
}

// TakeDropped reports whether any event was dropped for this subscriber since
// the previous call and clears the flag. Watch uses this to emit watch.gap.
func (s *Subscription) TakeDropped() bool {
	if s == nil || s.inner == nil {
		return false
	}
	return s.inner.dropped.Swap(false)
}

// Unsubscribe removes this subscription from the bus and closes Done.
// Safe to call multiple times; subsequent calls are no-ops.
func (s *Subscription) Unsubscribe() {
	s.once.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.subs, s.id)
		s.bus.mu.Unlock()
		close(s.done) // signal the consumer goroutine to exit
	})
}

// Drain returns all buffered events without blocking.
// Useful in tests to inspect what was published.
func (s *Subscription) Drain() []events.Event {
	var out []events.Event
	for {
		select {
		case e := <-s.C:
			out = append(out, e)
		default:
			return out
		}
	}
}
