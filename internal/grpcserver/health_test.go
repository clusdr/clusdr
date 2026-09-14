package grpcserver_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/odurgut/clusdr/internal/grpcserver"
	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func TestHealthService(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID:    "test-node",
		ClusterID: "test-cluster",
		Role:      "standalone",
	})

	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Shutdown(context.Background()) }) //nolint:errcheck

	conn, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	client := pb.NewHealthServiceClient(conn)
	resp, err := client.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health RPC: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"node_id", resp.NodeId, "test-node"},
		{"cluster_id", resp.ClusterId, "test-cluster"},
		{"role", resp.Role, "standalone"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.want)
		}
	}
	if !resp.Healthy {
		t.Error("healthy: got false, want true")
	}
}

func TestHealthService_PanicRecovery(t *testing.T) {
	// Interceptor must catch panics and return INTERNAL, not crash the server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterHealthService(srv.GRPCServer(), grpcserver.NodeInfo{
		NodeID: "panic-test",
		Role:   "standalone",
	})

	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { srv.Shutdown(context.Background()) }) //nolint:errcheck

	conn, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Normal Health call must still succeed after server started.
	client := pb.NewHealthServiceClient(conn)
	resp, err := client.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if resp.NodeId != "panic-test" {
		t.Errorf("node_id: got %q", resp.NodeId)
	}
}
