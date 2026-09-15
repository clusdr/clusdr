package heartbeat_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/heartbeat"
	"github.com/durguto/clusdr/internal/membership"
)

func nopLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

// startFakePeer starts a minimal gRPC server that responds to Ping.
// Returns its address and a cancel func to shut it down.
func startFakePeer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterHeartbeatServiceServer(srv, &fakePing{})
	go srv.Serve(ln) //nolint:errcheck
	return ln.Addr().String(), srv.Stop
}

type fakePing struct {
	pb.UnimplementedHeartbeatServiceServer
}

func (f *fakePing) Ping(_ context.Context, _ *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{NodeId: "fake", Alive: true}, nil
}

func TestMonitor_MarksLeaving_WhenPeerUnreachable(t *testing.T) {
	// Peer on an address that won't respond.
	e := membership.New("node-a", "127.0.0.1:9991", nopLog())
	if _, err := e.Join("node-b", "127.0.0.1:19999"); err != nil { // nothing listening there
		t.Fatalf("Join: %v", err)
	}

	cfg := heartbeat.Config{
		Interval:  50 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		MaxMisses: 2,
	}
	m := heartbeat.New(cfg, e, nopLog())
	m.Start()
	t.Cleanup(m.Stop)

	// Wait long enough for MaxMisses pings to fire.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		for _, mem := range e.Members() {
			if mem.ID == "node-b" && mem.Status == membership.StatusLeaving {
				return // success
			}
		}
	}
	t.Error("node-b was not marked leaving after missed heartbeats")
}

func TestMonitor_KeepsAlive_WhenPeerResponds(t *testing.T) {
	addr, stop := startFakePeer(t)
	defer stop()

	e := membership.New("node-a", "127.0.0.1:9992", nopLog())
	if _, err := e.Join("node-b", addr); err != nil {
		t.Fatalf("Join: %v", err)
	}

	cfg := heartbeat.Config{
		Interval:  50 * time.Millisecond,
		Timeout:   500 * time.Millisecond,
		MaxMisses: 2,
	}
	m := heartbeat.New(cfg, e, nopLog())
	m.Start()
	t.Cleanup(m.Stop)

	// Run for several ticks — node-b must stay alive.
	time.Sleep(400 * time.Millisecond)
	for _, mem := range e.Members() {
		if mem.ID == "node-b" && mem.Status != membership.StatusAlive {
			t.Errorf("node-b status: got %q, want alive", mem.Status)
		}
	}
}

func TestMonitor_IgnoresSelf(t *testing.T) {
	// Self address is unreachable but must never be marked leaving.
	e := membership.New("node-a", "127.0.0.1:19998", nopLog())

	cfg := heartbeat.Config{
		Interval:  50 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		MaxMisses: 1,
	}
	m := heartbeat.New(cfg, e, nopLog())
	m.Start()
	t.Cleanup(m.Stop)

	time.Sleep(300 * time.Millisecond)
	for _, mem := range e.Members() {
		if mem.ID == "node-a" && mem.Status != membership.StatusAlive {
			t.Errorf("self was marked as not alive: %q", mem.Status)
		}
	}
}

func TestMonitor_ReportDeadInsteadOfLeave(t *testing.T) {
	e := membership.New("node-a", "127.0.0.1:9991", nopLog())
	if _, err := e.Join("node-b", "127.0.0.1:19999"); err != nil {
		t.Fatalf("Join: %v", err)
	}

	got := make(chan string, 1)
	cfg := heartbeat.Config{
		Interval:  50 * time.Millisecond,
		Timeout:   50 * time.Millisecond,
		MaxMisses: 2,
	}
	m := heartbeat.New(cfg, e, nopLog())
	m.SetReportDead(func(id string) {
		select {
		case got <- id:
		default:
		}
	})
	m.Start()
	t.Cleanup(m.Stop)

	select {
	case id := <-got:
		if id != "node-b" {
			t.Fatalf("reportDead: got %q", id)
		}
		for _, mem := range e.Members() {
			if mem.ID == "node-b" && mem.Status != membership.StatusAlive {
				t.Fatalf("engine was mutated locally; status=%s", mem.Status)
			}
		}
	case <-time.After(2 * time.Second):
		t.Error("reportDead was not called")
	}
}

func TestDial_FakePeer(t *testing.T) {
	addr, stop := startFakePeer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer conn.Close()

	resp, err := pb.NewHeartbeatServiceClient(conn).Ping(
		context.Background(), &pb.PingRequest{SenderId: "test"})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if !resp.Alive {
		t.Error("expected alive=true")
	}
}
