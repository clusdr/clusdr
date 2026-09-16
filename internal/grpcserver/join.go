package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
	"github.com/clusdr/clusdr/internal/membership"
	"github.com/clusdr/clusdr/internal/mtls"
)

// Joiner is the read interface the join service uses for identity and the
// member list. Membership writes go through the Raft FSM (VoterAdder), not
// this interface — except when voter is nil (unit tests without Raft).
type Joiner interface {
	Members() []membership.Member
	SelfID() string
	Leader() (membership.Member, bool)
}

// VoterAdder is implemented by *consensus.Node. The join service calls it on
// the leader to:
//   - add the node to the Raft configuration (AddVoter or AddNonvoter)
//   - write an add_member command to the Raft log so all FSMs update their
//     membership engines (ApplyAddMember / ApplyAddMemberAs)
//
// Defined here (grpcserver package) so grpcserver does not import consensus.
type VoterAdder interface {
	AddVoter(id, raftAddr string) error
	AddNonvoter(id, raftAddr string) error
	PromoteToVoter(id string) error
	ApplyAddMember(id, addr string) error
	ApplyAddMemberAs(id, addr, role string) error
	IsLeader() bool
}

// JoinSecurity is the optional PKI surface for JoinService.
// A nil JoinSecurity means no token and no certs (unit tests).
type JoinSecurity interface {
	// CheckToken validates a join token. Returns a non-nil error when the
	// cluster requires a token and the supplied value does not match.
	CheckToken(token string) error
	// IssueNode signs a node certificate. Returns PEM cert, key, CA cert,
	// and the join-token hash for the joining node to persist.
	IssueNode(nodeID string) (nodeCert, nodeKey, caCert, tokenHash []byte, err error)
}

// JoinBootstrap persists certificates returned by a successful join.
type JoinBootstrap interface {
	SaveJoinCerts(caCert, nodeCert, nodeKey, tokenHash []byte) error
}

// RegisterJoinService registers the node-to-node JoinService on srv.
//
// When voter is set, this node is part of a Raft cluster: the leader commits
// membership via AddVoter + ApplyAddMember (FSM is the only writer). Followers
// forward Join to the leader and do not mutate local membership. Gossip
// fan-out is not used.
//
// voter may be nil in unit tests; then Join updates the in-memory engine
// directly (the test double for a single-node FSM).
// sec may be nil; when set, joins require a valid token and the joining node
// receives a CA-issued certificate.
// peerCreds, when non-nil, is used to dial the leader when forwarding.
func RegisterJoinService(srv *grpc.Server, clusterID string, joiner Joiner, voter VoterAdder, log *slog.Logger, sec JoinSecurity, peerCreds func() credentials.TransportCredentials) {
	pb.RegisterJoinServiceServer(srv, &joinService{
		clusterID: clusterID,
		joiner:    joiner,
		voter:     voter,
		log:       log,
		sec:       sec,
		peerCreds: peerCreds,
	})
}

type joinService struct {
	pb.UnimplementedJoinServiceServer
	clusterID string
	joiner    Joiner
	voter     VoterAdder // nil in unit tests without Raft
	log       *slog.Logger
	sec       JoinSecurity
	peerCreds func() credentials.TransportCredentials
}

// Join handles an incoming join request from another node.
func (j *joinService) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	if j.clusterID != "" && req.ClusterId != "" && req.ClusterId != j.clusterID {
		return &pb.JoinResponse{
			Accepted: false,
			Message:  fmt.Sprintf("cluster id mismatch: want %q", j.clusterID),
		}, nil
	}

	if j.sec != nil {
		if err := j.sec.CheckToken(req.GetJoinToken()); err != nil {
			return nil, status.Error(codes.Unauthenticated, "UNAUTHORIZED: invalid join token")
		}
	}

	if j.voter != nil && !j.voter.IsLeader() {
		if req.GetRelay() {
			return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
		}
		return j.forwardToLeader(ctx, req)
	}

	role := membership.RoleVoter
	if req.GetObserver() {
		role = membership.RoleObserver
	}

	if j.voter != nil {
		if req.RaftAddr != "" {
			var err error
			if req.GetObserver() {
				err = j.voter.AddNonvoter(req.NodeId, req.RaftAddr)
			} else {
				err = j.voter.AddVoter(req.NodeId, req.RaftAddr)
			}
			if err != nil {
				kind := "voter"
				if req.GetObserver() {
					kind = "nonvoter"
				}
				return nil, status.Errorf(codes.Internal, "add raft %s: %v", kind, err)
			}
		}
		if err := j.voter.ApplyAddMemberAs(req.NodeId, req.Address, role); err != nil {
			return nil, status.Errorf(codes.Internal, "apply add_member: %v", err)
		}
	} else {
		// No Raft (unit tests): the in-memory engine is the stand-in FSM.
		if err := joinLocal(j.joiner, req.NodeId, req.Address, role); err != nil {
			return nil, status.Errorf(codes.Internal, "join: %v", err)
		}
	}

	members := j.joiner.Members()
	out := make([]*pb.Member, 0, len(members))
	for _, m := range members {
		out = append(out, memberToProto(m))
	}
	resp := &pb.JoinResponse{Accepted: true, Members: out}

	if j.sec != nil {
		cert, key, ca, hash, err := j.sec.IssueNode(req.NodeId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "issue node cert: %v", err)
		}
		resp.NodeCert = cert
		resp.NodeKey = key
		resp.CaCert = ca
		resp.JoinTokenHash = hash
	}
	return resp, nil
}

const fanoutTimeout = 3 * time.Second

func joinLocal(j Joiner, id, addr, role string) error {
	if ja, ok := j.(interface {
		JoinAs(id, addr, role string) (bool, error)
	}); ok {
		_, err := ja.JoinAs(id, addr, role)
		return err
	}
	lj, ok := j.(interface {
		Join(id, addr string) (bool, error)
	})
	if !ok {
		return fmt.Errorf("join: no raft voter and engine cannot join")
	}
	_, err := lj.Join(id, addr)
	return err
}

// forwardToLeader sends Join to the current Raft leader. Relay=true prevents
// a second hop if the destination is also not the leader.
func (j *joinService) forwardToLeader(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	leader, ok := j.joiner.Leader()
	if !ok || leader.Address == "" || leader.ID == j.joiner.SelfID() {
		return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
	}

	fwd := &pb.JoinRequest{
		NodeId:    req.NodeId,
		ClusterId: req.ClusterId,
		Address:   req.Address,
		RaftAddr:  req.RaftAddr,
		JoinToken: req.JoinToken,
		Relay:     true,
		Observer:  req.Observer,
	}

	dctx, cancel := context.WithTimeout(ctx, fanoutTimeout)
	defer cancel()

	conn, err := mtls.Dial(leader.Address, credsOf(j.peerCreds))
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "dial leader %s: %v", leader.ID, err)
	}
	defer conn.Close()

	return pb.NewJoinServiceClient(conn).Join(dctx, fwd)
}

func (j *joinService) Promote(ctx context.Context, req *pb.PromoteRequest) (*pb.PromoteResponse, error) {
	id := req.GetNodeId()
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	if j.voter != nil && !j.voter.IsLeader() {
		if req.GetRelay() {
			return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
		}
		return j.forwardPromote(ctx, req)
	}

	if err := promoteMember(j.joiner, j.voter, id); err != nil {
		return nil, err
	}
	return &pb.PromoteResponse{Promoted: true, Members: protoMembers(j.joiner)}, nil
}

func (j *joinService) forwardPromote(ctx context.Context, req *pb.PromoteRequest) (*pb.PromoteResponse, error) {
	leader, ok := j.joiner.Leader()
	if !ok || leader.Address == "" || leader.ID == j.joiner.SelfID() {
		return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
	}

	dctx, cancel := context.WithTimeout(ctx, fanoutTimeout)
	defer cancel()

	conn, err := mtls.Dial(leader.Address, credsOf(j.peerCreds))
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "dial leader %s: %v", leader.ID, err)
	}
	defer conn.Close()

	return pb.NewJoinServiceClient(conn).Promote(dctx, &pb.PromoteRequest{
		NodeId: req.NodeId,
		Relay:  true,
	})
}

func protoMembers(j Joiner) []*pb.Member {
	members := j.Members()
	out := make([]*pb.Member, 0, len(members))
	for _, m := range members {
		out = append(out, memberToProto(m))
	}
	return out
}

func memberByID(j Joiner, id string) (membership.Member, bool) {
	for _, m := range j.Members() {
		if m.ID == id {
			return m, true
		}
	}
	return membership.Member{}, false
}

// promoteMember turns id into a voter. voter may be nil (unit tests).
func promoteMember(j Joiner, voter VoterAdder, id string) error {
	m, ok := memberByID(j, id)
	if !ok {
		return status.Errorf(codes.NotFound, "member %s not found", id)
	}
	if m.Status != membership.StatusAlive {
		return status.Errorf(codes.FailedPrecondition, "member %s is %s", id, m.Status)
	}
	if membership.NormalizeRole(m.Role) == membership.RoleVoter {
		return nil
	}
	if voter != nil {
		if err := voter.PromoteToVoter(id); err != nil {
			return status.Errorf(codes.Internal, "promote raft voter: %v", err)
		}
		if err := voter.ApplyAddMemberAs(id, m.Address, membership.RoleVoter); err != nil {
			return status.Errorf(codes.Internal, "apply add_member: %v", err)
		}
		return nil
	}
	if err := joinLocal(j, id, m.Address, membership.RoleVoter); err != nil {
		return status.Errorf(codes.Internal, "promote: %v", err)
	}
	return nil
}

// RegisterControlService registers the CLI-to-daemon ControlService on srv.
// raftAddr is this node's Raft TCP address, forwarded in the JoinRequest so
// the remote leader can call AddVoter or AddNonvoter.
func RegisterControlService(srv *grpc.Server, clusterID, raftAddr string, joiner Joiner, boot JoinBootstrap, joinCreds credentials.TransportCredentials, voter VoterAdder, peerCreds func() credentials.TransportCredentials) {
	pb.RegisterControlServiceServer(srv, &controlService{
		clusterID: clusterID,
		raftAddr:  raftAddr,
		joiner:    joiner,
		boot:      boot,
		joinCreds: joinCreds,
		voter:     voter,
		peerCreds: peerCreds,
	})
}

type controlService struct {
	pb.UnimplementedControlServiceServer
	clusterID string
	raftAddr  string // this node's Raft TCP address
	joiner    Joiner
	boot      JoinBootstrap
	joinCreds credentials.TransportCredentials
	voter     VoterAdder
	peerCreds func() credentials.TransportCredentials
}

// RequestJoin is called by the CLI. The daemon dials the remote node and
// triggers a node-to-node join, then updates local membership state.
func (c *controlService) RequestJoin(ctx context.Context, req *pb.RequestJoinRequest) (*pb.RequestJoinResponse, error) {
	if req.Addr == "" {
		return nil, status.Error(codes.InvalidArgument, "addr is required")
	}

	// Dial the remote node.
	conn, err := mtls.Dial(req.Addr, c.joinCreds)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "dial %s: %v", req.Addr, err)
	}
	defer conn.Close()

	// Find our own identity to send in the join request.
	selfID := c.joiner.SelfID()
	var selfAddr string
	for _, m := range c.joiner.Members() {
		if m.ID == selfID {
			selfAddr = m.Address
			break
		}
	}

	// Call the remote JoinService.
	resp, err := pb.NewJoinServiceClient(conn).Join(ctx, &pb.JoinRequest{
		NodeId:    selfID,
		ClusterId: c.clusterID,
		Address:   selfAddr,
		RaftAddr:  c.raftAddr,
		JoinToken: req.GetToken(),
		Observer:  req.GetObserver(),
	})
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			return nil, err
		}
		return nil, status.Errorf(codes.Unavailable, "remote join: %v", err)
	}
	if !resp.Accepted {
		return &pb.RequestJoinResponse{
			Joined:  false,
			Message: fmt.Sprintf("rejected: %s", resp.Message),
		}, nil
	}

	if c.boot != nil && len(resp.NodeCert) > 0 {
		if err := c.boot.SaveJoinCerts(resp.CaCert, resp.NodeCert, resp.NodeKey, resp.JoinTokenHash); err != nil {
			return nil, status.Errorf(codes.Internal, "save join certs: %v", err)
		}
	}

	// Membership on this node is filled by Raft log apply, not by copying
	// the JoinResponse. The CLI still gets the leader's committed list.
	return &pb.RequestJoinResponse{
		Joined:     true,
		Members:    resp.Members,
		CertIssued: len(resp.NodeCert) > 0,
	}, nil
}

func (c *controlService) RequestPromote(ctx context.Context, req *pb.RequestPromoteRequest) (*pb.RequestPromoteResponse, error) {
	id := req.GetNodeId()
	if id == "" {
		id = c.joiner.SelfID()
	}
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	if c.voter != nil && !c.voter.IsLeader() {
		resp, err := c.forwardPromote(ctx, id)
		if err != nil {
			return nil, err
		}
		return &pb.RequestPromoteResponse{Promoted: resp.Promoted, Message: resp.Message, Members: resp.Members}, nil
	}

	if err := promoteMember(c.joiner, c.voter, id); err != nil {
		return nil, err
	}
	return &pb.RequestPromoteResponse{Promoted: true, Members: protoMembers(c.joiner)}, nil
}

func (c *controlService) forwardPromote(ctx context.Context, id string) (*pb.PromoteResponse, error) {
	leader, ok := c.joiner.Leader()
	if !ok || leader.Address == "" || leader.ID == c.joiner.SelfID() {
		return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
	}
	dctx, cancel := context.WithTimeout(ctx, fanoutTimeout)
	defer cancel()
	conn, err := mtls.Dial(leader.Address, credsOf(c.peerCreds))
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "dial leader %s: %v", leader.ID, err)
	}
	defer conn.Close()
	return pb.NewJoinServiceClient(conn).Promote(dctx, &pb.PromoteRequest{NodeId: id, Relay: true})
}
