package soak

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"time"

	raftlib "github.com/hashicorp/raft"

	"github.com/clusdr/clusdr/internal/consensus"
	"github.com/clusdr/clusdr/internal/leases"
	"github.com/clusdr/clusdr/internal/locks"
	"github.com/clusdr/clusdr/internal/membership"
)

// Generous vs production LAN (150/75ms): -race + CI miss the leader lease
// the same way chaos tests do.
const (
	soakHeartbeat = 300 * time.Millisecond
	soakElection  = 300 * time.Millisecond
	soakLease     = 150 * time.Millisecond
)

type node struct {
	id     string
	raft   *consensus.Node
	mem    *membership.Engine
	locks  *locks.Table
	leases *leases.Table
	trans  *raftlib.InmemTransport
}

type cluster struct {
	core []*node
	log  *slog.Logger
}

func connect(a, b *raftlib.InmemTransport) {
	a.Connect(b.LocalAddr(), b)
	b.Connect(a.LocalAddr(), a)
}

func disconnect(a, b *raftlib.InmemTransport) {
	a.Disconnect(b.LocalAddr())
	b.Disconnect(a.LocalAddr())
}

func startCluster(dir string, n int, log *slog.Logger) (*cluster, error) {
	if n < 3 {
		return nil, fmt.Errorf("raft cluster needs at least 3 nodes, got %d", n)
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
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

	c := &cluster{log: log}
	for i, id := range ids {
		nd, err := startNode(filepath.Join(dir, id), id, i == 0, trans[i], log)
		if err != nil {
			_ = c.close()
			return nil, err
		}
		c.core = append(c.core, nd)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := waitLeaderOf(ctx, c.core[:1]...); err != nil {
		_ = c.close()
		return nil, fmt.Errorf("bootstrap leader: %w", err)
	}
	for _, nd := range c.core[1:] {
		nd := nd
		if err := c.withAnyLeader(ctx, func(lead *node) error {
			return lead.raft.AddVoter(nd.id, string(nd.trans.LocalAddr()))
		}); err != nil {
			_ = c.close()
			return nil, fmt.Errorf("add voter %s: %w", nd.id, err)
		}
	}
	if _, err := c.waitLeader(ctx); err != nil {
		_ = c.close()
		return nil, err
	}
	for _, nd := range c.core {
		nd := nd
		if err := c.withLeader(ctx, func(lead *node) error {
			return lead.raft.ApplyAddMember(nd.id, string(nd.trans.LocalAddr()))
		}); err != nil {
			_ = c.close()
			return nil, fmt.Errorf("add member %s: %w", nd.id, err)
		}
	}
	if err := c.waitAlive(ctx, ids...); err != nil {
		_ = c.close()
		return nil, err
	}
	return c, nil
}

func startNode(dir, id string, bootstrap bool, trans *raftlib.InmemTransport, log *slog.Logger) (*node, error) {
	mem := membership.New(id, string(trans.LocalAddr()), log)
	lockTab := locks.New()
	leaseTab := leases.New()
	cfg := consensus.Config{
		Bootstrap:          bootstrap,
		Transport:          trans,
		HeartbeatTimeout:   soakHeartbeat,
		ElectionTimeout:    soakElection,
		LeaderLeaseTimeout: soakLease,
		LogOutput:          io.Discard,
	}
	r, err := consensus.New(cfg, id, dir, mem, lockTab, leaseTab, log)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	return &node{id: id, raft: r, mem: mem, locks: lockTab, leases: leaseTab, trans: trans}, nil
}

func (c *cluster) close() error {
	if c == nil {
		return nil
	}
	var first error
	for _, n := range c.core {
		if n == nil || n.raft == nil {
			continue
		}
		if err := n.raft.Shutdown(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *cluster) startExpirers(ctx context.Context) {
	for _, n := range c.core {
		n := n
		go locks.RunExpirer(ctx, n.locks, n.raft.IsLeader, n.raft.ApplyLockExpire, 50*time.Millisecond, c.log)
		go leases.RunExpirer(ctx, n.leases, n.raft.IsLeader, n.raft.ApplyLeaseExpire, 50*time.Millisecond, c.log)
	}
}

func (c *cluster) waitLeader(ctx context.Context) (*node, error) {
	return waitLeaderOf(ctx, c.core...)
}

func waitLeaderOf(ctx context.Context, nodes ...*node) (*node, error) {
	stable := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for leader: %w", err)
		}
		var leaders []*node
		for _, n := range nodes {
			if n.raft.IsLeader() {
				leaders = append(leaders, n)
			}
		}
		if len(leaders) == 1 {
			id := leaders[0].id
			agree := true
			for _, n := range nodes {
				if n.raft.LeaderID() != id {
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

func (c *cluster) withAnyLeader(ctx context.Context, fn func(*node) error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		var lead *node
		for _, n := range c.core {
			if n.raft.IsLeader() {
				lead = n
				break
			}
		}
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

func (c *cluster) withLeader(ctx context.Context, fn func(*node) error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		lead, err := c.waitLeader(ctx)
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

func transientLeadership(err error) bool {
	return errors.Is(err, raftlib.ErrLeadershipLost) ||
		errors.Is(err, raftlib.ErrNotLeader) ||
		errors.Is(err, raftlib.ErrLeadershipTransferInProgress)
}

func (c *cluster) joinExtra(ctx context.Context, dir, id string) (*node, error) {
	_, tr := raftlib.NewInmemTransport("")
	for _, n := range c.core {
		connect(tr, n.trans)
	}
	extra, err := startNode(dir, id, false, tr, c.log)
	if err != nil {
		return nil, err
	}
	if err := c.withLeader(ctx, func(lead *node) error {
		return lead.raft.AddVoter(id, string(tr.LocalAddr()))
	}); err != nil {
		_ = extra.raft.Shutdown()
		return nil, fmt.Errorf("add voter %s: %w", id, err)
	}
	if err := c.withLeader(ctx, func(lead *node) error {
		return lead.raft.ApplyAddMember(id, string(tr.LocalAddr()))
	}); err != nil {
		_ = extra.raft.Shutdown()
		return nil, fmt.Errorf("add member %s: %w", id, err)
	}
	if err := c.waitHasMember(ctx, id); err != nil {
		_ = extra.raft.Shutdown()
		return nil, err
	}
	return extra, nil
}

func (c *cluster) leaveExtra(ctx context.Context, extra *node) error {
	id := extra.id
	if err := c.withLeader(ctx, func(lead *node) error {
		return lead.raft.ApplyDropMember(id)
	}); err != nil {
		return fmt.Errorf("drop member %s: %w", id, err)
	}
	if err := c.withLeader(ctx, func(lead *node) error {
		return lead.raft.RemoveVoter(id)
	}); err != nil {
		return fmt.Errorf("remove voter %s: %w", id, err)
	}
	_ = extra.raft.Shutdown()
	for _, n := range c.core {
		disconnect(extra.trans, n.trans)
	}
	if err := c.waitGone(ctx, id); err != nil {
		return err
	}
	ids := make([]string, 0, len(c.core))
	for _, n := range c.core {
		ids = append(ids, n.id)
	}
	return c.waitAlive(ctx, ids...)
}

func (c *cluster) waitAlive(ctx context.Context, ids ...string) error {
	want := append([]string(nil), ids...)
	slices.Sort(want)
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait alive %v: %w", want, err)
		}
		ok := true
		for _, n := range c.core {
			if !slices.Equal(aliveIDs(n.mem), want) {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait alive %v: %w", want, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (c *cluster) waitHasMember(ctx context.Context, id string) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait member %s: %w", id, err)
		}
		ok := true
		for _, n := range c.core {
			if !hasAlive(n.mem, id) {
				ok = false
				break
			}
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait member %s: %w", id, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (c *cluster) waitGone(ctx context.Context, id string) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait gone %s: %w", id, err)
		}
		ok := true
		for _, n := range c.core {
			for _, m := range n.mem.Members() {
				if m.ID == id {
					ok = false
					break
				}
			}
			if !ok {
				break
			}
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait gone %s: %w", id, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (c *cluster) waitLockGone(ctx context.Context, name string) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait lock gone %s: %w", name, err)
		}
		gone := true
		for _, n := range c.core {
			if _, ok := n.locks.Get(name); ok {
				gone = false
				break
			}
		}
		if gone {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait lock gone %s: %w", name, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (c *cluster) waitLeaseGone(ctx context.Context, name string) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait lease gone %s: %w", name, err)
		}
		gone := true
		for _, n := range c.core {
			if _, ok := n.leases.Get(name); ok {
				gone = false
				break
			}
		}
		if gone {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait lease gone %s: %w", name, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func aliveIDs(e *membership.Engine) []string {
	var ids []string
	for _, m := range e.Members() {
		if m.Status == membership.StatusAlive {
			ids = append(ids, m.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func hasAlive(e *membership.Engine, id string) bool {
	for _, m := range e.Members() {
		if m.ID == id && m.Status == membership.StatusAlive {
			return true
		}
	}
	return false
}
