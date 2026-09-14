package clusdr_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/odurgut/clusdr/internal/grpcserver"
	"github.com/odurgut/clusdr/internal/leases"
	"github.com/odurgut/clusdr/internal/membership"
	"github.com/odurgut/clusdr/sdk"
)

func startLeaseSDKServer(t *testing.T) (addr string, table *leases.Table) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mem := membership.New("node-a", ln.Addr().String(), nopLog())
	table = leases.New()
	srv := grpcserver.New(nopLog(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID: "node-a", ClusterID: "c1", Role: "leader",
	})
	grpcserver.RegisterLeaseService(srv.GRPCServer(), table, nil, mem, nopLog(), nil, 0)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return ln.Addr().String(), table
}

func TestSDK_LeaseGrantRevoke(t *testing.T) {
	addr, table := startLeaseSDKServer(t)
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

	lk, err := a.Lease(ctx, "worker-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lk.Token == 0 || lk.Owner != "worker-a" {
		t.Fatalf("lease: %+v", lk)
	}
	if lk.Deadline().IsZero() {
		t.Fatal("deadline missing")
	}

	if _, err := b.Lease(ctx, "worker-1", time.Second); err == nil {
		t.Fatal("worker-b must not steal the lease")
	}

	if err := a.Revoke(ctx, "worker-1"); err != nil {
		t.Fatal(err)
	}
	if _, held := table.Get("worker-1"); held {
		t.Fatal("table still holds worker-1")
	}

	won, err := b.Lease(ctx, "worker-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if won.Token <= lk.Token {
		t.Errorf("token did not advance: %d → %d", lk.Token, won.Token)
	}
	if err := b.Revoke(ctx, "worker-1"); err != nil {
		t.Fatal(err)
	}
}

func TestSDK_LeaseRenewKeepsGrant(t *testing.T) {
	addr, table := startLeaseSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lk, err := c.Lease(ctx, "worker-1", 150*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	first := lk.Deadline()
	if err := c.Renew(ctx, "worker-1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if due := table.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lease is due: %+v", due)
	}
	rec, ok := table.Get("worker-1")
	if !ok {
		t.Fatal("lease gone")
	}
	if !rec.Deadline.After(first) {
		t.Errorf("deadline not extended: %v → %v", first, rec.Deadline)
	}
}

func TestSDK_LeaseCancelStopsRenew(t *testing.T) {
	addr, table := startLeaseSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	hold, stop := context.WithCancel(context.Background())
	lk, err := c.Lease(hold, "worker-1", 80*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if due := table.Due(time.Now()); len(due) > 0 {
			table.Expire(due[0].Name, due[0].Token)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, held := table.Get("worker-1"); held {
		t.Fatal("cancelled lease should expire")
	}
	if lk.Token == 0 {
		t.Fatal("token missing")
	}
}

func TestSDK_CloseRevokesLease(t *testing.T) {
	addr, table := startLeaseSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure(), clusdr.WithHolder("worker-a"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.Lease(ctx, "worker-1", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, held := table.Get("worker-1"); held {
		t.Fatal("Close should revoke worker-1")
	}
}

func TestSDK_RevokeRequiresGrant(t *testing.T) {
	addr, _ := startLeaseSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Revoke(ctx, "missing"); err == nil {
		t.Fatal("expected error")
	}
	if err := c.Renew(ctx, "missing"); err == nil {
		t.Fatal("expected error")
	}
}
