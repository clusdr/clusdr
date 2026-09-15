package consensus_test

import (
	"context"
	"testing"
	"time"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/internal/presence"
)

func TestPresenceLeaseExpire_MemberLeft(t *testing.T) {
	dir := t.TempDir()
	mem := membership.New("node-a", "127.0.0.1:1", nopLog())
	if _, err := mem.Join("node-b", "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	bus := eventbus.New()
	mem.Emit = bus.Publish
	tab := leases.New()
	tab.Emit = bus.Publish
	sub := bus.Subscribe(32)
	t.Cleanup(sub.Unsubscribe)

	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "node-a", dir, mem, nil, tab, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go presence.WatchExpired(ctx, sub, "node-a", node.IsLeader, func(id string) {
		if err := node.ApplyRemoveMember(id); err != nil {
			t.Errorf("remove_member %s: %v", id, err)
		}
	})

	name := presence.LeaseName("node-b")
	tok, ok, err := node.ApplyLeaseGrant(name, "node-b", 80*time.Millisecond)
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
		t.Fatal("presence lease never became due")
	}
	if err := node.ApplyLeaseExpire(name, tok); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, m := range mem.Members() {
			if m.ID == "node-b" && m.Status == membership.StatusLeaving {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("node-b still alive after presence lease expired")
}

func TestPresenceLeaseExpire_UserLeaseDoesNotLeave(t *testing.T) {
	dir := t.TempDir()
	mem := membership.New("node-a", "127.0.0.1:1", nopLog())
	if _, err := mem.Join("node-b", "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	tab := leases.New()
	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "node-a", dir, mem, nil, tab, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLeaseGrant("worker-1", "node-b", 80*time.Millisecond)
	if err != nil || !ok {
		t.Fatal(err)
	}
	presence.OnExpired(events.Event{
		Type:   events.TypeLeaseExpired,
		Source: "worker-1",
	}, "node-a", true, func(id string) {
		t.Errorf("remove called for user lease owner %s", id)
		_ = node.ApplyRemoveMember(id)
	})
	if err := node.ApplyLeaseExpire("worker-1", tok); err != nil {
		t.Fatal(err)
	}
	for _, m := range mem.Members() {
		if m.ID == "node-b" && m.Status != membership.StatusAlive {
			t.Fatalf("node-b status %s after user lease expire", m.Status)
		}
	}
}
