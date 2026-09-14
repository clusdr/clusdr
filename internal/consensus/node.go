package consensus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	raftlib "github.com/hashicorp/raft"
	raftbolt "github.com/hashicorp/raft-boltdb/v2"

	"github.com/odurgut/clusdr/internal/leases"
	"github.com/odurgut/clusdr/internal/locks"
	"github.com/odurgut/clusdr/internal/membership"
)

// Config holds Raft-specific parameters.
type Config struct {
	// Addr is the TCP address this node binds for Raft peer communication.
	// Default: 127.0.0.1:7946
	Addr string

	// Bootstrap causes this node to bootstrap a new single-server cluster on
	// first start. Set only on the seed node. Idempotent: re-running a
	// bootstrapped node with Bootstrap=true is safe (ErrCantBootstrap is
	// silently ignored).
	Bootstrap bool

	// HeartbeatTimeout / ElectionTimeout / LeaderLeaseTimeout: use Raft
	// defaults unless overridden. Zero keeps DefaultConfig() values.
	// LeaderLeaseTimeout must be <= HeartbeatTimeout when both are set.
	HeartbeatTimeout   time.Duration
	ElectionTimeout    time.Duration
	LeaderLeaseTimeout time.Duration

	// Transport overrides the TCP Raft transport. Tests inject an in-memory
	// transport so peers can be partitioned without kernel filters.
	// Production leaves this nil.
	Transport raftlib.Transport

	// LogOutput receives hashicorp/raft's own logger. Nil uses os.Stderr.
	LogOutput io.Writer
}

// Node wraps hashicorp/raft with Clusdr lifecycle and observation conventions.
type Node struct {
	r      *raftlib.Raft
	log    *slog.Logger
	selfID string

	// ctx/cancel cover all background watcher goroutines.
	// Shutdown cancels ctx which stops all watchers before calling Raft.Shutdown.
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates and starts a Raft node. If cfg.Bootstrap is true the node
// immediately bootstraps a single-server cluster and will elect itself leader.
//
// applier is called by the FSM whenever a membership command is committed.
// lockTab is the lock table; nil disables lock commands.
// leaseTab is the lease table; nil disables lease commands.
//
// dataDir is the Clusdr data directory; Raft files are placed in dataDir/raft/.
func New(cfg Config, nodeID, dataDir string, applier MemberApplier, lockTab LockApplier, leaseTab LeaseApplier, log *slog.Logger) (*Node, error) {
	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir raft dir: %w", err)
	}

	// --- Raft configuration -------------------------------------------------
	rc := raftlib.DefaultConfig()
	rc.LocalID = raftlib.ServerID(nodeID)
	raftLog := cfg.LogOutput
	if raftLog == nil {
		raftLog = os.Stderr
	}
	// Suppress hashicorp/raft's own logs; Clusdr emits structured slog instead.
	rc.Logger = hclog.New(&hclog.LoggerOptions{
		Level:  hclog.Warn,
		Output: raftLog,
	})
	if cfg.HeartbeatTimeout > 0 {
		rc.HeartbeatTimeout = cfg.HeartbeatTimeout
	}
	if cfg.ElectionTimeout > 0 {
		rc.ElectionTimeout = cfg.ElectionTimeout
	}
	if cfg.LeaderLeaseTimeout > 0 {
		rc.LeaderLeaseTimeout = cfg.LeaderLeaseTimeout
	}

	transport, raftAddr, err := raftTransport(cfg)
	if err != nil {
		return nil, err
	}

	// --- Persistent stores ---------------------------------------------------
	boltPath := filepath.Join(raftDir, "raft.db")
	boltStore, err := raftbolt.New(raftbolt.Options{Path: boltPath})
	if err != nil {
		return nil, fmt.Errorf("raft bolt store: %w", err)
	}

	snapDir := filepath.Join(raftDir, "snapshots")
	snapStore, err := raftlib.NewFileSnapshotStore(snapDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("raft snapshot store: %w", err)
	}

	// --- Create Raft node ----------------------------------------------------
	fsm := &FSM{log: log, applier: applier, locks: lockTab, leases: leaseTab}
	r, err := raftlib.NewRaft(rc, fsm, boltStore, boltStore, snapStore, transport)
	if err != nil {
		return nil, fmt.Errorf("new raft: %w", err)
	}

	// --- Bootstrap (seed node only) ------------------------------------------
	if cfg.Bootstrap {
		configuration := raftlib.Configuration{
			Servers: []raftlib.Server{
				{
					ID:      raftlib.ServerID(nodeID),
					Address: raftlib.ServerAddress(raftAddr),
				},
			},
		}
		if err := r.BootstrapCluster(configuration).Error(); err != nil &&
			!errors.Is(err, raftlib.ErrCantBootstrap) {
			return nil, fmt.Errorf("raft bootstrap: %w", err)
		}
		log.Info("raft bootstrap complete", "node_id", nodeID, "addr", raftAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Node{r: r, log: log, selfID: nodeID, ctx: ctx, cancel: cancel}, nil
}

func raftTransport(cfg Config) (raftlib.Transport, string, error) {
	if cfg.Transport != nil {
		return cfg.Transport, string(cfg.Transport.LocalAddr()), nil
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", cfg.Addr)
	if err != nil {
		return nil, "", fmt.Errorf("resolve raft addr %q: %w", cfg.Addr, err)
	}
	transport, err := raftlib.NewTCPTransport(cfg.Addr, tcpAddr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, "", fmt.Errorf("raft tcp transport: %w", err)
	}
	return transport, cfg.Addr, nil
}

// WatchLeadership runs fn in a goroutine each time this node gains or loses
// leadership. fn(true) = became leader; fn(false) = lost leadership.
// Goroutine exits when the node is shut down.
func (n *Node) WatchLeadership(fn func(isLeader bool)) {
	ch := n.r.LeaderCh()
	go func() {
		for {
			select {
			case <-n.ctx.Done():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				n.log.Info("raft leadership changed", "is_leader", v, "node_id", n.selfID)
				fn(v)
			}
		}
	}()
}

// WatchLeaderChange runs fn whenever the cluster leader changes (any node).
// fn receives the new leader's node ID; empty string means no leader.
// Uses the Raft observer API so all nodes (leaders and followers) receive the
// event. Goroutine exits when the node is shut down.
func (n *Node) WatchLeaderChange(fn func(leaderID string)) {
	ch := make(chan raftlib.Observation, 8)
	obs := raftlib.NewObserver(ch, false, func(o *raftlib.Observation) bool {
		_, ok := o.Data.(raftlib.LeaderObservation)
		return ok
	})
	n.r.RegisterObserver(obs)

	go func() {
		defer n.r.DeregisterObserver(obs)
		for {
			select {
			case <-n.ctx.Done():
				return
			case o := <-ch:
				if lo, ok := o.Data.(raftlib.LeaderObservation); ok {
					fn(string(lo.LeaderID))
				}
			}
		}
	}()
}

// AddVoter adds a new voting member to the Raft cluster configuration.
// Must only be called on the leader; returns an error otherwise.
// The index and timeout parameters use Raft defaults (0 = no prev index guard,
// 10s timeout).
func (n *Node) AddVoter(id, raftAddr string) error {
	f := n.r.AddVoter(
		raftlib.ServerID(id),
		raftlib.ServerAddress(raftAddr),
		0, 10*time.Second,
	)
	if err := f.Error(); err != nil {
		return fmt.Errorf("raft add voter %s: %w", id, err)
	}
	n.log.Info("raft voter added", "node_id", id, "raft_addr", raftAddr)
	return nil
}

// AddNonvoter adds a Raft observer (non-voter). Quorum is unchanged.
// Must only be called on the leader.
func (n *Node) AddNonvoter(id, raftAddr string) error {
	f := n.r.AddNonvoter(
		raftlib.ServerID(id),
		raftlib.ServerAddress(raftAddr),
		0, 10*time.Second,
	)
	if err := f.Error(); err != nil {
		return fmt.Errorf("raft add nonvoter %s: %w", id, err)
	}
	n.log.Info("raft nonvoter added", "node_id", id, "raft_addr", raftAddr)
	return nil
}

// PromoteToVoter turns an existing Raft server (typically a non-voter) into a
// voter. Uses the address already in the Raft configuration. No-op if the
// server is already a voter. Must only be called on the leader.
func (n *Node) PromoteToVoter(id string) error {
	fut := n.r.GetConfiguration()
	if err := fut.Error(); err != nil {
		return fmt.Errorf("raft get configuration: %w", err)
	}
	var addr raftlib.ServerAddress
	found := false
	for _, s := range fut.Configuration().Servers {
		if string(s.ID) != id {
			continue
		}
		found = true
		if s.Suffrage == raftlib.Voter {
			return nil
		}
		addr = s.Address
		break
	}
	if !found {
		return fmt.Errorf("raft server %s not in configuration", id)
	}
	return n.AddVoter(id, string(addr))
}

// RemoveVoter removes a node from the Raft configuration (voter or observer).
// Must only be called on the leader; safe to call on an already-removed node
// (Raft returns nil in that case).
func (n *Node) RemoveVoter(id string) error {
	f := n.r.RemoveServer(
		raftlib.ServerID(id),
		0, 10*time.Second,
	)
	if err := f.Error(); err != nil {
		return fmt.Errorf("raft remove server %s: %w", id, err)
	}
	n.log.Info("raft server removed", "node_id", id)
	return nil
}

// IsLeader returns true when this node is the current Raft leader.
func (n *Node) IsLeader() bool {
	return n.r.State() == raftlib.Leader
}

// LeaderAddr returns the Raft address of the current leader (empty if unknown).
func (n *Node) LeaderAddr() string {
	addr, _ := n.r.LeaderWithID()
	return string(addr)
}

// LeaderID returns the server ID (node ID) of the current leader.
func (n *Node) LeaderID() string {
	_, id := n.r.LeaderWithID()
	return string(id)
}

// Ctx returns the node's lifecycle context. It is cancelled when Shutdown is
// called, which allows subscriber goroutines to detect teardown.
func (n *Node) Ctx() context.Context { return n.ctx }

// Raft exposes the underlying hashicorp/raft instance for voter
// management (AddVoter / RemoveServer). Avoid calling it outside internal/app.
func (n *Node) Raft() *raftlib.Raft { return n.r }

// ApplyAddMember writes an add_member command to the Raft log and blocks until
// it is committed on a quorum. Must only be called on the leader.
// The FSM.Apply handler propagates the join to all nodes' membership engines.
func (n *Node) ApplyAddMember(id, addr string) error {
	return n.ApplyAddMemberAs(id, addr, membership.RoleVoter)
}

// ApplyAddMemberAs writes add_member with an explicit role (voter or observer).
func (n *Node) ApplyAddMemberAs(id, addr, role string) error {
	role = membership.NormalizeRole(role)
	data, err := encodeCommand(Command{Kind: CmdAddMember, ID: id, Address: addr, Role: role})
	if err != nil {
		return err
	}
	if err := n.r.Apply(data, 5*time.Second).Error(); err != nil {
		return fmt.Errorf("raft apply add_member %s: %w", id, err)
	}
	return nil
}

// ApplyRemoveMember writes a remove_member command to the Raft log and blocks
// until committed. Must only be called on the leader.
func (n *Node) ApplyRemoveMember(id string) error {
	data, err := encodeCommand(Command{Kind: CmdRemoveMember, ID: id})
	if err != nil {
		return err
	}
	if err := n.r.Apply(data, 5*time.Second).Error(); err != nil {
		return fmt.Errorf("raft apply remove_member %s: %w", id, err)
	}
	return nil
}

// ApplyLockAcquire commits a lock acquire. Must be called on the leader.
// ttl <= 0 uses locks.DefaultTTL. The leader stamps an absolute deadline so
// followers do not call time.Now() when applying.
func (n *Node) ApplyLockAcquire(name, holder string, ttl time.Duration) (token uint64, acquired bool, err error) {
	ttl = locks.ClampTTL(ttl, 0)
	deadline := time.Now().Add(ttl)
	data, err := encodeCommand(Command{
		Kind:           CmdLockAcquire,
		ID:             holder,
		Name:           name,
		TTLMs:          ttl.Milliseconds(),
		DeadlineUnixMs: deadline.UnixMilli(),
	})
	if err != nil {
		return 0, false, err
	}
	res, err := n.applyLock(data, "lock_acquire", name)
	if err != nil {
		return 0, false, err
	}
	return res.Token, res.Acquired, nil
}

// ApplyLockRelease commits a lock release. Must be called on the leader.
func (n *Node) ApplyLockRelease(name, holder string, token uint64) error {
	data, err := encodeCommand(Command{Kind: CmdLockRelease, ID: holder, Name: name, Token: token})
	if err != nil {
		return err
	}
	_, err = n.applyLock(data, "lock_release", name)
	return err
}

// ApplyLockRenew extends a held lock. Must be called on the leader.
func (n *Node) ApplyLockRenew(name, holder string, token uint64, ttl time.Duration) (deadline time.Time, err error) {
	ttl = locks.ClampTTL(ttl, 0)
	deadline = time.Now().Add(ttl)
	data, err := encodeCommand(Command{
		Kind:           CmdLockRenew,
		ID:             holder,
		Name:           name,
		Token:          token,
		TTLMs:          ttl.Milliseconds(),
		DeadlineUnixMs: deadline.UnixMilli(),
	})
	if err != nil {
		return time.Time{}, err
	}
	if _, err := n.applyLock(data, "lock_renew", name); err != nil {
		return time.Time{}, err
	}
	return deadline, nil
}

// ApplyLockExpire commits an expiry. No-op if the token no longer matches.
func (n *Node) ApplyLockExpire(name string, token uint64) error {
	data, err := encodeCommand(Command{Kind: CmdLockExpire, Name: name, Token: token})
	if err != nil {
		return err
	}
	_, err = n.applyLock(data, "lock_expire", name)
	return err
}

// ApplyLeaseGrant commits a lease grant. Must be called on the leader.
func (n *Node) ApplyLeaseGrant(name, owner string, ttl time.Duration) (token uint64, granted bool, err error) {
	ttl = leases.ClampTTL(ttl, 0)
	deadline := time.Now().Add(ttl)
	data, err := encodeCommand(Command{
		Kind:           CmdLeaseGrant,
		ID:             owner,
		Name:           name,
		TTLMs:          ttl.Milliseconds(),
		DeadlineUnixMs: deadline.UnixMilli(),
	})
	if err != nil {
		return 0, false, err
	}
	res, err := n.applyLock(data, "lease_grant", name)
	if err != nil {
		return 0, false, err
	}
	return res.Token, res.Acquired, nil
}

// ApplyLeaseRenew extends a held lease. Must be called on the leader.
func (n *Node) ApplyLeaseRenew(name, owner string, token uint64, ttl time.Duration) (deadline time.Time, err error) {
	ttl = leases.ClampTTL(ttl, 0)
	deadline = time.Now().Add(ttl)
	data, err := encodeCommand(Command{
		Kind:           CmdLeaseRenew,
		ID:             owner,
		Name:           name,
		Token:          token,
		TTLMs:          ttl.Milliseconds(),
		DeadlineUnixMs: deadline.UnixMilli(),
	})
	if err != nil {
		return time.Time{}, err
	}
	if _, err := n.applyLock(data, "lease_renew", name); err != nil {
		return time.Time{}, err
	}
	return deadline, nil
}

// ApplyLeaseRevoke commits a lease revoke. Must be called on the leader.
func (n *Node) ApplyLeaseRevoke(name, owner string, token uint64) error {
	data, err := encodeCommand(Command{Kind: CmdLeaseRevoke, ID: owner, Name: name, Token: token})
	if err != nil {
		return err
	}
	_, err = n.applyLock(data, "lease_revoke", name)
	return err
}

// ApplyLeaseExpire commits an expiry. No-op if the token no longer matches.
func (n *Node) ApplyLeaseExpire(name string, token uint64) error {
	data, err := encodeCommand(Command{Kind: CmdLeaseExpire, Name: name, Token: token})
	if err != nil {
		return err
	}
	_, err = n.applyLock(data, "lease_expire", name)
	return err
}

func (n *Node) applyLock(data []byte, op, name string) (lockResult, error) {
	f := n.r.Apply(data, 5*time.Second)
	if err := f.Error(); err != nil {
		return lockResult{}, fmt.Errorf("raft apply %s %s: %w", op, name, err)
	}
	res, ok := f.Response().(lockResult)
	if !ok {
		return lockResult{}, fmt.Errorf("raft apply %s %s: unexpected result", op, name)
	}
	if res.Err != "" {
		return lockResult{}, fmt.Errorf("%s %s: %s", op, name, res.Err)
	}
	return res, nil
}

// Shutdown stops all watcher goroutines and shuts down the Raft node cleanly.
func (n *Node) Shutdown() error {
	n.cancel() // stops all watchers
	if err := n.r.Shutdown().Error(); err != nil {
		return fmt.Errorf("raft shutdown: %w", err)
	}
	return nil
}
