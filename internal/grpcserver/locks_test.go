package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/grpcserver"
	"github.com/clusdr/clusdr/internal/locks"
	"github.com/clusdr/clusdr/internal/membership"
)

func startLockServer(t *testing.T, engine *membership.Engine, table *locks.Table) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterLockService(srv.GRPCServer(), table, nil, engine, nopLogger(), nil, 0)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return ln.Addr().String()
}

func TestLockService_RaceOneWins(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := locks.New()
	addr := startLockServer(t, engine, table)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	a, err := c.TryLock(context.Background(), &pb.TryLockRequest{Name: "scheduler", Holder: "node-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Acquired {
		t.Fatal("node-a should win")
	}
	b, err := c.TryLock(context.Background(), &pb.TryLockRequest{Name: "scheduler", Holder: "node-b"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Acquired {
		t.Fatal("node-b must not win")
	}
	if b.Holder != "node-a" || b.FencingToken != a.FencingToken {
		t.Errorf("held by %s token=%d, want node-a token=%d", b.Holder, b.FencingToken, a.FencingToken)
	}

	list, err := c.ListLocks(context.Background(), &pb.ListLocksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Locks) != 1 || list.Locks[0].Name != "scheduler" {
		t.Fatalf("list: %+v", list.Locks)
	}

	if _, err := c.Unlock(context.Background(), &pb.UnlockRequest{
		Name: "scheduler", Holder: "node-b", FencingToken: a.FencingToken,
	}); err == nil {
		t.Fatal("wrong holder should not unlock")
	}
	u, err := c.Unlock(context.Background(), &pb.UnlockRequest{
		Name: "scheduler", Holder: "node-a", FencingToken: a.FencingToken,
	})
	if err != nil || !u.Released {
		t.Fatalf("unlock: %v %+v", err, u)
	}

	b2, err := c.TryLock(context.Background(), &pb.TryLockRequest{Name: "scheduler", Holder: "node-b"})
	if err != nil || !b2.Acquired {
		t.Fatalf("node-b after unlock: %+v %v", b2, err)
	}
	if b2.FencingToken <= a.FencingToken {
		t.Errorf("token did not advance: %d → %d", a.FencingToken, b2.FencingToken)
	}
}

func TestLockService_LockWaitsForUnlock(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := locks.New()
	addr := startLockServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	first, err := c.TryLock(context.Background(), &pb.TryLockRequest{Name: "job", Holder: "node-a"})
	if err != nil || !first.Acquired {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got := make(chan *pb.LockResponse, 1)
	go func() {
		resp, err := c.Lock(ctx, &pb.LockRequest{Name: "job", Holder: "node-b"})
		if err != nil {
			t.Errorf("blocking lock: %v", err)
			close(got)
			return
		}
		got <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := c.Unlock(context.Background(), &pb.UnlockRequest{
		Name: "job", Holder: "node-a", FencingToken: first.FencingToken,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case resp := <-got:
		if resp == nil || !resp.Acquired || resp.Holder != "node-b" {
			t.Fatalf("waiter: %+v", resp)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocking Lock did not complete")
	}
}

func TestLockService_TTLExpireThenOtherWins(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := locks.New()
	addr := startLockServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	a, err := c.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "job", Holder: "node-a", TtlMs: 50,
	})
	if err != nil || !a.Acquired {
		t.Fatalf("acquire: %+v %v", a, err)
	}
	if a.DeadlineUnixMs == 0 {
		t.Fatal("deadline missing")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		due := table.Due(time.Now())
		if len(due) > 0 {
			table.Expire(due[0].Name, due[0].Token)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	b, err := c.TryLock(context.Background(), &pb.TryLockRequest{Name: "job", Holder: "node-b"})
	if err != nil || !b.Acquired {
		t.Fatalf("node-b after expire: %+v %v", b, err)
	}
	if b.FencingToken <= a.FencingToken {
		t.Errorf("token did not advance: %d → %d", a.FencingToken, b.FencingToken)
	}
}

func TestLockService_RenewExtendsDeadline(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := locks.New()
	addr := startLockServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	a, err := c.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "job", Holder: "node-a", TtlMs: 80,
	})
	if err != nil || !a.Acquired {
		t.Fatal(err)
	}
	r, err := c.Renew(context.Background(), &pb.LockServiceRenewRequest{
		Name: "job", Holder: "node-a", FencingToken: a.FencingToken, TtlMs: 3600_000,
	})
	if err != nil || !r.Renewed {
		t.Fatalf("renew: %+v %v", r, err)
	}
	if r.DeadlineUnixMs <= a.DeadlineUnixMs {
		t.Errorf("deadline not extended: %d → %d", a.DeadlineUnixMs, r.DeadlineUnixMs)
	}
	time.Sleep(120 * time.Millisecond)
	if due := table.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lock is due: %+v", due)
	}
}

func TestLockService_LockWaitsForExpire(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := locks.New()
	addr := startLockServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	first, err := c.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "job", Holder: "node-a", TtlMs: 80,
	})
	if err != nil || !first.Acquired {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got := make(chan *pb.LockResponse, 1)
	go func() {
		resp, err := c.Lock(ctx, &pb.LockRequest{Name: "job", Holder: "node-b"})
		if err != nil {
			t.Errorf("blocking lock: %v", err)
			close(got)
			return
		}
		got <- resp
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		due := table.Due(time.Now())
		if len(due) > 0 {
			table.Expire(due[0].Name, due[0].Token)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case resp := <-got:
		if resp == nil || !resp.Acquired || resp.Holder != "node-b" {
			t.Fatalf("waiter: %+v", resp)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocking Lock did not complete after expire")
	}
}

func TestLockService_ObserverRejectsMutations(t *testing.T) {
	engine := membership.New("node-obs", "127.0.0.1:0", nopLogger())
	if _, err := engine.JoinAs("node-obs", "127.0.0.1:0", membership.RoleObserver); err != nil {
		t.Fatal(err)
	}
	table := locks.New()
	addr := startLockServer(t, engine, table)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLockServiceClient(conn)

	_, err = c.TryLock(context.Background(), &pb.TryLockRequest{Name: "scheduler", Holder: "app"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("TryLock: got %v, want FailedPrecondition", err)
	}
	_, err = c.Lock(context.Background(), &pb.LockRequest{Name: "scheduler", Holder: "app"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Lock: got %v, want FailedPrecondition", err)
	}
	_, err = c.Unlock(context.Background(), &pb.UnlockRequest{Name: "scheduler", Holder: "app"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Unlock: got %v, want FailedPrecondition", err)
	}
	_, err = c.Renew(context.Background(), &pb.LockServiceRenewRequest{Name: "scheduler", Holder: "app"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("Renew: got %v, want FailedPrecondition", err)
	}

	list, err := c.ListLocks(context.Background(), &pb.ListLocksRequest{})
	if err != nil {
		t.Fatalf("ListLocks: %v", err)
	}
	if len(list.Locks) != 0 {
		t.Fatalf("list: %+v", list.Locks)
	}
}
