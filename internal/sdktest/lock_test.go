package clusdr_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/grpcserver"
	"github.com/clusdr/clusdr/internal/locks"
	"github.com/clusdr/clusdr/internal/membership"
	"github.com/clusdr/clusdr/sdk"
)

func startLockSDKServer(t *testing.T) (addr string, table *locks.Table) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mem := membership.New("node-a", ln.Addr().String(), nopLog())
	table = locks.New()
	srv := grpcserver.New(nopLog(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID: "node-a", ClusterID: "c1", Role: "leader",
	})
	grpcserver.RegisterLockService(srv.GRPCServer(), table, nil, mem, nopLog(), nil, 0)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return ln.Addr().String(), table
}

func TestSDK_LockAcquireUnlock(t *testing.T) {
	addr, table := startLockSDKServer(t)
	a, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-b"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lk, err := a.Lock(ctx, "scheduler", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lk.Token == 0 || lk.Holder != "worker-a" {
		t.Fatalf("lock: %+v", lk)
	}
	if lk.Deadline().IsZero() {
		t.Fatal("deadline missing")
	}

	other, ok, err := b.TryLock(ctx, "scheduler", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("worker-b must not acquire")
	}
	if other == nil || other.Holder != "worker-a" || other.Token != lk.Token {
		t.Fatalf("held snapshot: %+v", other)
	}

	if err := a.Unlock(ctx, "scheduler"); err != nil {
		t.Fatal(err)
	}
	if _, held := table.Get("scheduler"); held {
		t.Fatal("table still holds scheduler")
	}

	won, ok, err := b.TryLock(ctx, "scheduler", time.Second)
	if err != nil || !ok {
		t.Fatalf("worker-b after unlock: %+v %v", won, err)
	}
	if won.Token <= lk.Token {
		t.Errorf("token did not advance: %d → %d", lk.Token, won.Token)
	}
	if err := b.Unlock(ctx, "scheduler"); err != nil {
		t.Fatal(err)
	}
}

func TestSDK_LockWaitsForUnlock(t *testing.T) {
	addr, _ := startLockSDKServer(t)
	a, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-b"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first, ok, err := a.TryLock(ctx, "job", time.Second)
	if err != nil || !ok || first == nil {
		t.Fatalf("trylock: %+v %v %v", first, ok, err)
	}

	got := make(chan *clusdr.Lock, 1)
	go func() {
		lk, err := b.Lock(ctx, "job", time.Second)
		if err != nil {
			t.Errorf("blocking lock: %v", err)
			close(got)
			return
		}
		got <- lk
	}()

	time.Sleep(50 * time.Millisecond)
	if err := a.Unlock(ctx, "job"); err != nil {
		t.Fatal(err)
	}
	select {
	case lk := <-got:
		if lk == nil || lk.Holder != "worker-b" {
			t.Fatalf("waiter: %+v", lk)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocking Lock did not complete")
	}
}

func TestSDK_UnlockRequiresAcquire(t *testing.T) {
	addr, _ := startLockSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Unlock(ctx, "missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSDK_CloseReleasesLock(t *testing.T) {
	addr, table := startLockSDKServer(t)
	a, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := a.Lock(ctx, "job", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, held := table.Get("job"); held {
		t.Fatal("Close should release job")
	}
}

func TestSDK_LockRenewKeepsGrant(t *testing.T) {
	addr, table := startLockSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lk, err := c.Lock(ctx, "job", 150*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	first := lk.Deadline()
	time.Sleep(200 * time.Millisecond)
	if due := table.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lock is due: %+v", due)
	}
	rec, ok := table.Get("job")
	if !ok {
		t.Fatal("lock gone")
	}
	if !rec.Deadline.After(first) {
		t.Errorf("deadline not extended: %v → %v", first, rec.Deadline)
	}
}
