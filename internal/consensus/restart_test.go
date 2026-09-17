package consensus_test

import (
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/consensus"
	"github.com/clusdr/clusdr/internal/leases"
	"github.com/clusdr/clusdr/internal/locks"
	"github.com/clusdr/clusdr/internal/membership"
)

func tcpFast(addr string, bootstrap bool) consensus.Config {
	return consensus.Config{
		Addr:               addr,
		Bootstrap:          bootstrap,
		HeartbeatTimeout:   150 * time.Millisecond,
		ElectionTimeout:    150 * time.Millisecond,
		LeaderLeaseTimeout: 75 * time.Millisecond,
	}
}

// TestNode_RestartSameDataDir: bounce a follower with the same raft dir; it
// catches up without AddVoter.
func TestNode_RestartSameDataDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	addrA := freeAddr(t)
	addrB := freeAddr(t)

	memA := membership.New("node-a", addrA, nopLog())
	locksA := locks.New()
	leasesA := leases.New()
	a, err := consensus.New(tcpFast(addrA, true), "node-a", dirA, memA, locksA, leasesA, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Shutdown() })
	waitLeader(t, a)

	memB := membership.New("node-b", addrB, nopLog())
	locksB := locks.New()
	leasesB := leases.New()
	b, err := consensus.New(tcpFast(addrB, false), "node-b", dirB, memB, locksB, leasesB, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddVoter("node-b", addrB); err != nil {
		t.Fatalf("add voter: %v", err)
	}
	if err := a.ApplyAddMember("node-a", addrA); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyAddMember("node-b", addrB); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := a.ApplyLockAcquire("job", "holder-a", time.Hour); err != nil || !ok {
		t.Fatalf("lock: ok=%v err=%v", ok, err)
	}

	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}

	memB2 := membership.New("node-b", addrB, nopLog())
	locksB2 := locks.New()
	leasesB2 := leases.New()
	b2, err := consensus.New(tcpFast(addrB, false), "node-b", dirB, memB2, locksB2, leasesB2, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b2.Shutdown() })

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if rec, ok := locksB2.Get("job"); ok && rec.Holder == "holder-a" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if rec, ok := locksB2.Get("job"); !ok || rec.Holder != "holder-a" {
		t.Fatal("restarted node did not catch up lock table")
	}

	var lead *consensus.Node
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		switch {
		case a.IsLeader():
			lead = a
		case b2.IsLeader():
			lead = b2
		default:
			time.Sleep(20 * time.Millisecond)
			continue
		}
		break
	}
	if lead == nil {
		t.Fatal("no leader after restart")
	}
	if !lead.HasServer("node-b") {
		t.Fatal("restarted node is not in Raft")
	}
	if err := lead.ApplyAddMember("node-b", addrB); err != nil {
		t.Fatalf("rejoin: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hasAlive(memA, "node-b") && hasAlive(memB2, "node-b") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("node-b not alive after restart rejoin")
}

func TestNode_LeaveDropsServer(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	addrA := freeAddr(t)
	addrB := freeAddr(t)

	memA := membership.New("node-a", addrA, nopLog())
	a, err := consensus.New(tcpFast(addrA, true), "node-a", dirA, memA, nil, nil, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Shutdown() })
	waitLeader(t, a)

	b, err := consensus.New(tcpFast(addrB, false), "node-b", dirB, membership.New("node-b", addrB, nopLog()), nil, nil, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Shutdown() })
	if err := a.AddVoter("node-b", addrB); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyAddMember("node-b", addrB); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyDropMember("node-b"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveVoter("node-b"); err != nil {
		t.Fatal(err)
	}
	if a.HasServer("node-b") {
		t.Fatal("leave left node-b in Raft")
	}
}
