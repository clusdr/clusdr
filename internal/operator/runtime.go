package operator

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/mtls"
)

// Runtime talks to daemons over the Runtime API (bootstrap TLS). Not Raft.
type Runtime struct{}

// Join calls ControlService.RequestJoin on the joiner at localAddr (hostIP:7947).
func (Runtime) Join(ctx context.Context, localAddr, seedAddr, token string, observer bool) error {
	conn, err := mtls.Dial(localAddr, mtls.BootstrapTLS())
	if err != nil {
		return fmt.Errorf("dial joiner %s: %w", localAddr, err)
	}
	defer conn.Close()
	resp, err := pb.NewControlServiceClient(conn).RequestJoin(ctx, &pb.RequestJoinRequest{
		Addr:     seedAddr,
		Token:    token,
		Observer: observer,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already") {
			return nil
		}
		return err
	}
	if resp.GetJoined() {
		return nil
	}
	msg := strings.ToLower(resp.GetMessage())
	if strings.Contains(msg, "already") {
		return nil
	}
	return fmt.Errorf("join rejected: %s", resp.GetMessage())
}

// Snapshot reads ListMembers, GetLeader, and Health from the seed Runtime.
func (Runtime) Snapshot(ctx context.Context, seedAddr string) (Status, error) {
	conn, err := mtls.Dial(seedAddr, mtls.BootstrapTLS())
	if err != nil {
		return Status{}, fmt.Errorf("dial seed %s: %w", seedAddr, err)
	}
	defer conn.Close()
	memc := pb.NewMembershipServiceClient(conn)
	list, err := memc.ListMembers(ctx, &pb.ListMembersRequest{})
	if err != nil {
		return Status{}, err
	}
	st := Status{Phase: "Ready"}
	for _, m := range list.GetMembers() {
		role := m.GetRole()
		if m.GetLeader() {
			role = "leader"
		} else if role == "" {
			role = "voter"
		}
		st.Members = append(st.Members, MemberStatus{
			ID:      m.GetId(),
			Address: m.GetAddress(),
			Status:  m.GetStatus(),
			Role:    role,
		})
	}
	lead, err := memc.GetLeader(ctx, &pb.GetLeaderRequest{})
	if err == nil {
		st.Leader = lead.GetLeaderId()
	}
	h, err := pb.NewHealthServiceClient(conn).Health(ctx, &pb.HealthRequest{})
	if err == nil {
		st.ClusterID = h.GetClusterId()
	}
	return st, nil
}

// Leave calls ControlService.RequestLeave on the seed Runtime (not on a missing pod).
func (Runtime) Leave(ctx context.Context, seedAddr, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("empty leave id")
	}
	conn, err := mtls.Dial(seedAddr, mtls.BootstrapTLS())
	if err != nil {
		return fmt.Errorf("dial seed %s: %w", seedAddr, err)
	}
	defer conn.Close()
	resp, err := pb.NewControlServiceClient(conn).RequestLeave(ctx, &pb.RequestLeaveRequest{NodeId: nodeID})
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not found") || strings.Contains(msg, "already") {
			return nil
		}
		return err
	}
	if resp.GetLeft() {
		return nil
	}
	msg := strings.ToLower(resp.GetMessage())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "already") {
		return nil
	}
	return fmt.Errorf("leave rejected: %s", resp.GetMessage())
}
