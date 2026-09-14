// Package clusdr is the application SDK for the local Clusdr daemon.
//
// Applications talk only to the daemon on the same host (Docker-style).
// The daemon is the cluster member; this package does not dial other nodes.
//
//	c, err := clusdr.Local()
//	members, err := c.Members(ctx)
//	ch, err := c.Watch(ctx)
package clusdr

import (
	"context"
	"sync"
	"time"
)

// Cluster is the application view of the local daemon.
// Context on every RPC: gRPC always has a deadline (caller or RequestTimeout).
type Cluster interface {
	Members(ctx context.Context) ([]Member, error)
	Leader(ctx context.Context) (Member, error)
	Watch(ctx context.Context) (<-chan Event, error)
	Publish(ctx context.Context, topic string, payload []byte) error
	// Lock blocks until name is acquired or ctx is done. ttl<=0 uses the daemon default.
	Lock(ctx context.Context, name string, ttl time.Duration) (*Lock, error)
	// TryLock attempts once. ok is false if another holder has the lock (not an error).
	TryLock(ctx context.Context, name string, ttl time.Duration) (lk *Lock, ok bool, err error)
	// Unlock releases a lock this client previously acquired.
	Unlock(ctx context.Context, name string) error
	// Lease grants name and renews in the background until ctx is cancelled,
	// Revoke, or Close. Cancelling ctx stops renewal; the grant then expires.
	Lease(ctx context.Context, name string, ttl time.Duration) (*Lease, error)
	// Renew extends a lease this client previously granted.
	Renew(ctx context.Context, name string) error
	// Revoke drops a lease this client previously granted.
	Revoke(ctx context.Context, name string) error
	Close() error
}

// Lock is a held exclusive lock. Token is the fencing token — store it with
// any write that must be fenced; a stale holder cannot unlock a newer grant.
type Lock struct {
	Name   string
	Holder string
	Token  uint64

	mu       sync.Mutex
	deadline time.Time
	c        *client
	stop     context.CancelFunc
}

// Deadline is the last known grant expiry (extended by background renew).
func (l *Lock) Deadline() time.Time {
	if l == nil {
		return time.Time{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.deadline
}

// Lease is a held TTL grant. Token is the fencing token.
type Lease struct {
	Name  string
	Owner string
	Token uint64

	mu       sync.Mutex
	deadline time.Time
	c        *client
	stop     context.CancelFunc
}

// Deadline is the last known grant expiry (extended by background renew).
func (l *Lease) Deadline() time.Time {
	if l == nil {
		return time.Time{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.deadline
}

// Member is a cluster node as seen by the local daemon.
type Member struct {
	ID      string
	Address string
	Status  string
	Leader  bool
	// Role is voter, observer, or empty (treated as voter).
	Role string
}

// Event is a cluster or custom event from the Watch stream.
type Event struct {
	Type      string
	Source    string
	Payload   []byte
	Timestamp time.Time
	Seq       uint64
}

// Local connects to the daemon on this host.
// Address: CLUSDR_GRPC_ADDR, or 127.0.0.1:7947.
// TLS: on unless CLUSDR_TLS=disabled; certs from CLUSDR_DATA_DIR or ~/.clusdr.
func Local(opts ...Option) (Cluster, error) {
	o := defaultOptions()
	o.apply(opts)
	if o.addr == "" {
		o.addr = localAddr()
	}
	return dial(o)
}

// Dial connects to addr. Tests and operators use this; applications use Local.
func Dial(addr string, opts ...Option) (Cluster, error) {
	o := defaultOptions()
	o.apply(opts)
	o.addr = addr
	return dial(o)
}
