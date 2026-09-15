package clusdr

import (
	"context"
	"sync"
	"time"
)

// Cluster is the application view of the local daemon.
//
// Safe for concurrent use. Every RPC takes a context; if the caller does not
// set a deadline, [WithRequestTimeout] applies (default 10s). Close releases
// locks and leases this connection still holds, then closes the gRPC conn.
type Cluster interface {
	// Members is the last applied membership list on this daemon (may be stale
	// on a partitioned follower).
	Members(ctx context.Context) ([]Member, error)
	// Leader is the current Raft leader. No leader → gRPC Unavailable.
	Leader(ctx context.Context) (Member, error)
	// Watch returns a channel of cluster and custom events. The call itself
	// returns immediately; a background goroutine fills the channel. Cancel ctx
	// to stop; the channel is then closed. Zero options is the full bus.
	// See [WithTopics] and [WithEventTypes].
	Watch(ctx context.Context, opts ...WatchOption) (<-chan Event, error)
	// Publish emits custom.<topic> on this node and one-hop to alive peers.
	// Not replicated on Raft. Payload max 64 KiB.
	Publish(ctx context.Context, topic string, payload []byte) error
	// Lock blocks until name is acquired or ctx is done. ttl<=0 uses the daemon default.
	// Same name already held by this connection → existing *Lock, no extra RPC.
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
	// Close unlocks and revokes grants this connection still holds, then
	// closes the gRPC connection. In-flight RPCs and Watch streams error out.
	Close() error
}

// Lock is a held exclusive lock. Token is the fencing token — store it with
// any write that must be fenced; a stale holder cannot unlock a newer grant.
type Lock struct {
	// Name is the exclusive lock name on the Raft log.
	Name string
	// Holder is this connection's lock identity (WithHolder or sdk-<hex>).
	Holder string
	// Token is the fencing token. Store it with any write that must be fenced.
	Token uint64

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
	// Name is the lease name on the Raft log.
	Name string
	// Owner is this connection's lease identity (same as lock holder).
	Owner string
	// Token is the fencing token.
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
	// ID is the stable node id (node.id).
	ID string
	// Address is the advertised Runtime API (node.addr).
	Address string
	// Status is alive, leaving, or dead.
	Status string
	// Leader is true if this id is the current Raft leader.
	Leader bool
	// Role is voter, observer, or empty (treated as voter).
	Role string
}

// Event is a cluster or custom event from the Watch stream.
type Event struct {
	// Type is member.join, leader.changed, custom.<topic>, watch.sync, …
	Type string
	// Source is usually a node id; lock/lease events use the name.
	Source string
	// Payload is raw bytes (JSON or otherwise). Empty for most cluster events.
	Payload []byte
	// Timestamp is when the daemon stamped the event.
	Timestamp time.Time
	// Seq is the bus sequence. Used as last_seq on Watch reconnect.
	Seq uint64
}

// Local connects to the daemon on this host.
//
// Address: CLUSDR_GRPC_ADDR, or 127.0.0.1:7947. TLS is on unless
// CLUSDR_TLS=disabled; certs from CLUSDR_DATA_DIR or ~/.clusdr. Then waits
// on the Health RPC (10s). That wait is not a public option.
func Local(opts ...Option) (Cluster, error) {
	o := defaultOptions()
	o.apply(opts)
	if o.addr == "" {
		o.addr = localAddr()
	}
	return dial(o)
}

// Dial connects to addr. Tests and operators use this; applications use Local.
//
// addr is a Runtime API host:port on this machine (or a test listener), not a
// remote cluster peer. Empty addr returns an error.
func Dial(addr string, opts ...Option) (Cluster, error) {
	o := defaultOptions()
	o.apply(opts)
	o.addr = addr
	return dial(o)
}
