package bench

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	raftlib "github.com/hashicorp/raft"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/locks"
	"github.com/durguto/clusdr/internal/membership"
)

const (
	lanHeartbeat = 150 * time.Millisecond
	lanElection  = 150 * time.Millisecond
	lanLease     = 75 * time.Millisecond
)

// Node is one in-process Raft member.
type Node struct {
	ID    string
	Raft  *consensus.Node
	Mem   *membership.Engine
	Locks *locks.Table
	Trans *raftlib.InmemTransport
}

// Cluster is an in-memory Raft group used by election and lock scenarios.
type Cluster struct {
	Nodes []*Node
	log   *slog.Logger
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func connect(a, b *raftlib.InmemTransport) {
	a.Connect(b.LocalAddr(), b)
	b.Connect(a.LocalAddr(), a)
}

func disconnect(a, b *raftlib.InmemTransport) {
	a.Disconnect(b.LocalAddr())
	b.Disconnect(a.LocalAddr())
}

// StartCluster boots n voting members (n >= 3) on an in-memory transport.
func StartCluster(dir string, n int, log *slog.Logger) (*Cluster, error) {
	if n < 3 {
		return nil, fmt.Errorf("raft cluster needs at least 3 nodes, got %d", n)
	}
	if log == nil {
		log = discardLog()
	}

	trans := make([]*raftlib.InmemTransport, n)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("node-%d", i)
		_, tr := raftlib.NewInmemTransport("")
		trans[i] = tr
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			connect(trans[i], trans[j])
		}
	}

	c := &Cluster{log: log}
	for i, id := range ids {
		node, err := startNode(filepath.Join(dir, id), id, i == 0, trans[i], log)
		if err != nil {
			_ = c.Close()
			return nil, err
		}
		c.Nodes = append(c.Nodes, node)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := waitLeaderOf(ctx, c.Nodes[:1]...); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("bootstrap leader: %w", err)
	}
	for _, n := range c.Nodes[1:] {
		n := n
		if err := c.withAnyLeader(ctx, func(lead *Node) error {
			return lead.Raft.AddVoter(n.ID, string(n.Trans.LocalAddr()))
		}); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("add voter %s: %w", n.ID, err)
		}
	}
	if _, err := c.WaitLeader(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	for _, n := range c.Nodes {
		n := n
		if err := c.withLeader(ctx, func(lead *Node) error {
			return lead.Raft.ApplyAddMember(n.ID, string(n.Trans.LocalAddr()))
		}); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("add member %s: %w", n.ID, err)
		}
	}
	return c, nil
}

func (c *Cluster) withAnyLeader(ctx context.Context, fn func(*Node) error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		lead := c.Leader()
		if lead == nil {
			select {
			case <-ctx.Done():
				if last != nil {
					return last
				}
				return ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		err := fn(lead)
		if err == nil {
			return nil
		}
		last = err
		if !transientLeadership(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(30 * time.Millisecond):
		}
	}
}

func (c *Cluster) withLeader(ctx context.Context, fn func(*Node) error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		lead, err := c.WaitLeader(ctx)
		if err != nil {
			if last != nil {
				return last
			}
			return err
		}
		err = fn(lead)
		if err == nil {
			return nil
		}
		last = err
		if !transientLeadership(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(30 * time.Millisecond):
		}
	}
}

func startNode(dir, id string, bootstrap bool, trans *raftlib.InmemTransport, log *slog.Logger) (*Node, error) {
	mem := membership.New(id, string(trans.LocalAddr()), log)
	tab := locks.New()
	leasesTab := leases.New()
	cfg := consensus.Config{
		Bootstrap:          bootstrap,
		Transport:          trans,
		HeartbeatTimeout:   lanHeartbeat,
		ElectionTimeout:    lanElection,
		LeaderLeaseTimeout: lanLease,
		LogOutput:          io.Discard,
	}
	r, err := consensus.New(cfg, id, dir, mem, tab, leasesTab, log)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	return &Node{ID: id, Raft: r, Mem: mem, Locks: tab, Trans: trans}, nil
}

// Close shuts every Raft node down.
func (c *Cluster) Close() error {
	if c == nil {
		return nil
	}
	var first error
	for _, n := range c.Nodes {
		if n == nil || n.Raft == nil {
			continue
		}
		if err := n.Raft.Shutdown(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Leader returns the current Raft leader, or nil.
func (c *Cluster) Leader() *Node {
	for _, n := range c.Nodes {
		if n.Raft.IsLeader() {
			return n
		}
	}
	return nil
}

// WaitLeader blocks until exactly one live node is leader and the others agree.
func (c *Cluster) WaitLeader(ctx context.Context) (*Node, error) {
	return waitLeaderOf(ctx, c.Nodes...)
}

func waitLeaderOf(ctx context.Context, nodes ...*Node) (*Node, error) {
	stable := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for leader: %w", err)
		}
		var leaders []*Node
		for _, n := range nodes {
			if n.Raft.IsLeader() {
				leaders = append(leaders, n)
			}
		}
		if len(leaders) == 1 {
			id := leaders[0].ID
			agree := true
			for _, n := range nodes {
				if n.Raft.LeaderID() != id {
					agree = false
					break
				}
			}
			if agree {
				stable++
				if stable >= 3 {
					return leaders[0], nil
				}
			} else {
				stable = 0
			}
		} else {
			stable = 0
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for leader: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// Isolate cuts id off from every other peer.
func (c *Cluster) Isolate(id string) error {
	cut, err := c.byID(id)
	if err != nil {
		return err
	}
	for _, n := range c.Nodes {
		if n.ID == id {
			continue
		}
		disconnect(cut.Trans, n.Trans)
	}
	return nil
}

// Heal restores id to the full mesh.
func (c *Cluster) Heal(id string) error {
	cut, err := c.byID(id)
	if err != nil {
		return err
	}
	for _, n := range c.Nodes {
		if n.ID == id {
			continue
		}
		connect(cut.Trans, n.Trans)
	}
	return nil
}

func (c *Cluster) byID(id string) (*Node, error) {
	for _, n := range c.Nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, fmt.Errorf("unknown node %s", id)
}

func (c *Cluster) except(id string) []*Node {
	var out []*Node
	for _, n := range c.Nodes {
		if n.ID != id {
			out = append(out, n)
		}
	}
	return out
}
