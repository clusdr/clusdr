// Package leases is the in-memory lease table mutated only by the Raft FSM.
package leases

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
	"unicode"

	"github.com/clusdr/clusdr/internal/events"
)

const (
	MaxNameLen = 128
	MaxLeases  = 4096

	// DefaultTTL is the grant lifetime when the client omits ttl_ms.
	DefaultTTL = 15 * time.Second
	MinTTL     = time.Millisecond
	MaxTTL     = 24 * time.Hour

	DefaultExpireInterval = 100 * time.Millisecond
)

// ClampTTL maps a requested TTL onto [MinTTL, MaxTTL].
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

// Record is a held lease.
type Record struct {
	Name      string
	Owner     string
	Token     uint64
	GrantedAt time.Time
	Deadline  time.Time     `json:",omitempty"`
	TTL       time.Duration `json:",omitempty"`
}

// Table holds named TTL grants. Safe for concurrent use.
// Writes must come from the Raft FSM so every node sees the same table.
type Table struct {
	mu     sync.Mutex
	leases map[string]Record
	next   uint64

	// Emit is an optional hook after granted / expired / revoked.
	Emit func(events.Event)
}

// New creates an empty lease table.
func New() *Table {
	return &Table{leases: make(map[string]Record)}
}

// Grant tries to take name for owner. Same owner already holding is
// idempotent (same token) and refreshes Deadline/TTL. Does not block.
func (t *Table) Grant(name, owner string, deadline time.Time, ttl time.Duration) (token uint64, granted bool, err error) {
	if err := ValidName(name); err != nil {
		return 0, false, err
	}
	if owner == "" {
		return 0, false, fmt.Errorf("lease owner is required")
	}

	t.mu.Lock()
	if rec, ok := t.leases[name]; ok {
		if rec.Owner == owner {
			rec.Deadline = deadline
			rec.TTL = ttl
			t.leases[name] = rec
			t.mu.Unlock()
			return rec.Token, true, nil
		}
		t.mu.Unlock()
		return rec.Token, false, nil
	}
	if len(t.leases) >= MaxLeases {
		t.mu.Unlock()
		return 0, false, fmt.Errorf("too many leases (max %d)", MaxLeases)
	}

	t.next++
	rec := Record{
		Name:      name,
		Owner:     owner,
		Token:     t.next,
		GrantedAt: time.Now(),
		Deadline:  deadline,
		TTL:       ttl,
	}
	t.leases[name] = rec
	emit := t.Emit
	t.mu.Unlock()

	t.emit(emit, events.TypeLeaseGranted, rec)
	return rec.Token, true, nil
}

// Renew extends the deadline if owner and token match.
func (t *Table) Renew(name, owner string, token uint64, deadline time.Time, ttl time.Duration) error {
	if err := ValidName(name); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.leases[name]
	if !ok {
		return fmt.Errorf("lease %q is not held", name)
	}
	if rec.Owner != owner || rec.Token != token {
		return fmt.Errorf("fencing token mismatch for lease %q", name)
	}
	rec.Deadline = deadline
	rec.TTL = ttl
	t.leases[name] = rec
	return nil
}

// Revoke drops name if owner and token match. Emits lease.revoked.
func (t *Table) Revoke(name, owner string, token uint64) error {
	if err := ValidName(name); err != nil {
		return err
	}
	t.mu.Lock()
	rec, ok := t.leases[name]
	if !ok {
		t.mu.Unlock()
		return fmt.Errorf("lease %q is not held", name)
	}
	if rec.Owner != owner || rec.Token != token {
		t.mu.Unlock()
		return fmt.Errorf("fencing token mismatch for lease %q", name)
	}
	delete(t.leases, name)
	emit := t.Emit
	t.mu.Unlock()

	t.emit(emit, events.TypeLeaseRevoked, rec)
	return nil
}

// Expire releases name if token still matches. No-op on mismatch (renew raced).
func (t *Table) Expire(name string, token uint64) bool {
	if err := ValidName(name); err != nil {
		return false
	}
	t.mu.Lock()
	rec, ok := t.leases[name]
	if !ok || rec.Token != token {
		t.mu.Unlock()
		return false
	}
	delete(t.leases, name)
	emit := t.Emit
	t.mu.Unlock()

	t.emit(emit, events.TypeLeaseExpired, rec)
	return true
}

// Due returns held leases whose deadline is set and not after now.
func (t *Table) Due(now time.Time) []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []Record
	for _, rec := range t.leases {
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

// Get returns a copy of the held lease, or false.
func (t *Table) Get(name string) (Record, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec, ok := t.leases[name]
	return rec, ok
}

// List returns held leases sorted by name.
func (t *Table) List() []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Record, 0, len(t.leases))
	for _, rec := range t.leases {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Snapshot is the serialisable lease table for Raft snapshots.
type Snapshot struct {
	Leases []Record `json:"leases"`
	Next   uint64   `json:"next"`
}

// SnapshotCopy returns a deep-enough copy for FSM snapshotting.
func (t *Table) SnapshotCopy() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := Snapshot{Next: t.next, Leases: make([]Record, 0, len(t.leases))}
	for _, rec := range t.leases {
		s.Leases = append(s.Leases, rec)
	}
	return s
}

// Restore replaces the table with snap. Used by FSM.Restore.
func (t *Table) Restore(snap Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.leases = make(map[string]Record, len(snap.Leases))
	for _, rec := range snap.Leases {
		t.leases[rec.Name] = rec
	}
	t.next = snap.Next
}

func (t *Table) emit(fn func(events.Event), typ string, rec Record) {
	if fn == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"owner": rec.Owner,
		"token": rec.Token,
	})
	fn(events.Event{
		Type:    typ,
		Source:  rec.Name,
		Payload: payload,
	})
}

// ValidName reports whether name is a legal lease identity.
func ValidName(name string) error {
	if name == "" {
		return fmt.Errorf("lease name is required")
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("lease name exceeds %d characters", MaxNameLen)
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("lease name contains invalid character %q", r)
		}
	}
	return nil
}
