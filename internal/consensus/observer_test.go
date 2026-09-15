package consensus_test

import (
	"testing"
	"time"

	raftlib "github.com/hashicorp/raft"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/membership"
)

func addObserver(t *testing.T, c *chaosCluster, id string) *chaosNode {
	t.Helper()
	_, tr := raftlib.NewInmemTransport("")
	for _, n := range c.nodes {
		connectInmem(tr, n.trans)
	}
	obs := startChaosNode(t, id, false, tr)
	if err := c.withAnyLeader(func(lead *consensus.Node) error {
		return lead.AddNonvoter(obs.id, string(obs.trans.LocalAddr()))
	}); err != nil {
		t.Fatalf("add nonvoter %s: %v", id, err)
	}
	c.nodes = append(c.nodes, obs)
	if err := c.withAnyLeader(func(lead *consensus.Node) error {
		return lead.ApplyAddMemberAs(obs.id, string(obs.trans.LocalAddr()), membership.RoleObserver)
	}); err != nil {
		t.Fatalf("apply observer %s: %v", id, err)
	}
	return obs
}

func observerRoleOn(t *testing.T, n *chaosNode, id string) string {
	t.Helper()
	for _, m := range n.mem.Members() {
		if m.ID == id {
			return m.Role
		}
	}
	t.Fatalf("%s: member %s not found", n.id, id)
	return ""
}

func TestObserver_ListedAndCatchesUp(t *testing.T) {
	c := startTriad(t)
	addObserver(t, c, "node-obs")
	c.waitAlive("node-a", "node-b", "node-c", "node-obs")
	c.waitRole("node-obs", membership.RoleObserver)

	for _, n := range c.live() {
		if got := observerRoleOn(t, n, "node-obs"); got != membership.RoleObserver {
			t.Fatalf("%s: observer role %q, want observer", n.id, got)
		}
		if got := observerRoleOn(t, n, "node-a"); got != membership.RoleVoter {
			t.Fatalf("%s: voter role %q, want voter", n.id, got)
		}
	}

	lead := c.mustLeader()
	if _, ok, err := lead.node.ApplyLockAcquire("obs-lock", lead.id, time.Hour); err != nil || !ok {
		t.Fatalf("lock acquire: ok=%v err=%v", ok, err)
	}
	c.waitLockHeld("obs-lock", lead.id)
}

func TestObserver_DeathKeepsLeader(t *testing.T) {
	c := startTriad(t)
	obs := addObserver(t, c, "node-obs")
	c.waitAlive("node-a", "node-b", "node-c", "node-obs")
	c.waitRole("node-obs", membership.RoleObserver)

	lead := c.mustLeader()
	leadID := lead.id
	c.kill(obs)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !c.byID(leadID).node.IsLeader() {
			t.Fatalf("leader changed after observer death; want %s still leader", leadID)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestObserver_VoterDeathElectsAmongVoters(t *testing.T) {
	c := startTriad(t)
	obs := addObserver(t, c, "node-obs")
	c.waitAlive("node-a", "node-b", "node-c", "node-obs")
	c.waitRole("node-obs", membership.RoleObserver)

	lead := c.mustLeader()
	old := lead.id
	c.kill(lead)

	var remaining []string
	for _, id := range []string{"node-a", "node-b", "node-c"} {
		if id != old {
			remaining = append(remaining, id)
		}
	}
	newLead := c.waitLeaderAmong(remaining...)
	if newLead.id == obs.id {
		t.Fatal("observer became leader")
	}
	if obs.node.IsLeader() {
		t.Fatal("observer IsLeader after voter death")
	}
}

func TestObserver_PromoteJoinsQuorum(t *testing.T) {
	c := startTriad(t)
	obs := addObserver(t, c, "node-obs")
	c.waitAlive("node-a", "node-b", "node-c", "node-obs")
	c.waitRole("node-obs", membership.RoleObserver)

	if err := c.withAnyLeader(func(lead *consensus.Node) error {
		return lead.PromoteToVoter(obs.id)
	}); err != nil {
		t.Fatalf("PromoteToVoter: %v", err)
	}
	if err := c.withAnyLeader(func(lead *consensus.Node) error {
		return lead.ApplyAddMemberAs(obs.id, string(obs.trans.LocalAddr()), membership.RoleVoter)
	}); err != nil {
		t.Fatalf("apply voter: %v", err)
	}
	c.waitRole("node-obs", membership.RoleVoter)
	c.waitStableLeader()

	old := c.mustLeader().id
	c.kill(c.byID(old))
	var remaining []string
	for _, id := range []string{"node-a", "node-b", "node-c", "node-obs"} {
		if id != old {
			remaining = append(remaining, id)
		}
	}
	newLead := c.waitLeaderAmong(remaining...)
	if newLead == nil {
		t.Fatal("no leader after promote + voter death")
	}
}
