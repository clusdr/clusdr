package grpcserver_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/grpcserver"
	"github.com/clusdr/clusdr/internal/membership"
)

type applyVoter struct {
	engine    *membership.Engine
	voters    []string
	nonvoters []string
	removed   []string
	absent    map[string]bool
}

func (v *applyVoter) IsLeader() bool { return true }
func (v *applyVoter) AddVoter(id, _ string) error {
	v.voters = append(v.voters, id)
	if v.absent != nil {
		delete(v.absent, id)
	}
	return nil
}
func (v *applyVoter) AddNonvoter(id, _ string) error {
	v.nonvoters = append(v.nonvoters, id)
	if v.absent != nil {
		delete(v.absent, id)
	}
	return nil
}
func (v *applyVoter) PromoteToVoter(id string) error {
	v.voters = append(v.voters, id)
	return nil
}
func (v *applyVoter) RemoveVoter(id string) error {
	v.removed = append(v.removed, id)
	if v.absent == nil {
		v.absent = map[string]bool{}
	}
	v.absent[id] = true
	return nil
}
func (v *applyVoter) ApplyAddMember(id, addr string) error {
	return v.ApplyAddMemberAs(id, addr, membership.RoleVoter)
}
func (v *applyVoter) ApplyAddMemberAs(id, addr, role string) error {
	_, err := v.engine.JoinAs(id, addr, role)
	return err
}
func (v *applyVoter) ApplyRemoveMember(id string) error {
	v.engine.MarkDead(id)
	return nil
}
func (v *applyVoter) ApplyDropMember(id string) error {
	v.engine.Leave(id)
	return nil
}
func (v *applyVoter) HasServer(id string) bool {
	if v.absent[id] {
		return false
	}
	for _, m := range v.engine.Members() {
		if m.ID == id {
			return true
		}
	}
	for _, x := range v.voters {
		if x == id {
			return true
		}
	}
	for _, x := range v.nonvoters {
		if x == id {
			return true
		}
	}
	return false
}

type followerVoter struct{}

func (followerVoter) IsLeader() bool                   { return false }
func (followerVoter) AddVoter(string, string) error    { return nil }
func (followerVoter) AddNonvoter(string, string) error { return nil }
func (followerVoter) PromoteToVoter(string) error      { return nil }
func (followerVoter) RemoveVoter(string) error         { return nil }
func (followerVoter) ApplyAddMember(string, string) error {
	return nil
}
func (followerVoter) ApplyAddMemberAs(string, string, string) error {
	return nil
}
func (followerVoter) ApplyRemoveMember(string) error { return nil }
func (followerVoter) ApplyDropMember(string) error   { return nil }
func (followerVoter) HasServer(string) bool          { return true }

func startJoinServer(t *testing.T, clusterID string, engine *membership.Engine, voter grpcserver.VoterAdder) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpcserver.New(nopLogger(), grpcserver.DefaultConfig())
	grpcserver.RegisterJoinService(srv.GRPCServer(), clusterID, engine, voter, nopLogger(), nil, nil)
	go srv.Serve(ln)                                         //nolint:errcheck
	t.Cleanup(func() { srv.Shutdown(context.Background()) }) //nolint:errcheck
	return ln.Addr().String()
}

func TestJoinService_FollowerForwardsToLeader(t *testing.T) {
	clusterID := "test-cluster"

	leaderEng := membership.New("node-a", "127.0.0.1:0", nopLogger())
	addrLeader := startJoinServer(t, clusterID, leaderEng, &applyVoter{engine: leaderEng})
	leaderEng2 := membership.New("node-a", addrLeader, nopLogger())
	addrLeader = startJoinServer(t, clusterID, leaderEng2, &applyVoter{engine: leaderEng2})

	followerEng := membership.New("node-b", "127.0.0.1:0", nopLogger())
	if _, err := followerEng.Join("node-a", addrLeader); err != nil {
		t.Fatal(err)
	}
	followerEng.SetLeader("node-a")
	addrFollower := startJoinServer(t, clusterID, followerEng, followerVoter{})

	conn, err := grpc.NewClient(addrFollower, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp, err := pb.NewJoinServiceClient(conn).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-c",
		ClusterId: clusterID,
		Address:   "127.0.0.1:19999",
		RaftAddr:  "127.0.0.1:19998",
	})
	if err != nil {
		t.Fatalf("C→follower Join: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("join rejected: %s", resp.Message)
	}

	gotIDs := map[string]bool{}
	for _, m := range resp.Members {
		gotIDs[m.Id] = true
	}
	if !gotIDs["node-a"] || !gotIDs["node-c"] {
		t.Errorf("leader list missing members: %v", gotIDs)
	}

	for _, m := range followerEng.Members() {
		if m.ID == "node-c" {
			t.Fatal("follower mutated membership locally; FSM/leader should be the only writer")
		}
	}
	found := false
	for _, m := range leaderEng2.Members() {
		if m.ID == "node-c" {
			found = true
		}
	}
	if !found {
		t.Fatal("leader engine missing node-c after ApplyAddMember")
	}
}

func TestJoinService_FollowerNoLeader(t *testing.T) {
	engine := membership.New("node-b", "127.0.0.1:0", nopLogger())
	addr := startJoinServer(t, "c", engine, followerVoter{})

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	_, err = pb.NewJoinServiceClient(conn).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-c",
		ClusterId: "c",
		Address:   "127.0.0.1:1",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("got %v, want FailedPrecondition", err)
	}
}

func TestJoinService_ClusterIDMismatch(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	addrA := startJoinServer(t, "cluster-X", engine, nil)

	conn, err := grpc.NewClient(addrA, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	resp, err := pb.NewJoinServiceClient(conn).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "intruder",
		ClusterId: "cluster-Y",
		Address:   "127.0.0.1:9999",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Accepted {
		t.Error("expected join to be rejected for wrong cluster_id")
	}
}

func TestJoinService_ObserverUsesNonvoter(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	voter := &applyVoter{engine: engine}
	addr := startJoinServer(t, "c", engine, voter)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp, err := pb.NewJoinServiceClient(conn).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-obs",
		ClusterId: "c",
		Address:   "127.0.0.1:3",
		RaftAddr:  "127.0.0.1:4",
		Observer:  true,
	})
	if err != nil {
		t.Fatalf("Join observer: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("rejected: %s", resp.Message)
	}
	if len(voter.nonvoters) != 1 || voter.nonvoters[0] != "node-obs" {
		t.Fatalf("AddNonvoter calls: %v", voter.nonvoters)
	}
	if len(voter.voters) != 0 {
		t.Fatalf("AddVoter should be unused: %v", voter.voters)
	}

	found := false
	for _, m := range engine.Members() {
		if m.ID == "node-obs" {
			found = true
			if m.Role != membership.RoleObserver {
				t.Errorf("role: got %q, want observer", m.Role)
			}
		}
	}
	if !found {
		t.Fatal("observer missing from members")
	}
	for _, m := range resp.Members {
		if m.Id == "node-obs" && m.Role != membership.RoleObserver {
			t.Errorf("proto role: got %q, want observer", m.Role)
		}
	}
}

func TestJoinService_FollowerForwardsObserver(t *testing.T) {
	clusterID := "test-cluster"

	leaderEng := membership.New("node-a", "127.0.0.1:0", nopLogger())
	addrLeader := startJoinServer(t, clusterID, leaderEng, &applyVoter{engine: leaderEng})
	leaderEng2 := membership.New("node-a", addrLeader, nopLogger())
	leadVoter := &applyVoter{engine: leaderEng2}
	addrLeader = startJoinServer(t, clusterID, leaderEng2, leadVoter)

	followerEng := membership.New("node-b", "127.0.0.1:0", nopLogger())
	if _, err := followerEng.Join("node-a", addrLeader); err != nil {
		t.Fatal(err)
	}
	followerEng.SetLeader("node-a")
	addrFollower := startJoinServer(t, clusterID, followerEng, followerVoter{})

	conn, err := grpc.NewClient(addrFollower, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp, err := pb.NewJoinServiceClient(conn).Join(context.Background(), &pb.JoinRequest{
		NodeId:    "node-obs",
		ClusterId: clusterID,
		Address:   "127.0.0.1:19997",
		RaftAddr:  "127.0.0.1:19996",
		Observer:  true,
	})
	if err != nil {
		t.Fatalf("observer→follower Join: %v", err)
	}
	if !resp.Accepted {
		t.Fatalf("join rejected: %s", resp.Message)
	}
	if len(leadVoter.nonvoters) != 1 || leadVoter.nonvoters[0] != "node-obs" {
		t.Fatalf("forward lost observer flag; nonvoters=%v", leadVoter.nonvoters)
	}
	for _, m := range leaderEng2.Members() {
		if m.ID == "node-obs" && m.Role != membership.RoleObserver {
			t.Errorf("leader role: got %q, want observer", m.Role)
		}
	}
}

func TestJoinService_PromoteObserver(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	if _, err := engine.JoinAs("node-obs", "127.0.0.1:3", membership.RoleObserver); err != nil {
		t.Fatal(err)
	}
	voter := &applyVoter{engine: engine}
	addr := startJoinServer(t, "c", engine, voter)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp, err := pb.NewJoinServiceClient(conn).Promote(context.Background(), &pb.PromoteRequest{NodeId: "node-obs"})
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if !resp.Promoted {
		t.Fatalf("not promoted: %s", resp.Message)
	}
	if len(voter.voters) != 1 || voter.voters[0] != "node-obs" {
		t.Fatalf("PromoteToVoter: %v", voter.voters)
	}
	for _, m := range engine.Members() {
		if m.ID == "node-obs" && m.Role != membership.RoleVoter {
			t.Errorf("role after promote: %q", m.Role)
		}
	}

	again, err := pb.NewJoinServiceClient(conn).Promote(context.Background(), &pb.PromoteRequest{NodeId: "node-obs"})
	if err != nil || !again.Promoted {
		t.Fatalf("promote voter again: %v %+v", err, again)
	}

	_, err = pb.NewJoinServiceClient(conn).Promote(context.Background(), &pb.PromoteRequest{NodeId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing: got %v, want NotFound", err)
	}
}

func TestJoinService_LeaveRemovesServer(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	if _, err := engine.Join("node-b", "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	voter := &applyVoter{engine: engine}
	addr := startJoinServer(t, "c", engine, voter)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cli := pb.NewJoinServiceClient(conn)

	resp, err := cli.Leave(context.Background(), &pb.LeaveRequest{NodeId: "node-b"})
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}
	if !resp.Left {
		t.Fatalf("not left: %s", resp.Message)
	}
	if len(voter.removed) != 1 || voter.removed[0] != "node-b" {
		t.Fatalf("RemoveVoter: %v", voter.removed)
	}
	if voter.HasServer("node-b") {
		t.Fatal("HasServer still true after leave")
	}
	for _, m := range engine.Members() {
		if m.ID == "node-b" {
			t.Fatal("left member still in Members()")
		}
	}

	again, err := cli.Leave(context.Background(), &pb.LeaveRequest{NodeId: "node-b"})
	if err != nil || !again.Left {
		t.Fatalf("leave already gone: %v %+v", err, again)
	}

	_, err = cli.Leave(context.Background(), &pb.LeaveRequest{NodeId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing: got %v, want NotFound", err)
	}
}

func TestJoinService_RejoinKeepsRaftServer(t *testing.T) {
	engine := membership.New("node-a", "127.0.0.1:0", nopLogger())
	if _, err := engine.Join("node-b", "127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	engine.MarkDead("node-b")
	voter := &applyVoter{engine: engine}
	addr := startJoinServer(t, "c", engine, voter)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cli := pb.NewJoinServiceClient(conn)

	resp, err := cli.Rejoin(context.Background(), &pb.RejoinRequest{
		NodeId:  "node-b",
		Address: "127.0.0.1:2",
	})
	if err != nil {
		t.Fatalf("Rejoin: %v", err)
	}
	if !resp.Rejoined {
		t.Fatalf("not rejoined: %s", resp.Message)
	}
	if len(voter.voters) != 0 {
		t.Fatalf("Rejoin must not AddVoter: %v", voter.voters)
	}
	found := false
	for _, m := range engine.Members() {
		if m.ID == "node-b" && m.Status == membership.StatusAlive {
			found = true
		}
	}
	if !found {
		t.Fatal("node-b not alive after Rejoin")
	}

	_, err = cli.Rejoin(context.Background(), &pb.RejoinRequest{
		NodeId:  "stranger",
		Address: "127.0.0.1:9",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("unknown rejoin: got %v, want FailedPrecondition", err)
	}
}
