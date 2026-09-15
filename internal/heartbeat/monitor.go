// Package heartbeat implements peer liveness detection.
//
// Monitor ticks on a configurable interval and pings all known alive peers.
// Consecutive failures beyond MaxMisses cause the peer to be marked leaving
// via membership.Engine.Leave — which emits member.left.
package heartbeat

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/mtls"
	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

// Config holds liveness probe parameters.
type Config struct {
	Interval  time.Duration
	Timeout   time.Duration
	MaxMisses int
}

// Leaver is the subset of membership.Engine used by the monitor.
type Leaver interface {
	Members() []membership.Member
	Leave(id string)
	SelfID() string
}

// Monitor runs a background ticker that pings known peers.
// It is safe for concurrent use.
type Monitor struct {
	cfg    Config
	engine Leaver
	log    *slog.Logger

	mu     sync.Mutex
	misses map[string]int // node_id → consecutive miss count
	creds  func() credentials.TransportCredentials
	// reportDead, when set, is called instead of engine.Leave. Production
	// wiring commits remove_member on the Raft leader; unit tests leave this
	// nil so the engine is updated directly.
	reportDead func(id string)

	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a Monitor. Call Start to begin pinging.
func New(cfg Config, engine Leaver, log *slog.Logger) *Monitor {
	return &Monitor{
		cfg:    cfg,
		engine: engine,
		log:    log,
		misses: make(map[string]int),
		done:   make(chan struct{}),
	}
}

// SetPeerCreds sets the credentials used to ping peers. nil (or a nil
// return) dials insecure — the default, used by unit tests.
func (m *Monitor) SetPeerCreds(fn func() credentials.TransportCredentials) {
	m.creds = fn
}

// SetReportDead sets the callback invoked after MaxMisses. nil → engine.Leave.
func (m *Monitor) SetReportDead(fn func(id string)) {
	m.reportDead = fn
}

// Start launches the background ping loop. Returns immediately.
func (m *Monitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.run(ctx)
}

// Stop cancels the ping loop and waits for it to exit.
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
}

func (m *Monitor) run(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

// tick pings all alive peers sequentially.
// Sequential is intentional: small clusters (≤25) and short timeouts make
// parallel pinging unnecessary.
func (m *Monitor) tick(ctx context.Context) {
	selfID := m.engine.SelfID()
	for _, mem := range m.engine.Members() {
		if mem.ID == selfID {
			continue
		}
		if mem.Status != membership.StatusAlive {
			continue
		}

		pingCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
		err := m.ping(pingCtx, mem.Address)
		cancel()

		m.mu.Lock()
		if err != nil {
			m.misses[mem.ID]++
			m.log.Debug("heartbeat miss",
				"node_id", mem.ID,
				"misses", m.misses[mem.ID],
				"max", m.cfg.MaxMisses,
				"err", err,
			)
			if m.misses[mem.ID] >= m.cfg.MaxMisses {
				m.log.Warn("peer unreachable, marking leaving",
					"node_id", mem.ID,
					"misses", m.misses[mem.ID],
				)
				if m.reportDead != nil {
					m.reportDead(mem.ID)
				} else {
					m.engine.Leave(mem.ID)
				}
				delete(m.misses, mem.ID)
			}
		} else {
			delete(m.misses, mem.ID) // reset on success
		}
		m.mu.Unlock()
	}
}

func (m *Monitor) ping(ctx context.Context, addr string) error {
	var creds credentials.TransportCredentials
	if m.creds != nil {
		creds = m.creds()
	}
	conn, err := mtls.Dial(addr, creds)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	_, err = pb.NewHeartbeatServiceClient(conn).Ping(ctx, &pb.PingRequest{
		SenderId: m.engine.SelfID(),
	})
	return err
}
