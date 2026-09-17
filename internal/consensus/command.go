package consensus

import (
	"encoding/json"
	"fmt"
)

// CommandKind identifies the FSM operation type.
type CommandKind string

const (
	// CmdAddMember is applied when a new node joins the cluster.
	CmdAddMember CommandKind = "add_member"
	// CmdRemoveMember marks a member dead (liveness). Historical name; does
	// not RemoveServer. Crash/heartbeat/presence use this.
	CmdRemoveMember CommandKind = "remove_member"
	// CmdDropMember removes a member from the list after clusdr leave.
	CmdDropMember CommandKind = "drop_member"
	// CmdLockAcquire tries to take a named lock.
	CmdLockAcquire CommandKind = "lock_acquire"
	// CmdLockRelease releases a named lock (holder + fencing token).
	CmdLockRelease CommandKind = "lock_release"
	// CmdLockRenew extends a held lock's deadline.
	CmdLockRenew CommandKind = "lock_renew"
	// CmdLockExpire releases a lock whose deadline has passed.
	CmdLockExpire CommandKind = "lock_expire"
	// CmdLeaseGrant tries to take a named lease.
	CmdLeaseGrant CommandKind = "lease_grant"
	// CmdLeaseRenew extends a held lease's deadline.
	CmdLeaseRenew CommandKind = "lease_renew"
	// CmdLeaseRevoke releases a named lease (owner + fencing token).
	CmdLeaseRevoke CommandKind = "lease_revoke"
	// CmdLeaseExpire releases a lease whose deadline has passed.
	CmdLeaseExpire CommandKind = "lease_expire"
)

// Command is the payload written to the Raft log by the leader.
// It is encoded as JSON in the Raft log entry's Data field.
type Command struct {
	Kind           CommandKind `json:"kind"`
	ID             string      `json:"id"`
	Address        string      `json:"address,omitempty"`          // add_member
	Role           string      `json:"role,omitempty"`             // add_member: voter | observer
	Name           string      `json:"name,omitempty"`             // lock / lease name
	Token          uint64      `json:"token,omitempty"`            // fencing token
	TTLMs          int64       `json:"ttl_ms,omitempty"`           // grant lifetime
	DeadlineUnixMs int64       `json:"deadline_unix_ms,omitempty"` // absolute expiry (leader clock)
}

func encodeCommand(c Command) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode command: %w", err)
	}
	return b, nil
}

func decodeCommand(data []byte) (Command, error) {
	var c Command
	if err := json.Unmarshal(data, &c); err != nil {
		return Command{}, fmt.Errorf("decode command: %w", err)
	}
	return c, nil
}
