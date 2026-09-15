package grpcserver_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/grpcserver"
	"github.com/durguto/clusdr/internal/membership"
)

func startMembershipServer(t *testing.T, lister grpcserver.MemberLister) pb.MembershipServiceClient {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterMembershipService(srv.GRPCServer(), lister)
	go srv.Serve(ln)                                         //nolint:errcheck
	t.Cleanup(func() { srv.Shutdown(context.Background()) }) //nolint:errcheck

	conn, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewMembershipServiceClient(conn)
}

func newEngine(t *testing.T) *membership.Engine {
	t.Helper()
	return membership.New("node-1", "10.0.0.1:7947", nopLogger())
}

func TestMembershipService_ListMembers(t *testing.T) {
	client := startMembershipServer(t, newEngine(t))

	resp, err := client.ListMembers(context.Background(), &pb.ListMembersRequest{})
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(resp.Members) != 1 {
		t.Fatalf("members count: got %d, want 1", len(resp.Members))
	}

	m := resp.Members[0]
	if m.Id != "node-1" {
		t.Errorf("id: got %q, want %q", m.Id, "node-1")
	}
	if m.Status != "alive" {
		t.Errorf("status: got %q, want alive", m.Status)
	}
	if !m.Leader {
		t.Error("single node should be leader")
	}
	if m.Role != "voter" {
		t.Errorf("role: got %q, want voter", m.Role)
	}
}

func TestMembershipService_GetLeader(t *testing.T) {
	client := startMembershipServer(t, newEngine(t))

	resp, err := client.GetLeader(context.Background(), &pb.GetLeaderRequest{})
	if err != nil {
		t.Fatalf("GetLeader: %v", err)
	}
	if resp.LeaderId != "node-1" {
		t.Errorf("leader_id: got %q, want %q", resp.LeaderId, "node-1")
	}
}

func TestMembershipService_ListAfterJoin(t *testing.T) {
	e := newEngine(t)
	if _, err := e.Join("node-2", "10.0.0.2:7947"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	client := startMembershipServer(t, e)

	resp, err := client.ListMembers(context.Background(), &pb.ListMembersRequest{})
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(resp.Members) != 2 {
		t.Fatalf("members count: got %d, want 2", len(resp.Members))
	}
}

func TestMembershipService_ObserverRole(t *testing.T) {
	e := newEngine(t)
	if _, err := e.JoinAs("node-obs", "10.0.0.9:7947", membership.RoleObserver); err != nil {
		t.Fatalf("JoinAs: %v", err)
	}
	client := startMembershipServer(t, e)

	resp, err := client.ListMembers(context.Background(), &pb.ListMembersRequest{})
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	found := false
	for _, m := range resp.Members {
		if m.Id == "node-obs" {
			found = true
			if m.Role != "observer" {
				t.Errorf("role: got %q, want observer", m.Role)
			}
		}
	}
	if !found {
		t.Fatal("observer missing from ListMembers")
	}
}
