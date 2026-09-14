package app

// units.go registers named units for cmd/clusdr.
//
// Each Init reads from a.Opt or previously-set App fields; each Close
// tears down what Init started. New phases append new Unit vars here
// and add them to the slice in cmd/clusdr/start.go.

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc/credentials"

	"github.com/odurgut/clusdr/internal/config"
	"github.com/odurgut/clusdr/internal/consensus"
	"github.com/odurgut/clusdr/internal/eventbus"
	"github.com/odurgut/clusdr/internal/grpcserver"
	"github.com/odurgut/clusdr/internal/heartbeat"
	"github.com/odurgut/clusdr/internal/leases"
	"github.com/odurgut/clusdr/internal/locks"
	"github.com/odurgut/clusdr/internal/membership"
	"github.com/odurgut/clusdr/internal/mtls"
	"github.com/odurgut/clusdr/internal/presence"
	"github.com/odurgut/clusdr/internal/store"
	"github.com/odurgut/clusdr/internal/version"
)

// Named unit vars for cmd/clusdr. Declare them here so the command file
// stays a flat, readable list — no logic.
var (
	UnitConfig     = Unit{Name: "config", InitFn: InitConfig}
	UnitLogger     = Unit{Name: "logger", InitFn: InitLogger}
	UnitStore      = Unit{Name: "store", InitFn: InitStore, CloseFn: CloseStore}
	UnitMembership = Unit{Name: "membership", InitFn: InitMembership}
	UnitBus        = Unit{Name: "bus", InitFn: InitBus}
	UnitLocks      = Unit{Name: "locks", InitFn: InitLocks}
	UnitLeases     = Unit{Name: "leases", InitFn: InitLeases}
	UnitConsensus  = Unit{Name: "consensus", InitFn: InitConsensus, CloseFn: CloseConsensus}
	UnitGRPC       = Unit{Name: "grpc", InitFn: InitGRPC, CloseFn: CloseGRPC}
	UnitHeartbeat  = Unit{Name: "heartbeat", InitFn: InitHeartbeat, CloseFn: CloseHeartbeat}
	UnitPresence   = Unit{Name: "presence", InitFn: InitPresence, CloseFn: ClosePresence}

	// Future units:
	//   UnitStore      = Unit{Name: "store",      InitFn: InitStore,      CloseFn: CloseStore}
	//   UnitMembership = Unit{Name: "membership", InitFn: InitMembership, CloseFn: CloseMembership}
	//   UnitRaft       = Unit{Name: "raft",       InitFn: InitRaft,       CloseFn: CloseRaft}
	//   UnitWatch      = Unit{Name: "watch",      InitFn: InitWatch,      CloseFn: CloseWatch}
	//   UnitEvents     = Unit{Name: "events",     InitFn: InitEvents,     CloseFn: CloseEvents}
	//   UnitLocks      = Unit{Name: "locks",      InitFn: InitLocks,      CloseFn: CloseLocks}
	//   UnitLeases     = Unit{Name: "leases",     InitFn: InitLeases,     CloseFn: CloseLeases}
)

// DefaultUnits is the production startup order.
// cmd/clusdr/start.go passes this list to InitApp.
func DefaultUnits() []Unit {
	return []Unit{
		UnitConfig,
		UnitLogger,
		UnitStore,
		UnitMembership,
		UnitBus,       // wires Emit into membership; must be before consensus
		UnitLocks,     // lock table; FSM is the only writer
		UnitLeases,    // lease table; FSM is the only writer
		UnitConsensus, // before GRPC so InitGRPC can pass a.Consensus as VoterAdder
		UnitGRPC,      // needs the Raft node and heartbeat port already bound
		UnitHeartbeat,
		UnitPresence, // presence lease; after gRPC so followers can Grant on the leader
	}
}

// InitConfig loads CLUSDR_* environment variables and the YAML config file
// into a.Cfg. Must be the first unit.
func InitConfig(_ context.Context, a *App) error {
	getenv := a.Opt.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	// Config file path comes from the CLI flag stored in Opt; the zero value
	// ("") means "no file" which is fine — defaults + env still apply.
	cfg, err := config.LoadFrom(a.Opt.ConfigPath, getenv)
	if err != nil {
		return err
	}
	a.Cfg = cfg
	return nil
}

// InitStore opens the BoltDB state database and loads node identity into
// a.Cfg so downstream units (gRPC, membership) see the resolved IDs.
// If the node has not been initialized, it logs a warning and continues —
// the daemon can start without identity but callers will see empty IDs.
func InitStore(_ context.Context, a *App) error {
	s, err := store.Open(a.Cfg.Data.Dir)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	a.Store = s

	nodeID, clusterID, err := s.Identity()
	if err != nil {
		// Not initialized is non-fatal: warn and continue with config values.
		a.Log.Warn("node identity not found in store",
			"hint", "run 'clusdr init' to initialize this node",
			"err", err)
		return nil
	}

	// Store wins over YAML/env if the YAML didn't set explicit IDs,
	// preserving what 'clusdr init' wrote.
	if a.Cfg.Node.ID == "" {
		a.Cfg.Node.ID = nodeID
	}
	if a.Cfg.Cluster.ID == "" {
		a.Cfg.Cluster.ID = clusterID
	}

	a.Log.Info("identity loaded",
		"node_id", a.Cfg.Node.ID,
		"cluster_id", a.Cfg.Cluster.ID,
		"data_dir", a.Cfg.Data.Dir,
	)
	return nil
}

// CloseStore closes the BoltDB state database.
func CloseStore(_ context.Context, a *App) error {
	if a.Store == nil {
		return nil
	}
	err := a.Store.Close()
	a.Store = nil
	return err
}

// InitMembership creates the membership engine with the local node as the
// initial member. Must run after InitStore (needs resolved node/cluster IDs).
func InitMembership(_ context.Context, a *App) error {
	a.Membership = membership.New(a.Cfg.Node.ID, a.Cfg.Node.Addr, a.Log)
	return nil
}

// InitGRPC starts the gRPC server on both the Runtime TCP and Control Unix
// socket endpoints, registers Health/Membership/Join/Watch services, and
// logs the bound addresses. Must run after InitBus (Watch needs the bus).
func InitGRPC(_ context.Context, a *App) error {
	cfg := grpcserver.DefaultConfig()
	tlsMode := "disabled"
	if a.Cfg.TLSEnabled() {
		if _, _, _, err := a.loadTLSMaterial(); err != nil {
			a.Log.Info("tls skipped", "reason", "no certificates")
		} else {
			tlsCfg, err := mtls.ReloadableServerTLS(a.loadTLSMaterial)
			if err != nil {
				return fmt.Errorf("tls: %w", err)
			}
			cfg.TLS = tlsCfg
			tlsMode = "mtls"
			ca, cert, key, _ := a.loadTLSMaterial()
			if err := mtls.WriteFiles(a.Cfg.Data.Dir, ca, cert, key); err != nil {
				a.Log.Warn("write tls files", "err", err)
			}
		}
	}
	srv := grpcserver.New(a.Log, cfg)

	role := "standalone"
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID:    a.Cfg.Node.ID,
		ClusterID: a.Cfg.Cluster.ID,
		Role:      role,
	})
	grpcserver.RegisterMembershipService(srv.GRPCServer(), a.Membership)
	var sec grpcserver.JoinSecurity
	var boot grpcserver.JoinBootstrap
	if a.Store != nil {
		sec = &joinSec{store: a.Store}
		boot = &joinBoot{store: a.Store, dataDir: a.Cfg.Data.Dir}
	}
	peerCreds := a.peerTransportCreds
	var joinCreds = credentialsOrNil(cfg.TLS != nil)
	grpcserver.RegisterJoinService(srv.GRPCServer(), a.Cfg.Cluster.ID, a.Membership, a.Consensus, a.Log, sec, peerCreds)
	grpcserver.RegisterControlService(srv.GRPCServer(), a.Cfg.Cluster.ID, a.Cfg.Raft.Addr, a.Membership, boot, joinCreds, a.Consensus, peerCreds)
	grpcserver.RegisterHeartbeatService(srv.GRPCServer(), a.Cfg.Node.ID)
	if a.Bus != nil {
		grpcserver.RegisterWatchService(srv.GRPCServer(), a.Bus, a.Membership, a.Log)
		grpcserver.RegisterEventService(srv.GRPCServer(), a.Bus, a.Membership, a.Log, peerCreds)
	}
	if a.Locks != nil && a.Membership != nil {
		var raft grpcserver.LockRaft
		if a.Consensus != nil {
			raft = a.Consensus
		}
		grpcserver.RegisterLockService(srv.GRPCServer(), a.Locks, raft, a.Membership, a.Log, peerCreds, a.Cfg.Lock.TTL)
	}
	if a.Leases != nil && a.Membership != nil {
		var raft grpcserver.LeaseRaft
		if a.Consensus != nil {
			raft = a.Consensus
		}
		grpcserver.RegisterLeaseService(srv.GRPCServer(), a.Leases, raft, a.Membership, a.Log, peerCreds, a.Cfg.Lease.TTL)
	}

	// Runtime API — TCP (applications connect here)
	runtimeLn := a.Opt.GRPCListener
	if runtimeLn == nil {
		ln, err := net.Listen("tcp", a.Cfg.GRPC.Addr)
		if err != nil {
			return fmt.Errorf("listen grpc runtime %s: %w", a.Cfg.GRPC.Addr, err)
		}
		runtimeLn = ln
	}

	// Control API — Unix socket (CLI connects here)
	controlLn := a.Opt.ControlListener
	if controlLn == nil {
		ln, err := net.Listen("unix", a.Cfg.GRPC.ControlSocket)
		if err != nil {
			// Non-fatal in dev: socket may require /var/run.
			// Log a warning and skip; CLI status will report "not running".
			a.Log.Warn("control socket unavailable, skipping",
				"socket", a.Cfg.GRPC.ControlSocket, "err", err)
		} else {
			controlLn = ln
		}
	}

	a.GRPC = srv

	a.StartServe(func(ln net.Listener) error { return srv.Serve(ln) }, runtimeLn)
	if controlLn != nil {
		a.StartServe(func(ln net.Listener) error { return srv.Serve(ln) }, controlLn)
	}

	a.Log.Info("clusdr ready",
		"version", version.Version,
		"commit", version.Commit,
		"grpc", runtimeLn.Addr().String(),
		"tls", tlsMode,
	)
	return nil
}

// InitHeartbeat starts the peer liveness monitor. Must run after InitGRPC
// (the gRPC server must be listening before peers can ping us back).
func InitHeartbeat(_ context.Context, a *App) error {
	cfg := heartbeat.Config{
		Interval:  a.Cfg.Heartbeat.Interval,
		Timeout:   a.Cfg.Heartbeat.Timeout,
		MaxMisses: a.Cfg.Heartbeat.MaxMisses,
	}
	m := heartbeat.New(cfg, a.Membership, a.Log)
	m.SetPeerCreds(a.peerTransportCreds)
	m.SetReportDead(a.removeDeadMember)
	m.Start()
	a.Heartbeat = m
	return nil
}

// CloseHeartbeat stops the peer liveness monitor.
func CloseHeartbeat(_ context.Context, a *App) error {
	if a.Heartbeat == nil {
		return nil
	}
	a.Heartbeat.Stop()
	a.Heartbeat = nil
	return nil
}

// InitPresence starts the local presence lease and watches lease.expired
// so a dead node is removed without waiting for heartbeat misses.
func InitPresence(_ context.Context, a *App) error {
	if !a.Cfg.Lease.Presence {
		return nil
	}
	if a.Consensus == nil || a.Membership == nil {
		return nil
	}

	holder := &presence.RaftHolder{
		Raft:      a.Consensus,
		Nodes:     leaderFinder{e: a.Membership},
		PeerCreds: a.peerTransportCreds,
	}
	k := presence.NewKeeper(presence.Config{
		NodeID:         a.Cfg.Node.ID,
		TTL:            a.Cfg.Lease.PresenceTTL,
		RequestTimeout: a.Cfg.GRPC.RequestTimeout,
	}, holder, a.Log)

	ctl := &presenceCtl{keeper: k, done: make(chan struct{})}
	if a.Bus != nil {
		ctx, cancel := context.WithCancel(context.Background())
		ctl.cancel = cancel
		ctl.sub = a.Bus.Subscribe(64)
		go func() {
			defer close(ctl.done)
			presence.WatchExpired(ctx, ctl.sub, a.Cfg.Node.ID, a.Consensus.IsLeader, a.removeDeadMember)
		}()
	} else {
		close(ctl.done)
	}
	k.Start()
	a.presence = ctl
	a.Log.Info("presence lease enabled", "ttl", a.Cfg.Lease.PresenceTTL)
	return nil
}

// ClosePresence stops renewing the local presence lease.
func ClosePresence(_ context.Context, a *App) error {
	if a.presence == nil {
		return nil
	}
	a.presence.stop()
	a.presence = nil
	return nil
}

// removeDeadMember commits a Raft leave for id. Leader-only; no-op for self.
func (a *App) removeDeadMember(id string) {
	if id == "" || a.Consensus == nil || !a.Consensus.IsLeader() {
		if a.Log != nil {
			a.Log.Debug("peer unreachable; waiting for leader leave commit", "node_id", id)
		}
		return
	}
	if a.Membership != nil && id == a.Membership.SelfID() {
		if a.Log != nil {
			a.Log.Warn("refusing to remove self from cluster", "node_id", id)
		}
		return
	}
	if err := a.Consensus.RemoveVoter(id); err != nil && a.Log != nil {
		a.Log.Warn("remove raft voter failed", "node_id", id, "err", err)
	}
	if err := a.Consensus.ApplyRemoveMember(id); err != nil && a.Log != nil {
		a.Log.Warn("apply remove_member failed", "node_id", id, "err", err)
	}
}

type leaderFinder struct {
	e *membership.Engine
}

func (l leaderFinder) SelfID() string { return l.e.SelfID() }

func (l leaderFinder) Leader() (string, string, bool) {
	m, ok := l.e.Leader()
	if !ok {
		return "", "", false
	}
	return m.ID, m.Address, true
}

type presenceCtl struct {
	keeper *presence.Keeper
	sub    *eventbus.Subscription
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *presenceCtl) stop() {
	if p == nil {
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	if p.sub != nil {
		p.sub.Unsubscribe()
	}
	if p.done != nil {
		<-p.done
	}
	if p.keeper != nil {
		p.keeper.Stop()
	}
}

// InitBus creates the event bus and wires it into the membership engine so that
// Join, Leave, and SetLeader calls automatically publish events.
// Must run after UnitMembership (engine must exist) and before UnitConsensus
// (consensus subscribes to member.left events).
func InitBus(_ context.Context, a *App) error {
	b := eventbus.New()
	a.Bus = b
	a.Membership.Emit = b.Publish
	return nil
}

// InitLocks creates the in-memory lock table. Raft FSM is the only writer.
func InitLocks(_ context.Context, a *App) error {
	a.Locks = locks.New()
	if a.Bus != nil {
		a.Locks.Emit = a.Bus.Publish
	}
	return nil
}

// InitLeases creates the in-memory lease table. Raft FSM is the only writer.
func InitLeases(_ context.Context, a *App) error {
	a.Leases = leases.New()
	if a.Bus != nil {
		a.Leases.Emit = a.Bus.Publish
	}
	return nil
}

// InitConsensus creates and starts the Raft node. Runs before UnitGRPC so the
// gRPC registration can pass a.Consensus as a VoterAdder to RegisterJoinService.
func InitConsensus(_ context.Context, a *App) error {
	cfg := consensus.Config{
		Addr:               a.Cfg.Raft.Addr,
		Bootstrap:          a.Cfg.Raft.Bootstrap,
		HeartbeatTimeout:   a.Cfg.Raft.HeartbeatTimeout,
		ElectionTimeout:    a.Cfg.Raft.ElectionTimeout,
		LeaderLeaseTimeout: a.Cfg.Raft.LeaderLeaseTimeout,
	}
	node, err := consensus.New(cfg, a.Cfg.Node.ID, a.Cfg.Data.Dir, a.Membership, a.Locks, a.Leases, a.Log)
	if err != nil {
		return fmt.Errorf("consensus init: %w", err)
	}

	mem := a.Membership

	selfID := a.Cfg.Node.ID
	selfAddr := a.Cfg.GRPC.Addr // gRPC address peers use to reach this node

	// WatchLeaderChange fires on ALL nodes (leader and followers) whenever the
	// cluster leader changes. Every node updates its membership engine so
	// clusdr leader reflects the current Raft leader.
	node.WatchLeaderChange(func(leaderID string) {
		if leaderID == "" {
			a.Log.Info("raft: no leader")
			return
		}
		mem.SetLeader(leaderID)

		// When THIS node becomes leader, ensure its own membership entry is
		// in the Raft log. The bootstrap node was never added via join, so
		// a follower restarting from the log would otherwise miss it.
		// ApplyAddMember is idempotent: if the entry already exists the FSM
		// returns isNew=false and discards it silently.
		if leaderID == selfID {
			if err := node.ApplyAddMember(selfID, selfAddr); err != nil {
				a.Log.Warn("failed to apply self add_member", "err", err)
			}
		}
	})

	a.Consensus = node
	go locks.RunExpirer(node.Ctx(), a.Locks, node.IsLeader, node.ApplyLockExpire, a.Cfg.Lock.ExpireInterval, a.Log)
	go leases.RunExpirer(node.Ctx(), a.Leases, node.IsLeader, node.ApplyLeaseExpire, a.Cfg.Lease.ExpireInterval, a.Log)
	return nil
}

// CloseConsensus shuts down the Raft node cleanly.
func CloseConsensus(_ context.Context, a *App) error {
	if a.Consensus == nil {
		return nil
	}
	if err := a.Consensus.Shutdown(); err != nil {
		return fmt.Errorf("consensus shutdown: %w", err)
	}
	a.Consensus = nil
	return nil
}

// CloseGRPC gracefully stops the gRPC server.
func CloseGRPC(ctx context.Context, a *App) error {
	if a.GRPC == nil {
		return nil
	}
	err := a.GRPC.Shutdown(ctx)
	a.GRPC = nil
	return err
}

// InitLogger builds the slog.Logger and stores it in a.Log.
// Must run after InitConfig so log level/format are resolved.
func InitLogger(_ context.Context, a *App) error {
	out := a.Opt.LogOutput
	if out == nil {
		out = os.Stderr
	}

	var level slog.Level
	switch a.Cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if a.Cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	log := slog.New(handler)
	if log == nil {
		return fmt.Errorf("logger: nil handler")
	}
	a.Log = log
	return nil
}

func (a *App) loadTLSMaterial() (ca, cert, key []byte, err error) {
	if a == nil || a.Store == nil {
		return nil, nil, nil, fmt.Errorf("no store")
	}
	c, err := a.Store.Certs()
	if err != nil {
		return nil, nil, nil, err
	}
	if len(c.CACert) == 0 || len(c.NodeCert) == 0 || len(c.NodeKey) == 0 {
		return nil, nil, nil, fmt.Errorf("incomplete certificates")
	}
	return c.CACert, c.NodeCert, c.NodeKey, nil
}

func (a *App) peerTransportCreds() credentials.TransportCredentials {
	if a == nil || !a.Cfg.TLSEnabled() {
		return nil
	}
	ca, cert, key, err := a.loadTLSMaterial()
	if err != nil {
		return nil
	}
	tc, err := mtls.ClientTLS(ca, cert, key)
	if err != nil {
		if a.Log != nil {
			a.Log.Warn("mtls client creds", "err", err)
		}
		return nil
	}
	return tc
}

func credentialsOrNil(tlsOn bool) credentials.TransportCredentials {
	if !tlsOn {
		return nil
	}
	return mtls.BootstrapTLS()
}
