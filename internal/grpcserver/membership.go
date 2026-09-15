package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/membership"
)

// MemberLister is the read interface the membership service consumes.
// Implemented by *membership.Engine; may be replaced by a test double.
type MemberLister interface {
	Members() []membership.Member
	Leader() (membership.Member, bool)
}

// RegisterMembershipService registers the MembershipService on srv.
func RegisterMembershipService(srv *grpc.Server, lister MemberLister) {
	pb.RegisterMembershipServiceServer(srv, &membershipService{lister: lister})
}

type membershipService struct {
	pb.UnimplementedMembershipServiceServer
	lister MemberLister
}

func (m *membershipService) ListMembers(_ context.Context, _ *pb.ListMembersRequest) (*pb.ListMembersResponse, error) {
	members := m.lister.Members()
	out := make([]*pb.Member, 0, len(members))
	for _, mem := range members {
		out = append(out, memberToProto(mem))
	}
	return &pb.ListMembersResponse{Members: out}, nil
}

func (m *membershipService) GetLeader(_ context.Context, _ *pb.GetLeaderRequest) (*pb.GetLeaderResponse, error) {
	leader, ok := m.lister.Leader()
	if !ok {
		return nil, status.Error(codes.Unavailable, "no leader elected")
	}
	return &pb.GetLeaderResponse{
		LeaderId: leader.ID,
		Address:  leader.Address,
	}, nil
}

// memberToProto converts an internal Member to its proto representation.
func memberToProto(m membership.Member) *pb.Member {
	return &pb.Member{
		Id:      m.ID,
		Address: m.Address,
		Status:  string(m.Status),
		Leader:  m.Leader,
		Role:    membership.NormalizeRole(m.Role),
	}
}
