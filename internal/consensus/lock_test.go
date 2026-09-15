package consensus_test

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/durguto/clusdr/internal/consensus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/locks"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func waitLeader(t *testing.T, node *consensus.Node) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if node.IsLeader() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("not leader")
}

func TestLocks_RaceOneWins(t *testing.T) {
	dir := t.TempDir()
	tab := locks.New()
	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lock-node", dir, &nopApplier{}, tab, nil, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	var wg sync.WaitGroup
	type outcome struct {
		holder   string
		acquired bool
		token    uint64
		err      error
	}
	ch := make(chan outcome, 2)
	for _, h := range []string{"node-a", "node-b"} {
		wg.Add(1)
		go func(holder string) {
			defer wg.Done()
			tok, ok, err := node.ApplyLockAcquire("scheduler", holder, 0)
			ch <- outcome{holder: holder, acquired: ok, token: tok, err: err}
		}(h)
	}
	wg.Wait()
	close(ch)

	var winners int
	var winner outcome
	for o := range ch {
		if o.err != nil {
			t.Fatalf("%s: %v", o.holder, o.err)
		}
		if o.acquired {
			winners++
			winner = o
		}
	}
	if winners != 1 {
		t.Fatalf("winners: got %d, want 1", winners)
	}
	rec, ok := tab.Get("scheduler")
	if !ok {
		t.Fatal("lock missing from table")
	}
	if rec.Holder != winner.holder || rec.Token != winner.token {
		t.Errorf("table holder=%s token=%d; winner=%s token=%d",
			rec.Holder, rec.Token, winner.holder, winner.token)
	}

	if err := node.ApplyLockRelease("scheduler", winner.holder, winner.token); err != nil {
		t.Fatal(err)
	}
	loser := "node-a"
	if winner.holder == "node-a" {
		loser = "node-b"
	}
	tok, ok, err := node.ApplyLockAcquire("scheduler", loser, 0)
	if err != nil || !ok {
		t.Fatalf("loser retry: ok=%v err=%v", ok, err)
	}
	if tok <= winner.token {
		t.Errorf("fencing token did not advance: old=%d new=%d", winner.token, tok)
	}
}

func TestLocks_ExpireThenOtherWins(t *testing.T) {
	dir := t.TempDir()
	tab := locks.New()
	var expired events.Event
	tab.Emit = func(e events.Event) { expired = e }

	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lock-node", dir, &nopApplier{}, tab, nil, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLockAcquire("job", "node-a", 80*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var due []locks.Record
	for time.Now().Before(deadline) {
		due = tab.Due(time.Now())
		if len(due) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(due) == 0 {
		t.Fatal("lock never became due")
	}
	if err := node.ApplyLockExpire(due[0].Name, due[0].Token); err != nil {
		t.Fatal(err)
	}
	if _, held := tab.Get("job"); held {
		t.Fatal("expired lock still held")
	}
	if expired.Type != events.TypeLockExpired || expired.Source != "job" {
		t.Errorf("event: %+v", expired)
	}

	tok2, ok, err := node.ApplyLockAcquire("job", "node-b", time.Second)
	if err != nil || !ok {
		t.Fatalf("node-b after expire: ok=%v err=%v", ok, err)
	}
	if tok2 <= tok {
		t.Errorf("token did not advance: %d → %d", tok, tok2)
	}
}

func TestLocks_RenewPreventsExpire(t *testing.T) {
	dir := t.TempDir()
	tab := locks.New()
	addr := freeAddr(t)
	node, err := consensus.New(consensus.Config{
		Addr:      addr,
		Bootstrap: true,
	}, "lock-node", dir, &nopApplier{}, tab, nil, nopLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = node.Shutdown() })
	waitLeader(t, node)

	tok, ok, err := node.ApplyLockAcquire("job", "node-a", 80*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	if _, err := node.ApplyLockRenew("job", "node-a", tok, time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if due := tab.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lock is due: %+v", due)
	}
	if _, ok := tab.Get("job"); !ok {
		t.Fatal("renewed lock was dropped")
	}
}
