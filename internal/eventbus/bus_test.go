package eventbus_test

import (
	"testing"
	"time"

	"github.com/odurgut/clusdr/internal/eventbus"
	"github.com/odurgut/clusdr/internal/events"
)

func TestBus_PubSubBasic(t *testing.T) {
	b := eventbus.New()
	sub := b.Subscribe(8)
	defer sub.Unsubscribe()

	b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "node-1"})

	select {
	case e := <-sub.C:
		if e.Type != events.TypeMemberJoin {
			t.Errorf("type: got %q, want %q", e.Type, events.TypeMemberJoin)
		}
		if e.Source != "node-1" {
			t.Errorf("source: got %q, want %q", e.Source, "node-1")
		}
		if e.Seq == 0 {
			t.Error("seq should be non-zero")
		}
		if e.Timestamp.IsZero() {
			t.Error("timestamp should be set")
		}
	case <-time.After(time.Second):
		t.Fatal("no event received within 1s")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	b := eventbus.New()
	s1 := b.Subscribe(4)
	s2 := b.Subscribe(4)
	defer s1.Unsubscribe()
	defer s2.Unsubscribe()

	b.Publish(events.Event{Type: events.TypeLeaderChanged, Source: "leader-1"})

	for _, sub := range []*eventbus.Subscription{s1, s2} {
		select {
		case e := <-sub.C:
			if e.Type != events.TypeLeaderChanged {
				t.Errorf("type: got %q", e.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive event")
		}
	}
}

func TestBus_SlowSubscriberDropsEvents(t *testing.T) {
	b := eventbus.New()
	slow := b.Subscribe(2) // tiny buffer
	fast := b.Subscribe(100)
	defer slow.Unsubscribe()
	defer fast.Unsubscribe()

	// Publish 10 events — slow subscriber's buffer fills and drops.
	for i := 0; i < 10; i++ {
		b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "x"})
	}

	// Fast subscriber received all 10.
	if got := len(fast.Drain()); got != 10 {
		t.Errorf("fast subscriber: got %d events, want 10", got)
	}
	// Slow subscriber received at most 2 (buffer size), cluster did not block.
	slowEvents := slow.Drain()
	if len(slowEvents) > 2 {
		t.Errorf("slow subscriber got %d events (> buffer=2)", len(slowEvents))
	}
	if !slow.TakeDropped() {
		t.Error("slow subscriber should report dropped events")
	}
	if slow.TakeDropped() {
		t.Error("TakeDropped should clear the flag")
	}
	if fast.TakeDropped() {
		t.Error("fast subscriber should not report drops")
	}
}

func TestBus_UnsubscribeStopsDelivery(t *testing.T) {
	b := eventbus.New()
	sub := b.Subscribe(8)

	b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "a"})
	sub.Unsubscribe()
	b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "b"}) // should not arrive

	// Drain only gets the first event.
	got := sub.Drain()
	for _, e := range got {
		if e.Source == "b" {
			t.Error("received event after Unsubscribe")
		}
	}

	// Done channel is closed.
	select {
	case <-sub.Done:
	case <-time.After(time.Second):
		t.Fatal("Done not closed after Unsubscribe")
	}
}

func TestBus_SequenceIsMonotonic(t *testing.T) {
	b := eventbus.New()
	sub := b.Subscribe(16)
	defer sub.Unsubscribe()

	for i := 0; i < 5; i++ {
		b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "n"})
	}

	got := sub.Drain()
	if len(got) != 5 {
		t.Fatalf("got %d events, want 5", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Seq <= got[i-1].Seq {
			t.Errorf("sequence not monotonic: [%d].Seq=%d <= [%d].Seq=%d",
				i, got[i].Seq, i-1, got[i-1].Seq)
		}
	}
}

func TestBus_PublishNonBlocking(t *testing.T) {
	b := eventbus.New()
	// Subscribe with zero buffer — every publish should drop but never block.
	_ = b.Subscribe(0) // bufSize 0 → defaults to 16 internally
	_ = b.Subscribe(1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			b.Publish(events.Event{Type: events.TypeMemberLeft, Source: "x"})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked for > 2s")
	}
}

func TestBus_LastSeq(t *testing.T) {
	b := eventbus.New()
	if b.LastSeq() != 0 {
		t.Errorf("LastSeq before publish: got %d, want 0", b.LastSeq())
	}
	b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "a"})
	b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "b"})
	if b.LastSeq() != 2 {
		t.Errorf("LastSeq: got %d, want 2", b.LastSeq())
	}
}
