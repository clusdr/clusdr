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

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
	"github.com/durguto/clusdr/internal/leases"
	"github.com/durguto/clusdr/internal/mtls"
)

// LeaseRaft is the consensus surface LeaseService uses. Implemented by *consensus.Node.
type LeaseRaft interface {
	IsLeader() bool
	ApplyLeaseGrant(name, owner string, ttl time.Duration) (token uint64, granted bool, err error)
	ApplyLeaseRenew(name, owner string, token uint64, ttl time.Duration) (deadline time.Time, err error)
	ApplyLeaseRevoke(name, owner string, token uint64) error
}

// RegisterLeaseService attaches LeaseService.
// raft may be nil in tests. defaultTTL 0 → leases.DefaultTTL.
func RegisterLeaseService(
	srv *grpc.Server,
	table *leases.Table,
	raft LeaseRaft,
	nodes Joiner,
	log *slog.Logger,
	peerCreds func() credentials.TransportCredentials,
	defaultTTL time.Duration,
) {
	if defaultTTL <= 0 {
		defaultTTL = leases.DefaultTTL
	}
	pb.RegisterLeaseServiceServer(srv, &leaseService{
		table:      table,
		raft:       raft,
		nodes:      membershipLeader{j: nodes},
		log:        log,
		peerCreds:  peerCreds,
		defaultTTL: defaultTTL,
	})
}

type leaseService struct {
	pb.UnimplementedLeaseServiceServer
	table      *leases.Table
	raft       LeaseRaft
	nodes      LockLeaderFinder
	log        *slog.Logger
	peerCreds  func() credentials.TransportCredentials
	defaultTTL time.Duration
}

func (s *leaseService) Grant(ctx context.Context, req *pb.GrantLeaseRequest) (*pb.GrantLeaseResponse, error) {
	if err := leases.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardGrant(ctx, req)
	}
	owner := s.owner(req.GetOwner())
	ttl := s.ttlFromMs(req.GetTtlMs(), 0)
	tok, ok, err := s.applyGrant(req.GetName(), owner, ttl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "grant: %v", err)
	}
	if !ok {
		held, _ := s.table.Get(req.GetName())
		return &pb.GrantLeaseResponse{
			Granted:        false,
			Message:        "held",
			FencingToken:   held.Token,
			Owner:          held.Owner,
			DeadlineUnixMs: unixMs(held.Deadline),
		}, nil
	}
	return s.grantResponse(req.GetName(), owner, tok), nil
}

func (s *leaseService) Renew(ctx context.Context, req *pb.RenewLeaseRequest) (*pb.RenewLeaseResponse, error) {
	if err := leases.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardRenew(ctx, req)
	}
	owner := s.owner(req.GetOwner())
	reuse := s.defaultTTL
	if rec, ok := s.table.Get(req.GetName()); ok && rec.TTL > 0 {
		reuse = rec.TTL
	}
	ttl := s.ttlFromMs(req.GetTtlMs(), reuse)
	deadline, err := s.applyRenew(req.GetName(), owner, req.GetFencingToken(), ttl)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.RenewLeaseResponse{
		Renewed:        true,
		FencingToken:   req.GetFencingToken(),
		DeadlineUnixMs: unixMs(deadline),
	}, nil
}

func (s *leaseService) Revoke(ctx context.Context, req *pb.RevokeLeaseRequest) (*pb.RevokeLeaseResponse, error) {
	if err := leases.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardRevoke(ctx, req)
	}
	owner := s.owner(req.GetOwner())
	if err := s.applyRevoke(req.GetName(), owner, req.GetFencingToken()); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.RevokeLeaseResponse{Revoked: true}, nil
}

func (s *leaseService) ListLeases(_ context.Context, _ *pb.ListLeasesRequest) (*pb.ListLeasesResponse, error) {
	if s.table == nil {
		return &pb.ListLeasesResponse{}, nil
	}
	recs := s.table.List()
	out := make([]*pb.LeaseInfo, 0, len(recs))
	for _, rec := range recs {
		out = append(out, &pb.LeaseInfo{
			Name:           rec.Name,
			Owner:          rec.Owner,
			FencingToken:   rec.Token,
			GrantedUnixMs:  rec.GrantedAt.UnixMilli(),
			DeadlineUnixMs: unixMs(rec.Deadline),
		})
	}
	return &pb.ListLeasesResponse{Leases: out}, nil
}

func (s *leaseService) grantResponse(name, owner string, tok uint64) *pb.GrantLeaseResponse {
	resp := &pb.GrantLeaseResponse{
		Granted:      true,
		FencingToken: tok,
		Owner:        owner,
	}
	if s.table != nil {
		if rec, ok := s.table.Get(name); ok {
			resp.DeadlineUnixMs = unixMs(rec.Deadline)
		}
	}
	return resp
}

func (s *leaseService) ttlFromMs(ms int64, reuse time.Duration) time.Duration {
	var requested time.Duration
	if ms > 0 {
		requested = time.Duration(ms) * time.Millisecond
	} else {
		requested = reuse
	}
	return leases.ClampTTL(requested, s.defaultTTL)
}

func (s *leaseService) owner(h string) string {
	if h != "" {
		return h
	}
	if s.nodes != nil {
		return s.nodes.SelfID()
	}
	return ""
}

func (s *leaseService) applyGrant(name, owner string, ttl time.Duration) (uint64, bool, error) {
	if s.raft != nil {
		return s.raft.ApplyLeaseGrant(name, owner, ttl)
	}
	if s.table == nil {
		return 0, false, fmt.Errorf("lease table not initialized")
	}
	return s.table.Grant(name, owner, time.Now().Add(ttl), ttl)
}

func (s *leaseService) applyRenew(name, owner string, token uint64, ttl time.Duration) (time.Time, error) {
	if s.raft != nil {
		return s.raft.ApplyLeaseRenew(name, owner, token, ttl)
	}
	if s.table == nil {
		return time.Time{}, fmt.Errorf("lease table not initialized")
	}
	deadline := time.Now().Add(ttl)
	if err := s.table.Renew(name, owner, token, deadline, ttl); err != nil {
		return time.Time{}, err
	}
	return deadline, nil
}

func (s *leaseService) applyRevoke(name, owner string, token uint64) error {
	if s.raft != nil {
		return s.raft.ApplyLeaseRevoke(name, owner, token)
	}
	if s.table == nil {
		return fmt.Errorf("lease table not initialized")
	}
	return s.table.Revoke(name, owner, token)
}

func (s *leaseService) forwardGrant(ctx context.Context, req *pb.GrantLeaseRequest) (*pb.GrantLeaseResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLeaseServiceClient(cc).Grant(ctx, req)
}

func (s *leaseService) forwardRenew(ctx context.Context, req *pb.RenewLeaseRequest) (*pb.RenewLeaseResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLeaseServiceClient(cc).Renew(ctx, req)
}

func (s *leaseService) forwardRevoke(ctx context.Context, req *pb.RevokeLeaseRequest) (*pb.RevokeLeaseResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLeaseServiceClient(cc).Revoke(ctx, req)
}

func (s *leaseService) dialLeader() (*grpc.ClientConn, error) {
	if s.nodes == nil {
		return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
	}
	id, addr, ok := s.nodes.Leader()
	if !ok || addr == "" || id == s.nodes.SelfID() {
		return nil, status.Error(codes.FailedPrecondition, "NOT_LEADER")
	}
	conn, err := mtls.Dial(addr, credsOf(s.peerCreds))
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "dial leader %s: %v", id, err)
	}
	return conn, nil
}
