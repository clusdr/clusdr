package consensus_test

import (
	"testing"
	"time"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/leases"
)

func TestLeases_GrantExpireThenOtherWins(t *testing.T) {
	dir := t.TempDir()
	tab := leases.New()
	var expired events.Event
	tab.Emit = func(e events.Event) { expired = e }

	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lease-node", dir, &nopApplier{}, nil, tab, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLeaseGrant("worker-1", "node-a", 80*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("grant: ok=%v err=%v", ok, err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var due []leases.Record
	for time.Now().Before(deadline) {
		due = tab.Due(time.Now())
		if len(due) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(due) == 0 {
		t.Fatal("lease never became due")
	}
	if err := node.ApplyLeaseExpire(due[0].Name, due[0].Token); err != nil {
		t.Fatal(err)
	}
	if _, held := tab.Get("worker-1"); held {
		t.Fatal("expired lease still held")
	}
	if expired.Type != events.TypeLeaseExpired || expired.Source != "worker-1" {
		t.Errorf("event: %+v", expired)
	}

	tok2, ok, err := node.ApplyLeaseGrant("worker-1", "node-b", time.Second)
	if err != nil || !ok {
		t.Fatalf("node-b after expire: ok=%v err=%v", ok, err)
	}
	if tok2 <= tok {
		t.Errorf("token did not advance: %d → %d", tok, tok2)
	}
}

func TestLeases_RenewPreventsDue(t *testing.T) {
	dir := t.TempDir()
	tab := leases.New()
	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lease-node", dir, &nopApplier{}, nil, tab, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLeaseGrant("worker-1", "node-a", 80*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("grant: ok=%v err=%v", ok, err)
	}
	if _, err := node.ApplyLeaseRenew("worker-1", "node-a", tok, time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if due := tab.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lease is due: %+v", due)
	}
	if _, ok := tab.Get("worker-1"); !ok {
		t.Fatal("renewed lease was dropped")
	}
}

func TestLeases_RevokeEmits(t *testing.T) {
	dir := t.TempDir()
	tab := leases.New()
	var last events.Event
	tab.Emit = func(e events.Event) { last = e }

	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lease-node", dir, &nopApplier{}, nil, tab, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLeaseGrant("worker-1", "node-a", time.Second)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if last.Type != events.TypeLeaseGranted {
		t.Errorf("granted event: %+v", last)
	}
	if err := node.ApplyLeaseRevoke("worker-1", "node-a", tok); err != nil {
		t.Fatal(err)
	}
	if last.Type != events.TypeLeaseRevoked || last.Source != "worker-1" {
		t.Errorf("revoked event: %+v", last)
	}
}
