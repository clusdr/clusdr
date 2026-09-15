package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/membership"
	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

func startLeaseServer(t *testing.T, engine *membership.Engine, table *leases.Table) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterLeaseService(srv.GRPCServer(), table, nil, engine, nopLogger(), nil, 0)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return ln.Addr().String()
}

func TestLeaseService_GrantListRevoke(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := leases.New()
	addr := startLeaseServer(t, engine, table)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLeaseServiceClient(conn)

	a, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{Name: "worker-1", Owner: "node-a"})
	if err != nil || !a.Granted {
		t.Fatalf("grant: %+v %v", a, err)
	}
	b, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{Name: "worker-1", Owner: "node-b"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Granted {
		t.Fatal("node-b must not steal the lease")
	}

	list, err := c.ListLeases(context.Background(), &pb.ListLeasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Leases) != 1 || list.Leases[0].Name != "worker-1" || list.Leases[0].Owner != "node-a" {
		t.Fatalf("list: %+v", list.Leases)
	}

	if _, err := c.Revoke(context.Background(), &pb.RevokeLeaseRequest{
		Name: "worker-1", Owner: "node-b", FencingToken: a.FencingToken,
	}); err == nil {
		t.Fatal("wrong owner should not revoke")
	}
	u, err := c.Revoke(context.Background(), &pb.RevokeLeaseRequest{
		Name: "worker-1", Owner: "node-a", FencingToken: a.FencingToken,
	})
	if err != nil || !u.Revoked {
		t.Fatalf("revoke: %v %+v", err, u)
	}

	b2, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{Name: "worker-1", Owner: "node-b"})
	if err != nil || !b2.Granted {
		t.Fatalf("node-b after revoke: %+v %v", b2, err)
	}
	if b2.FencingToken <= a.FencingToken {
		t.Errorf("token did not advance: %d → %d", a.FencingToken, b2.FencingToken)
	}
}

func TestLeaseService_ExpireThenOtherWins(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := leases.New()
	var expired events.Event
	table.Emit = func(e events.Event) { expired = e }
	addr := startLeaseServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLeaseServiceClient(conn)

	a, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{
		Name: "worker-1", Owner: "node-a", TtlMs: 50,
	})
	if err != nil || !a.Granted {
		t.Fatalf("grant: %+v %v", a, err)
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

	list, err := c.ListLeases(context.Background(), &pb.ListLeasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Leases) != 0 {
		t.Fatalf("list after expire: %+v", list.Leases)
	}
	if expired.Type != events.TypeLeaseExpired {
		t.Errorf("event: %+v", expired)
	}

	b, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{Name: "worker-1", Owner: "node-b"})
	if err != nil || !b.Granted {
		t.Fatalf("node-b after expire: %+v %v", b, err)
	}
}

func TestLeaseService_RenewExtendsDeadline(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	table := leases.New()
	addr := startLeaseServer(t, engine, table)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pb.NewLeaseServiceClient(conn)

	a, err := c.Grant(context.Background(), &pb.GrantLeaseRequest{
		Name: "worker-1", Owner: "node-a", TtlMs: 80,
	})
	if err != nil || !a.Granted {
		t.Fatal(err)
	}
	r, err := c.Renew(context.Background(), &pb.RenewLeaseRequest{
		Name: "worker-1", Owner: "node-a", FencingToken: a.FencingToken, TtlMs: 3600_000,
	})
	if err != nil || !r.Renewed {
		t.Fatalf("renew: %+v %v", r, err)
	}
	if r.DeadlineUnixMs <= a.DeadlineUnixMs {
		t.Errorf("deadline not extended: %d → %d", a.DeadlineUnixMs, r.DeadlineUnixMs)
	}
	time.Sleep(120 * time.Millisecond)
	if due := table.Due(time.Now()); len(due) != 0 {
		t.Fatalf("renewed lease is due: %+v", due)
	}
}
