// Package locks is the in-memory lock table mutated only by the Raft FSM.
package locks

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
	"unicode"

	"github.com/durguto/clusdr/internal/events"
)

const (
	MaxNameLen = 128
	MaxLocks   = 4096

	// DefaultTTL is the grant lifetime when the client omits ttl_ms.
	// Production: a dead holder must not hold forever.
	DefaultTTL = 15 * time.Second
	MinTTL     = time.Millisecond
	MaxTTL     = 24 * time.Hour

	// DefaultExpireInterval is how often the leader scans for expired locks.
	DefaultExpireInterval = 100 * time.Millisecond
)

// ClampTTL maps a requested TTL onto [MinTTL, MaxTTL].
// requested <= 0 uses fallback; fallback <= 0 uses DefaultTTL.
func ClampTTL(requested, fallback time.Duration) time.Duration {
	if requested <= 0 {
		requested = fallback
	}
	if requested <= 0 {
		requested = DefaultTTL
	}
	if requested < MinTTL {
		return MinTTL
	}
	if requested > MaxTTL {
		return MaxTTL
	}
	return requested
}

// Record is a held lock.
type Record struct {
	Name       string
	Holder     string
	Token      uint64
	AcquiredAt time.Time
	// Deadline is when the grant expires. Zero means no expiry (tests / old snapshots).
	Deadline time.Time `json:",omitempty"`
	// TTL is the last applied lifetime; Renew with ttl_ms=0 reuses it.
	TTL time.Duration `json:",omitempty"`
}

// Table holds exclusive locks. Safe for concurrent use.
// Writes must come from the Raft FSM so every node sees the same table.
type Table struct {
	mu    sync.Mutex
	wait  *sync.Cond
	locks map[string]Record
	next  uint64

	// Emit is an optional hook after lock.expired (FSM apply on every node).
	Emit func(events.Event)
}

// New creates an empty lock table.
func New() *Table {
	t := &Table{locks: make(map[string]Record)}
	t.wait = sync.NewCond(&t.mu)
	return t
}

// Acquire tries to take name for holder. Same holder already holding is
// idempotent (same token) and refreshes Deadline/TTL. Does not block.
func (t *Table) Acquire(name, holder string, deadline time.Time, ttl time.Duration) (token uint64, acquired bool, err error) {
	if err := ValidName(name); err != nil {
		return 0, false, err
	}
	if holder == "" {
		return 0, false, fmt.Errorf("lock holder is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if rec, ok := t.locks[name]; ok {
		if rec.Holder == holder {
			rec.Deadline = deadline
			rec.TTL = ttl
			t.locks[name] = rec
			t.wait.Broadcast()
			return rec.Token, true, nil
		}
		return rec.Token, false, nil
	}
	if len(t.locks) >= MaxLocks {
		return 0, false, fmt.Errorf("too many locks (max %d)", MaxLocks)
	}

	t.next++
	rec := Record{
		Name:       name,
		Holder:     holder,
		Token:      t.next,
		AcquiredAt: time.Now(),
		Deadline:   deadline,
		TTL:        ttl,
	}
	t.locks[name] = rec
	t.wait.Broadcast()
	return rec.Token, true, nil
}

// Release drops name if holder and token match the current grant.
func (t *Table) Release(name, holder string, token uint64) error {
	if err := ValidName(name); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.locks[name]
	if !ok {
		return fmt.Errorf("lock %q is not held", name)
	}
	if rec.Holder != holder || rec.Token != token {
		return fmt.Errorf("fencing token mismatch for lock %q", name)
	}
	delete(t.locks, name)
	t.wait.Broadcast()
	return nil
}

// Renew extends the deadline if holder and token match.
func (t *Table) Renew(name, holder string, token uint64, deadline time.Time, ttl time.Duration) error {
	if err := ValidName(name); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.locks[name]
	if !ok {
		return fmt.Errorf("lock %q is not held", name)
	}
	if rec.Holder != holder || rec.Token != token {
		return fmt.Errorf("fencing token mismatch for lock %q", name)
	}
	rec.Deadline = deadline
	rec.TTL = ttl
	t.locks[name] = rec
	return nil
}

// Expire releases name if token still matches. No-op on mismatch (renew raced).
// Emits lock.expired when a grant is actually dropped.
func (t *Table) Expire(name string, token uint64) bool {
	if err := ValidName(name); err != nil {
		return false
	}
	t.mu.Lock()
	rec, ok := t.locks[name]
	if !ok || rec.Token != token {
		t.mu.Unlock()
		return false
	}
	delete(t.locks, name)
	emit := t.Emit
	t.wait.Broadcast()
	t.mu.Unlock()

	if emit != nil {
		payload, _ := json.Marshal(map[string]any{
			"holder": rec.Holder,
			"token":  rec.Token,
		})
		emit(events.Event{
			Type:    events.TypeLockExpired,
			Source:  name,
			Payload: payload,
		})
	}
	return true
}

// Due returns held locks whose deadline is set and not after now.
func (t *Table) Due(now time.Time) []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []Record
	for _, rec := range t.locks {
		if rec.Deadline.IsZero() {
			continue
		}
		if !rec.Deadline.After(now) {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a copy of the held lock, or false.
func (t *Table) Get(name string) (Record, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, ok := t.locks[name]
	return rec, ok
}

// List returns held locks sorted by name.
func (t *Table) List() []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Record, 0, len(t.locks))
	for _, rec := range t.locks {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Snapshot is the serialisable lock table for Raft snapshots.
type Snapshot struct {
	Locks []Record `json:"locks"`
	Next  uint64   `json:"next"`
}

// SnapshotCopy returns a deep-enough copy for FSM snapshotting.
func (t *Table) SnapshotCopy() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := Snapshot{Next: t.next, Locks: make([]Record, 0, len(t.locks))}
	for _, rec := range t.locks {
		s.Locks = append(s.Locks, rec)
	}
	return s
}

// Restore replaces the table with snap. Used by FSM.Restore.
func (t *Table) Restore(snap Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.locks = make(map[string]Record, len(snap.Locks))
	for _, rec := range snap.Locks {
		t.locks[rec.Name] = rec
	}
	t.next = snap.Next
	t.wait.Broadcast()
}

// Wait wakes when the table changes or ctx is done.
func (t *Table) Wait(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	stop := context.AfterFunc(ctx, func() {
		t.mu.Lock()
		t.wait.Broadcast()
		t.mu.Unlock()
	})
	defer stop()
	t.wait.Wait()
	return ctx.Err()
}

// ValidName reports whether name is a legal lock identity.
func ValidName(name string) error {
	if name == "" {
		return fmt.Errorf("lock name is required")
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("lock name exceeds %d characters", MaxNameLen)
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("lock name contains invalid character %q", r)
		}
	}
	return nil
}
