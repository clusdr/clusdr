package consensus_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	raftlib "github.com/hashicorp/raft"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/locks"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/presence"
)

// Chaos tests run a real 3-voter Raft cluster on an in-memory transport.
// Peers can be killed or partitioned without iptables; production still uses TCP.

const (
	// Generous vs production LAN (150/75ms): GitHub Actions + -race
	// routinely misses the leader lease and flakes Apply.
	chaosHeartbeat = 300 * time.Millisecond
	chaosElection  = 300 * time.Millisecond
	chaosLease     = 150 * time.Millisecond
)

type chaosNode struct {
	id     string
	node   *consensus.Node
	mem    *membership.Engine
	locks  *locks.Table
	leases *leases.Table
	bus    *eventbus.Bus
	trans  *raftlib.InmemTransport
}

type chaosCluster struct {
	t     *testing.T
	nodes []*chaosNode
	dead  map[string]bool
}

func connectInmem(a, b *raftlib.InmemTransport) {
	a.Connect(b.LocalAddr(), b)
	b.Connect(a.LocalAddr(), a)
}

func disconnectInmem(a, b *raftlib.InmemTransport) {
	a.Disconnect(b.LocalAddr())
	b.Disconnect(a.LocalAddr())
}

func fastRaft(bootstrap bool, trans raftlib.Transport) consensus.Config {
	return consensus.Config{
		Bootstrap:          bootstrap,
		Transport:          trans,
		HeartbeatTimeout:   chaosHeartbeat,
		ElectionTimeout:    chaosElection,
		LeaderLeaseTimeout: chaosLease,
	}
}

func startChaosNode(t *testing.T, id string, bootstrap bool, trans *raftlib.InmemTransport) *chaosNode {
	t.Helper()
	n := &chaosNode{
		id:     id,
		mem:    membership.New(id, string(trans.LocalAddr()), nopLog()),
		locks:  locks.New(),
		leases: leases.New(),
		bus:    eventbus.New(),
		trans:  trans,
	}
	n.mem.Emit = n.bus.Publish
	n.locks.Emit = n.bus.Publish
	n.leases.Emit = n.bus.Publish

	node, err := consensus.New(fastRaft(bootstrap, trans), id, t.TempDir(), n.mem, n.locks, n.leases, nopLog())
	if err != nil {
		t.Fatalf("%s: new raft: %v", id, err)
	}
	n.node = node
	t.Cleanup(func() { _ = node.Shutdown() })
	return n
}

func startTriad(t *testing.T) *chaosCluster {
	t.Helper()
	ids := []string{"node-a", "node-b", "node-c"}
	trans := make([]*raftlib.InmemTransport, len(ids))
	for i := range ids {
		_, tr := raftlib.NewInmemTransport("")
		trans[i] = tr
	}
	for i := range trans {
		for j := i + 1; j < len(trans); j++ {
			connectInmem(trans[i], trans[j])
		}
	}

	c := &chaosCluster{t: t, dead: make(map[string]bool)}
	for i, id := range ids {
		c.nodes = append(c.nodes, startChaosNode(t, id, i == 0, trans[i]))
	}

	waitLeader(t, c.nodes[0].node)
	for _, n := range c.nodes[1:] {
		n := n
		if err := c.withAnyLeader(func(lead *consensus.Node) error {
			return lead.AddVoter(n.id, string(n.trans.LocalAddr()))
		}); err != nil {
			t.Fatalf("add voter %s: %v", n.id, err)
		}
	}
	c.waitStableLeader()

	for _, n := range c.nodes {
		n := n
		if err := c.withLeader(func(lead *consensus.Node) error {
			return lead.ApplyAddMember(n.id, string(n.trans.LocalAddr()))
		}); err != nil {
			t.Fatalf("add member %s: %v", n.id, err)
		}
	}
	c.waitAlive(ids...)
	return c
}

func (c *chaosCluster) withAnyLeader(fn func(*consensus.Node) error) error {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		var lead *chaosNode
		for _, n := range c.live() {
			if n.node.IsLeader() {
				lead = n
				break
			}
		}
		if lead == nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		err := fn(lead.node)
		if err == nil {
			return nil
		}
		last = err
		if !transientLeadership(err) {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return last
}

func (c *chaosCluster) withLeader(fn func(*consensus.Node) error) error {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		lead := c.waitStableLeader()
		err := fn(lead.node)
		if err == nil {
			return nil
		}
		last = err
		if !transientLeadership(err) {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return last
}

func (c *chaosCluster) live() []*chaosNode {
	var out []*chaosNode
	for _, n := range c.nodes {
		if !c.dead[n.id] {
			out = append(out, n)
		}
	}
	return out
}

func (c *chaosCluster) byID(id string) *chaosNode {
	c.t.Helper()
	for _, n := range c.nodes {
		if n.id == id {
			return n
		}
	}
	c.t.Fatalf("unknown node %s", id)
	return nil
}

func (c *chaosCluster) mustLeader() *chaosNode {
	c.t.Helper()
	for _, n := range c.live() {
		if n.node.IsLeader() {
			return n
		}
	}
	c.t.Fatal("no leader among live nodes")
	return nil
}

func (c *chaosCluster) waitStableLeader() *chaosNode {
	c.t.Helper()
	return waitLeaderOf(c.t, c.live()...)
}

func waitLeaderOf(t *testing.T, nodes ...*chaosNode) *chaosNode {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	stable := 0
	for time.Now().Before(deadline) {
		var leaders []*chaosNode
		for _, n := range nodes {
			if n.node.IsLeader() {
				leaders = append(leaders, n)
			}
		}
		if len(leaders) == 1 {
			id := leaders[0].id
			agree := true
			for _, n := range nodes {
				if n.node.LeaderID() != id {
					agree = false
					break
				}
			}
			if agree {
				stable++
				if stable >= 3 {
					return leaders[0]
				}
				time.Sleep(20 * time.Millisecond)
				continue
			}
		}
		stable = 0
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no stable leader")
	return nil
}

func (c *chaosCluster) waitLeaderAmong(ids ...string) *chaosNode {
	c.t.Helper()
	want := append([]string(nil), ids...)
	deadline := time.Now().Add(10 * time.Second)
	stable := 0
	for time.Now().Before(deadline) {
		var leaders []*chaosNode
		for _, n := range c.live() {
			if !slices.Contains(want, n.id) {
				continue
			}
			if n.node.IsLeader() {
				leaders = append(leaders, n)
			}
		}
		if len(leaders) == 1 {
			id := leaders[0].id
			agree := true
			for _, n := range c.live() {
				if !slices.Contains(want, n.id) {
					continue
				}
				if n.node.LeaderID() != id {
					agree = false
					break
				}
			}
			if agree {
				stable++
				if stable >= 3 {
					return leaders[0]
				}
				time.Sleep(20 * time.Millisecond)
				continue
			}
		}
		stable = 0
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("no stable leader among %v", ids)
	return nil
}

func (c *chaosCluster) kill(n *chaosNode) {
	c.t.Helper()
	if err := n.node.Shutdown(); err != nil {
		c.t.Logf("shutdown %s: %v", n.id, err)
	}
	c.dead[n.id] = true
}

func (c *chaosCluster) isolate(id string) {
	c.t.Helper()
	cut := c.byID(id)
	for _, n := range c.live() {
		if n.id == id {
			continue
		}
		disconnectInmem(cut.trans, n.trans)
	}
}

func (c *chaosCluster) heal(id string) {
	c.t.Helper()
	cut := c.byID(id)
	for _, n := range c.live() {
		if n.id == id {
			continue
		}
		connectInmem(cut.trans, n.trans)
	}
}

func (c *chaosCluster) waitAlive(ids ...string) {
	c.t.Helper()
	want := append([]string(nil), ids...)
	slices.Sort(want)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range c.live() {
			if !slices.Equal(aliveIDs(n.mem), want) {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, n := range c.live() {
		c.t.Logf("%s alive=%v", n.id, aliveIDs(n.mem))
	}
	c.t.Fatalf("membership not consistent; want alive %v", want)
}

func (c *chaosCluster) waitRole(id, role string) {
	c.t.Helper()
	want := membership.NormalizeRole(role)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range c.live() {
			found := false
			for _, m := range n.mem.Members() {
				if m.ID == id && membership.NormalizeRole(m.Role) == want {
					found = true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, n := range c.live() {
		c.t.Logf("%s role(%s)=%q", n.id, id, observerRoleOn(c.t, n, id))
	}
	c.t.Fatalf("member %s role not %s on all live nodes", id, want)
}

func (c *chaosCluster) waitHasMember(id string) {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range c.live() {
			if !hasAlive(n.mem, id) {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("member %s not alive on all live nodes", id)
}

func (c *chaosCluster) waitLockHeld(name, holder string) {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range c.live() {
			rec, held := n.locks.Get(name)
			if !held || rec.Holder != holder {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("lock %q holder %s not on all live nodes", name, holder)
}

func (c *chaosCluster) waitLockGone(name string) {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		gone := true
		for _, n := range c.live() {
			if _, ok := n.locks.Get(name); ok {
				gone = false
				break
			}
		}
		if gone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("lock %q still held after TTL", name)
}

func transientLeadership(err error) bool {
	return errors.Is(err, raftlib.ErrLeadershipLost) ||
		errors.Is(err, raftlib.ErrNotLeader) ||
		errors.Is(err, raftlib.ErrLeadershipTransferInProgress)
}

func (c *chaosCluster) acquireAfterFailover(name, holder string, ttl time.Duration) (uint64, bool, error) {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var lastOK bool
	var lastErr error
	for time.Now().Before(deadline) {
		lead := c.waitStableLeader()
		tok, ok, err := lead.node.ApplyLockAcquire(name, holder, ttl)
		if err == nil && ok {
			return tok, true, nil
		}
		lastOK, lastErr = ok, err
		if err != nil && !transientLeadership(err) {
			return 0, ok, err
		}
		time.Sleep(30 * time.Millisecond)
	}
	return 0, lastOK, lastErr
}

func (c *chaosCluster) waitLeaseGone(name string) {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		gone := true
		for _, n := range c.live() {
			if _, ok := n.leases.Get(name); ok {
				gone = false
				break
			}
		}
		if gone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("lease %q still held after TTL", name)
}

func (c *chaosCluster) waitLeaving(id string) {
	c.t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, n := range c.live() {
			found := false
			for _, m := range n.mem.Members() {
				if m.ID == id && m.Status == membership.StatusLeaving {
					found = true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("%s not leaving on all live nodes", id)
}

func (c *chaosCluster) startLockExpirers(ctx context.Context) {
	for _, n := range c.nodes {
		n := n
		go locks.RunExpirer(ctx, n.locks, n.node.IsLeader, n.node.ApplyLockExpire, 50*time.Millisecond, nopLog())
	}
}

func (c *chaosCluster) startLeaseExpirers(ctx context.Context) {
	for _, n := range c.nodes {
		n := n
		go leases.RunExpirer(ctx, n.leases, n.node.IsLeader, n.node.ApplyLeaseExpire, 50*time.Millisecond, nopLog())
		sub := n.bus.Subscribe(32)
		go presence.WatchExpired(ctx, sub, n.id, n.node.IsLeader, func(id string) {
			if !n.node.IsLeader() || id == n.id {
				return
			}
			_ = n.node.RemoveVoter(id)
			_ = n.node.ApplyRemoveMember(id)
		})
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

// TestChaos_KillLeaderMidWrite: a committed write survives leader death;
// an in-flight apply may fail; the new leader accepts writes.
func TestChaos_KillLeaderMidWrite(t *testing.T) {
	c := startTriad(t)
	lead := c.mustLeader()

	if _, ok, err := lead.node.ApplyLockAcquire("committed", "holder-a", time.Hour); err != nil || !ok {
		t.Fatalf("committed lock: ok=%v err=%v", ok, err)
	}
	if err := lead.node.ApplyAddMember("meta-1", "10.0.0.1:1"); err != nil {
		t.Fatalf("committed member: %v", err)
	}
	c.waitHasMember("meta-1")
	c.waitLockHeld("committed", "holder-a")

	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(i int) {
			done <- lead.node.ApplyAddMember("inflight-"+strconv.Itoa(i), "10.0.0.2:1")
		}(i)
	}
	c.kill(lead)

	surviving := c.waitStableLeader()
	c.waitLockHeld("committed", "holder-a")
	if !hasAlive(surviving.mem, "meta-1") {
		t.Fatal("committed member lost after leader death")
	}
	for _, n := range c.live() {
		if !hasAlive(n.mem, "meta-1") {
			t.Fatalf("%s lost committed member after failover", n.id)
		}
	}

	if err := surviving.node.ApplyAddMember("after-failover", "10.0.0.3:1"); err != nil {
		t.Fatalf("write after failover: %v", err)
	}
	c.waitHasMember("after-failover")

	// Drain in-flight applies; success or leadership loss are both valid.
	timeout := time.After(6 * time.Second)
	for i := 0; i < 8; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("in-flight apply did not return after leader kill")
		}
	}
}

// TestChaos_PartitionAndHeal: the majority stays writable; the minority
// is stale until the partition heals, then it catches up.
func TestChaos_PartitionAndHeal(t *testing.T) {
	c := startTriad(t)
	c.isolate("node-c")

	majority := c.waitLeaderAmong("node-a", "node-b")
	if err := majority.node.ApplyAddMember("parted", "10.0.0.4:1"); err != nil {
		t.Fatalf("majority write: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hasAlive(c.byID("node-a").mem, "parted") && hasAlive(c.byID("node-b").mem, "parted") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !hasAlive(c.byID("node-a").mem, "parted") || !hasAlive(c.byID("node-b").mem, "parted") {
		t.Fatal("majority did not apply write during partition")
	}
	if hasAlive(c.byID("node-c").mem, "parted") {
		t.Fatal("minority applied majority write while partitioned")
	}

	c.heal("node-c")
	c.waitStableLeader()
	c.waitHasMember("parted")
}

// TestChaos_RapidJoinLeave: churning extra voters leaves the core
// membership identical on every survivor.
func TestChaos_RapidJoinLeave(t *testing.T) {
	c := startTriad(t)
	const cycles = 6
	for i := 0; i < cycles; i++ {
		id := "extra-" + strconv.Itoa(i)
		_, tr := raftlib.NewInmemTransport("")
		for _, n := range c.live() {
			connectInmem(tr, n.trans)
		}
		extra := startChaosNode(t, id, false, tr)

		lead := c.waitStableLeader()
		if err := lead.node.AddVoter(id, string(tr.LocalAddr())); err != nil {
			t.Fatalf("cycle %d add voter: %v", i, err)
		}
		voters := append(c.live(), extra)
		lead = waitLeaderOf(t, voters...)
		if err := lead.node.ApplyAddMember(id, string(tr.LocalAddr())); err != nil {
			t.Fatalf("cycle %d add member: %v", i, err)
		}
		c.waitHasMember(id)

		lead = waitLeaderOf(t, voters...)
		if err := lead.node.RemoveVoter(id); err != nil {
			t.Fatalf("cycle %d remove voter: %v", i, err)
		}
		lead = c.waitStableLeader()
		if err := lead.node.ApplyRemoveMember(id); err != nil {
			t.Fatalf("cycle %d remove member: %v", i, err)
		}
		_ = extra.node.Shutdown()
		for _, n := range c.live() {
			disconnectInmem(tr, n.trans)
		}
		c.waitLeaving(id)
		c.waitAlive("node-a", "node-b", "node-c")
	}

	core := []string{"node-a", "node-b", "node-c"}
	for _, n := range c.live() {
		if !slices.Contains(core, n.id) {
			continue
		}
		got := aliveIDs(n.mem)
		if !slices.Equal(got, core) {
			t.Fatalf("%s alive=%v want %v after join/leave churn", n.id, got, core)
		}
	}
}

// TestChaos_LockHolderDies: a holder that stops renewing loses the lock
// after TTL, including across a leader failover; another holder then wins.
func TestChaos_LockHolderDies(t *testing.T) {
	c := startTriad(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.startLockExpirers(ctx)

	lead := c.mustLeader()
	if _, ok, err := lead.node.ApplyLockAcquire("scheduler", "worker-1", 800*time.Millisecond); err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	c.waitLockHeld("scheduler", "worker-1")

	c.kill(lead)
	c.waitStableLeader()
	c.waitLockGone("scheduler")

	tok, ok, err := c.acquireAfterFailover("scheduler", "worker-2", time.Hour)
	if err != nil || !ok {
		t.Fatalf("reacquire after holder death: ok=%v err=%v", ok, err)
	}
	if tok == 0 {
		t.Fatal("fencing token is zero")
	}
	c.waitLockHeld("scheduler", "worker-2")
}

// TestChaos_LeaseHolderDies: killing a node without revoke expires its
// presence lease and the survivors mark it leaving.
func TestChaos_LeaseHolderDies(t *testing.T) {
	c := startTriad(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.startLeaseExpirers(ctx)

	lead := c.mustLeader()
	victim := c.byID("node-b")
	name := presence.LeaseName(victim.id)
	if _, ok, err := lead.node.ApplyLeaseGrant(name, victim.id, 400*time.Millisecond); err != nil || !ok {
		t.Fatalf("grant presence: ok=%v err=%v", ok, err)
	}

	c.kill(victim)
	c.waitStableLeader()
	c.waitLeaseGone(name)
	c.waitLeaving(victim.id)

	for _, n := range c.live() {
		if hasAlive(n.mem, victim.id) {
			t.Fatalf("%s still lists %s as alive after presence expiry", n.id, victim.id)
		}
	}
}
