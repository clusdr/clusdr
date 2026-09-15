package consensus

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	raftlib "github.com/hashicorp/raft"

	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/locks"
	"github.com/durguto/clusdr/internal/membership"
)

// MemberApplier is the subset of membership.Engine used by the FSM.
type MemberApplier interface {
	Join(id, addr string) (bool, error)
	JoinAs(id, addr, role string) (bool, error)
	MarkLeaving(id string)
	Members() []membership.Member
}

// LockApplier is the lock table the FSM mutates. nil disables lock commands.
type LockApplier interface {
	Acquire(name, holder string, deadline time.Time, ttl time.Duration) (token uint64, acquired bool, err error)
	Release(name, holder string, token uint64) error
	Renew(name, holder string, token uint64, deadline time.Time, ttl time.Duration) error
	Expire(name string, token uint64) bool
	SnapshotCopy() locks.Snapshot
	Restore(locks.Snapshot)
}

// LeaseApplier is the lease table the FSM mutates. nil disables lease commands.
type LeaseApplier interface {
	Grant(name, owner string, deadline time.Time, ttl time.Duration) (token uint64, granted bool, err error)
	Renew(name, owner string, token uint64, deadline time.Time, ttl time.Duration) error
	Revoke(name, owner string, token uint64) error
	Expire(name string, token uint64) bool
	SnapshotCopy() leases.Snapshot
	Restore(leases.Snapshot)
}

// lockResult is returned from FSM.Apply for lock commands (via Raft Future).
type lockResult struct {
	Acquired       bool
	Token          uint64
	DeadlineUnixMs int64
	Err            string
}

// FSM is the Raft finite state machine for Clusdr.
//
// Apply is the single writer for membership, locks, and leases.
type FSM struct {
	log     *slog.Logger
	applier MemberApplier
	locks   LockApplier
	leases  LeaseApplier
}

// Apply is called by Raft on the leader and all followers after a log entry
// is committed.
func (f *FSM) Apply(entry *raftlib.Log) interface{} {
	if entry.Type != raftlib.LogCommand {
		return nil
	}
	cmd, err := decodeCommand(entry.Data)
	if err != nil {
		f.log.Error("fsm apply: decode failed", "index", entry.Index, "err", err)
		return err
	}

	switch cmd.Kind {
	case CmdAddMember:
		role := membership.NormalizeRole(cmd.Role)
		if _, err := f.applier.JoinAs(cmd.ID, cmd.Address, role); err != nil {
			f.log.Error("fsm apply: join failed", "node_id", cmd.ID, "err", err)
			return err
		}
		f.log.Debug("fsm applied add_member", "index", entry.Index, "node_id", cmd.ID, "role", role)

	case CmdRemoveMember:
		f.applier.MarkLeaving(cmd.ID)
		f.log.Debug("fsm applied remove_member", "index", entry.Index, "node_id", cmd.ID)

	case CmdLockAcquire:
		if f.locks == nil {
			return lockResult{Err: "lock table not initialized"}
		}
		tok, ok, err := f.locks.Acquire(cmd.Name, cmd.ID, deadlineOf(cmd), ttlOf(cmd))
		if err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lock_acquire",
			"index", entry.Index, "name", cmd.Name, "holder", cmd.ID, "acquired", ok, "token", tok)
		return lockResult{Acquired: ok, Token: tok, DeadlineUnixMs: cmd.DeadlineUnixMs}

	case CmdLockRelease:
		if f.locks == nil {
			return lockResult{Err: "lock table not initialized"}
		}
		if err := f.locks.Release(cmd.Name, cmd.ID, cmd.Token); err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lock_release",
			"index", entry.Index, "name", cmd.Name, "holder", cmd.ID)
		return lockResult{Acquired: false, Token: cmd.Token}

	case CmdLockRenew:
		if f.locks == nil {
			return lockResult{Err: "lock table not initialized"}
		}
		if err := f.locks.Renew(cmd.Name, cmd.ID, cmd.Token, deadlineOf(cmd), ttlOf(cmd)); err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lock_renew",
			"index", entry.Index, "name", cmd.Name, "holder", cmd.ID)
		return lockResult{Acquired: true, Token: cmd.Token, DeadlineUnixMs: cmd.DeadlineUnixMs}

	case CmdLockExpire:
		if f.locks == nil {
			return lockResult{Err: "lock table not initialized"}
		}
		ok := f.locks.Expire(cmd.Name, cmd.Token)
		f.log.Debug("fsm applied lock_expire",
			"index", entry.Index, "name", cmd.Name, "expired", ok, "token", cmd.Token)
		return lockResult{Acquired: ok, Token: cmd.Token}

	case CmdLeaseGrant:
		if f.leases == nil {
			return lockResult{Err: "lease table not initialized"}
		}
		tok, ok, err := f.leases.Grant(cmd.Name, cmd.ID, deadlineOf(cmd), ttlOf(cmd))
		if err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lease_grant",
			"index", entry.Index, "name", cmd.Name, "owner", cmd.ID, "granted", ok, "token", tok)
		return lockResult{Acquired: ok, Token: tok, DeadlineUnixMs: cmd.DeadlineUnixMs}

	case CmdLeaseRenew:
		if f.leases == nil {
			return lockResult{Err: "lease table not initialized"}
		}
		if err := f.leases.Renew(cmd.Name, cmd.ID, cmd.Token, deadlineOf(cmd), ttlOf(cmd)); err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lease_renew",
			"index", entry.Index, "name", cmd.Name, "owner", cmd.ID)
		return lockResult{Acquired: true, Token: cmd.Token, DeadlineUnixMs: cmd.DeadlineUnixMs}

	case CmdLeaseRevoke:
		if f.leases == nil {
			return lockResult{Err: "lease table not initialized"}
		}
		if err := f.leases.Revoke(cmd.Name, cmd.ID, cmd.Token); err != nil {
			return lockResult{Err: err.Error()}
		}
		f.log.Debug("fsm applied lease_revoke",
			"index", entry.Index, "name", cmd.Name, "owner", cmd.ID)
		return lockResult{Token: cmd.Token}

	case CmdLeaseExpire:
		if f.leases == nil {
			return lockResult{Err: "lease table not initialized"}
		}
		ok := f.leases.Expire(cmd.Name, cmd.Token)
		f.log.Debug("fsm applied lease_expire",
			"index", entry.Index, "name", cmd.Name, "expired", ok, "token", cmd.Token)
		return lockResult{Acquired: ok, Token: cmd.Token}

	default:
		f.log.Warn("fsm apply: unknown command kind", "kind", cmd.Kind)
	}
	return nil
}

type snapshotPayload struct {
	Members        []memberEntry   `json:"members"`
	Locks          []locks.Record  `json:"locks,omitempty"`
	NextLockToken  uint64          `json:"next_lock_token,omitempty"`
	Leases         []leases.Record `json:"leases,omitempty"`
	NextLeaseToken uint64          `json:"next_lease_token,omitempty"`
}

type memberEntry struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Role    string `json:"role,omitempty"`
}

func (f *FSM) Snapshot() (raftlib.FSMSnapshot, error) {
	payload := snapshotPayload{}
	for _, m := range f.applier.Members() {
		if m.Status == membership.StatusAlive {
			payload.Members = append(payload.Members, memberEntry{
				ID:      m.ID,
				Address: m.Address,
				Role:    membership.NormalizeRole(m.Role),
			})
		}
	}
	if f.locks != nil {
		snap := f.locks.SnapshotCopy()
		payload.Locks = snap.Locks
		payload.NextLockToken = snap.Next
	}
	if f.leases != nil {
		snap := f.leases.SnapshotCopy()
		payload.Leases = snap.Leases
		payload.NextLeaseToken = snap.Next
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("snapshot marshal: %w", err)
	}
	return &fsmSnapshot{data: data}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var payload snapshotPayload
	if err := json.NewDecoder(rc).Decode(&payload); err != nil {
		return fmt.Errorf("snapshot decode: %w", err)
	}
	for _, m := range payload.Members {
		if _, err := f.applier.JoinAs(m.ID, m.Address, membership.NormalizeRole(m.Role)); err != nil {
			f.log.Error("fsm restore: join failed", "node_id", m.ID, "err", err)
		}
	}
	if f.locks != nil {
		f.locks.Restore(locks.Snapshot{Locks: payload.Locks, Next: payload.NextLockToken})
	}
	if f.leases != nil {
		f.leases.Restore(leases.Snapshot{Leases: payload.Leases, Next: payload.NextLeaseToken})
	}
	f.log.Info("fsm snapshot restored",
		"members", len(payload.Members),
		"locks", len(payload.Locks),
		"leases", len(payload.Leases),
	)
	return nil
}

type fsmSnapshot struct {
	data []byte
}

func (s *fsmSnapshot) Persist(sink raftlib.SnapshotSink) error {
	if _, err := sink.Write(s.data); err != nil {
		sink.Cancel() //nolint:errcheck
		return fmt.Errorf("snapshot persist: %w", err)
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}

func deadlineOf(cmd Command) time.Time {
	if cmd.DeadlineUnixMs == 0 {
		return time.Time{}
	}
	return time.UnixMilli(cmd.DeadlineUnixMs)
}

func ttlOf(cmd Command) time.Duration {
	if cmd.TTLMs <= 0 {
		return 0
	}
	return time.Duration(cmd.TTLMs) * time.Millisecond
}
