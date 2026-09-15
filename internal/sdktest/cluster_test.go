package clusdr_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/durguto/clusdr/internal/eventbus"
	"github.com/durguto/clusdr/internal/events"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
	"github.com/durguto/clusdr/sdk"
)

func nopLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func startSDKServer(t *testing.T) (addr string, mem *membership.Engine, bus *eventbus.Bus) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mem = membership.New("node-a", ln.Addr().String(), nopLog())
	bus = eventbus.New()
	mem.Emit = bus.Publish
	srv := grpcserver.New(nopLog(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID: "node-a", ClusterID: "c1", Role: "leader",
	})
	grpcserver.RegisterMembershipService(srv.GRPCServer(), mem)
	grpcserver.RegisterWatchService(srv.GRPCServer(), bus, mem, nopLog())
	grpcserver.RegisterEventService(srv.GRPCServer(), bus, mem, nopLog(), nil)
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return ln.Addr().String(), mem, bus
}

func TestSDK_MembersLeaderWatchPublish(t *testing.T) {
	addr, _, _ := startSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	members, err := c.Members(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].ID != "node-a" || !members[0].Leader {
		t.Fatalf("members: %+v", members)
	}

	leader, err := c.Leader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if leader.ID != "node-a" || !leader.Leader {
		t.Fatalf("leader: %+v", leader)
	}

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	ch, err := c.Watch(watchCtx)
	if err != nil {
		t.Fatal(err)
	}

	// Snapshot includes member.join for node-a.
	gotJoin := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !gotJoin {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("watch closed")
			}
			if ev.Type == events.TypeMemberJoin && ev.Source == "node-a" {
				gotJoin = true
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	if !gotJoin {
		t.Fatal("watch snapshot missed member.join")
	}

	if err := c.Publish(ctx, "deployment", []byte(`{"sha":"abc"}`)); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("watch closed")
			}
			if ev.Type == events.CustomType("deployment") {
				return
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("did not see custom.deployment")
}

func TestSDK_WatchSeesLiveJoin(t *testing.T) {
	addr, mem, _ := startSDKServer(t)
	c, err := clusdr.Dial(addr, clusdr.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := c.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Drain snapshot.
	time.Sleep(50 * time.Millisecond)
	for {
		select {
		case <-ch:
		default:
			goto joined
		}
	}
joined:
	if _, err := mem.Join("node-b", "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-ch:
			if ev.Type == events.TypeMemberJoin && ev.Source == "node-b" {
				return
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("live member.join for node-b not seen")
}

func TestSDK_LocalUsesEnvAddr(t *testing.T) {
	addr, _, _ := startSDKServer(t)
	t.Setenv("CLUSDR_GRPC_ADDR", addr)
	t.Setenv("CLUSDR_TLS", "disabled")
	c, err := clusdr.Local()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	members, err := c.Members(ctx)
	if err != nil || len(members) != 1 {
		t.Fatalf("local members: %+v %v", members, err)
	}
}

func TestSDK_RetryUntilReady(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	started := make(chan *grpcserver.Server, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		mem := membership.New("node-a", addr, nopLog())
		srv := grpcserver.New(nopLog(), grpcserver.DefaultConfig())
		grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{NodeID: "node-a"})
		grpcserver.RegisterMembershipService(srv.GRPCServer(), mem)
		started <- srv
		_ = srv.Serve(ln)
	}()

	c, err := clusdr.Dial(addr, clusdr.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	srv := <-started
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := c.Members(ctx); err != nil {
		t.Fatal(err)
	}
}
