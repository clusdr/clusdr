// Package membership manages the in-memory cluster member list.
//
// Leader is set by Raft via SetLeader. Join/Leave/SetLeader emit events
// onto the bus for Watch streams.
package membership

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/durguto/clusdr/internal/events"
)

// Status is the liveness state of a cluster member.
type Status string

const (
	StatusAlive   Status = "alive"
	StatusLeaving Status = "leaving"
	StatusDead    Status = "dead"
)

// Raft membership role. Empty Role on a Member is treated as voter.
const (
	RoleVoter    = "voter"
	RoleObserver = "observer"
)

// NormalizeRole maps empty or unknown values to voter.
func NormalizeRole(role string) string {
	if role == RoleObserver {
		return RoleObserver
	}
	return RoleVoter
}

// Member is a snapshot of a single cluster node.
type Member struct {
	ID       string
	Address  string
	Status   Status
	Leader   bool
	Role     string
	JoinedAt time.Time
}

// Engine manages the cluster member list for the local node.
// It is safe for concurrent use.
type Engine struct {
	mu       sync.RWMutex
	selfID   string
	leaderID string
	members  map[string]Member
	log      *slog.Logger

	// Emit is an optional hook called after membership events (join, left,
	// leader changed).
	// Called without the mutex held. Safe to call gRPC or Raft methods.
	Emit func(events.Event)
}

// New creates an Engine with the local node as the only member and leader.
func New(selfID, selfAddr string, log *slog.Logger) *Engine {
	e := &Engine{
		selfID:   selfID,
		leaderID: selfID, // single node is its own leader until Raft takes over
		members:  make(map[string]Member),
		log:      log,
	}
	e.members[selfID] = Member{
		ID:       selfID,
		Address:  selfAddr,
		Status:   StatusAlive,
		Leader:   true,
		Role:     RoleVoter,
		JoinedAt: time.Now(),
	}
	return e
}

// Join adds or refreshes a member. Safe to call if the member already exists.
//
// Returns (true, nil) when the member was newly added, (false, nil) when the
// member was already known and alive. The Raft FSM uses the bool only for
// logging; duplicate apply is safe.
//
// Returns a non-nil error only for programming bugs (empty id).
func (e *Engine) Join(id, addr string) (bool, error) {
	return e.JoinAs(id, addr, RoleVoter)
}

// JoinAs is Join with an explicit Raft role (voter or observer).
// Empty or unknown role is voter.
func (e *Engine) JoinAs(id, addr, role string) (bool, error) {
	if id == "" {
		return false, fmt.Errorf("membership join: empty node id")
	}
	role = NormalizeRole(role)
	e.mu.Lock()

	existing, ok := e.members[id]
	isNew := !ok || existing.Status != StatusAlive
	joinedAt := time.Now()
	if ok && existing.Status == StatusAlive {
		joinedAt = existing.JoinedAt
	}
	e.members[id] = Member{
		ID:       id,
		Address:  addr,
		Status:   StatusAlive,
		Leader:   id == e.leaderID,
		Role:     role,
		JoinedAt: joinedAt,
	}
	emit := e.Emit
	e.mu.Unlock()

	if isNew {
		e.log.Info("member.join", "node_id", id, "address", addr, "role", role)
		if emit != nil {
			payload, _ := json.Marshal(map[string]string{"address": addr, "role": role})
			emit(events.Event{
				Type:    events.TypeMemberJoin,
				Source:  id,
				Payload: payload,
			})
		}
	}
	return isNew, nil
}

// MarkLeaving marks a member as leaving. Used by the consensus FSM when
// applying a remove_member log entry. Emits member.left for Watch; callers
// must not turn that event back into another Raft command.
func (e *Engine) MarkLeaving(id string) {
	e.mu.Lock()
	m, ok := e.members[id]
	if !ok || m.Status != StatusAlive {
		e.mu.Unlock()
		return
	}
	m.Status = StatusLeaving
	e.members[id] = m
	emit := e.Emit
	e.mu.Unlock()

	e.log.Info("member.left", "node_id", id)
	if emit != nil {
		emit(events.Event{Type: events.TypeMemberLeft, Source: id})
	}
}

// Leave marks a member as leaving. Noop if the member is unknown.
// Emits a TypeMemberLeft event via Emit (if set) after releasing the lock so callers can safely
// perform gRPC or Raft operations from the hook.
func (e *Engine) Leave(id string) {
	e.mu.Lock()
	m, ok := e.members[id]
	if !ok || m.Status != StatusAlive {
		e.mu.Unlock()
		return
	}
	m.Status = StatusLeaving
	e.members[id] = m
	emit := e.Emit
	e.mu.Unlock()

	e.log.Info("member.left", "node_id", id)
	if emit != nil {
		emit(events.Event{Type: events.TypeMemberLeft, Source: id})
	}
}

// Members returns a stable-sorted snapshot of all known members.
func (e *Engine) Members() []Member {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Member, 0, len(e.members))
	for _, m := range e.members {
		out = append(out, m)
	}
	// Stable sort by ID for deterministic output.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Leader returns the current leader member.
func (e *Engine) Leader() (Member, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m, ok := e.members[e.leaderID]
	return m, ok
}

// SetLeader updates the known leader. Called by the Raft engine.
// Same id again is a no-op: Raft can re-observe the current leader
// (brief empty window, then the same node) and that is not a change.
func (e *Engine) SetLeader(id string) {
	e.mu.Lock()
	if e.leaderID == id {
		e.mu.Unlock()
		return
	}
	e.leaderID = id
	for k, m := range e.members {
		m.Leader = (k == id)
		e.members[k] = m
	}
	emit := e.Emit
	e.mu.Unlock()

	e.log.Info("leader.changed", "leader_id", id)
	if emit != nil {
		emit(events.Event{Type: events.TypeLeaderChanged, Source: id})
	}
}

// SelfID returns the local node's ID.
func (e *Engine) SelfID() string { return e.selfID }

// SelfRole returns this node's Raft role (voter or observer).
func (e *Engine) SelfRole() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if m, ok := e.members[e.selfID]; ok {
		return NormalizeRole(m.Role)
	}
	return RoleVoter
}

// LeaderID returns the current known leader's node ID, or "" if unknown.
func (e *Engine) LeaderID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.leaderID
}
