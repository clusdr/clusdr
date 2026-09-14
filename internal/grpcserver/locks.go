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

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
	"github.com/odurgut/clusdr/internal/locks"
	"github.com/odurgut/clusdr/internal/membership"
	"github.com/odurgut/clusdr/internal/mtls"
)

// LockRaft is the consensus surface LockService uses. Implemented by *consensus.Node.
type LockRaft interface {
	IsLeader() bool
	ApplyLockAcquire(name, holder string, ttl time.Duration) (token uint64, acquired bool, err error)
	ApplyLockRelease(name, holder string, token uint64) error
	ApplyLockRenew(name, holder string, token uint64, ttl time.Duration) (deadline time.Time, err error)
}

// LockLeaderFinder is membership read for forwarding to the Raft leader.
type LockLeaderFinder interface {
	SelfID() string
	Leader() (addrID string, addr string, ok bool)
}

type membershipLeader struct {
	j Joiner
}

func (m membershipLeader) SelfID() string { return m.j.SelfID() }
func (m membershipLeader) Leader() (string, string, bool) {
	l, ok := m.j.Leader()
	if !ok {
		return "", "", false
	}
	return l.ID, l.Address, true
}

// RegisterLockService attaches LockService.
// raft may be nil in tests that inject a stub.
// defaultTTL is used when the client sends ttl_ms=0; 0 → locks.DefaultTTL.
func RegisterLockService(
	srv *grpc.Server,
	table *locks.Table,
	raft LockRaft,
	nodes Joiner,
	log *slog.Logger,
	peerCreds func() credentials.TransportCredentials,
	defaultTTL time.Duration,
) {
	if defaultTTL <= 0 {
		defaultTTL = locks.DefaultTTL
	}
	pb.RegisterLockServiceServer(srv, &lockService{
		table:      table,
		raft:       raft,
		nodes:      membershipLeader{j: nodes},
		members:    nodes,
		log:        log,
		peerCreds:  peerCreds,
		defaultTTL: defaultTTL,
	})
}

type lockService struct {
	pb.UnimplementedLockServiceServer
	table      *locks.Table
	raft       LockRaft
	nodes      LockLeaderFinder
	members    Joiner
	log        *slog.Logger
	peerCreds  func() credentials.TransportCredentials
	defaultTTL time.Duration
}

func (s *lockService) rejectIfObserver() error {
	if s.members == nil {
		return nil
	}
	self := s.members.SelfID()
	for _, m := range s.members.Members() {
		if m.ID == self && membership.NormalizeRole(m.Role) == membership.RoleObserver {
			return status.Error(codes.FailedPrecondition, "observer cannot mutate locks")
		}
	}
	return nil
}

func (s *lockService) Lock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	if err := locks.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.rejectIfObserver(); err != nil {
		return nil, err
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardLock(ctx, req)
	}
	holder := s.holder(req.GetHolder())
	ttl := s.ttlFromMs(req.GetTtlMs(), 0)
	for {
		if err := ctx.Err(); err != nil {
			return nil, status.FromContextError(err).Err()
		}
		tok, ok, err := s.applyAcquire(req.GetName(), holder, ttl)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "lock: %v", err)
		}
		if ok {
			return s.grantResponse(req.GetName(), holder, tok), nil
		}
		if err := s.table.Wait(ctx); err != nil {
			return nil, status.FromContextError(err).Err()
		}
	}
}

func (s *lockService) TryLock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	if err := locks.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.rejectIfObserver(); err != nil {
		return nil, err
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardTryLock(ctx, req)
	}
	holder := s.holder(req.GetHolder())
	ttl := s.ttlFromMs(req.GetTtlMs(), 0)
	tok, ok, err := s.applyAcquire(req.GetName(), holder, ttl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "trylock: %v", err)
	}
	if !ok {
		held, _ := s.table.Get(req.GetName())
		return &pb.LockResponse{
			Acquired:       false,
			Message:        "held",
			FencingToken:   held.Token,
			Holder:         held.Holder,
			DeadlineUnixMs: unixMs(held.Deadline),
		}, nil
	}
	return s.grantResponse(req.GetName(), holder, tok), nil
}

func (s *lockService) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	if err := locks.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.rejectIfObserver(); err != nil {
		return nil, err
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardUnlock(ctx, req)
	}
	holder := s.holder(req.GetHolder())
	if err := s.applyRelease(req.GetName(), holder, req.GetFencingToken()); err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.UnlockResponse{Released: true}, nil
}

func (s *lockService) Renew(ctx context.Context, req *pb.RenewLockRequest) (*pb.RenewLockResponse, error) {
	if err := locks.ValidName(req.GetName()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.rejectIfObserver(); err != nil {
		return nil, err
	}
	if s.raft != nil && !s.raft.IsLeader() {
		return s.forwardRenew(ctx, req)
	}
	holder := s.holder(req.GetHolder())
	reuse := s.defaultTTL
	if rec, ok := s.table.Get(req.GetName()); ok && rec.TTL > 0 {
		reuse = rec.TTL
	}
	ttl := s.ttlFromMs(req.GetTtlMs(), reuse)
	deadline, err := s.applyRenew(req.GetName(), holder, req.GetFencingToken(), ttl)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pb.RenewLockResponse{
		Renewed:        true,
		FencingToken:   req.GetFencingToken(),
		DeadlineUnixMs: unixMs(deadline),
	}, nil
}

func (s *lockService) ListLocks(_ context.Context, _ *pb.ListLocksRequest) (*pb.ListLocksResponse, error) {
	if s.table == nil {
		return &pb.ListLocksResponse{}, nil
	}
	recs := s.table.List()
	out := make([]*pb.LockInfo, 0, len(recs))
	for _, rec := range recs {
		out = append(out, &pb.LockInfo{
			Name:           rec.Name,
			Holder:         rec.Holder,
			FencingToken:   rec.Token,
			AcquiredUnixMs: rec.AcquiredAt.UnixMilli(),
			DeadlineUnixMs: unixMs(rec.Deadline),
		})
	}
	return &pb.ListLocksResponse{Locks: out}, nil
}

func (s *lockService) grantResponse(name, holder string, tok uint64) *pb.LockResponse {
	resp := &pb.LockResponse{
		Acquired:     true,
		FencingToken: tok,
		Holder:       holder,
	}
	if s.table != nil {
		if rec, ok := s.table.Get(name); ok {
			resp.DeadlineUnixMs = unixMs(rec.Deadline)
		}
	}
	return resp
}

func (s *lockService) ttlFromMs(ms int64, reuse time.Duration) time.Duration {
	var requested time.Duration
	if ms > 0 {
		requested = time.Duration(ms) * time.Millisecond
	} else {
		requested = reuse
	}
	return locks.ClampTTL(requested, s.defaultTTL)
}

func (s *lockService) holder(h string) string {
	if h != "" {
		return h
	}
	if s.nodes != nil {
		return s.nodes.SelfID()
	}
	return ""
}

func (s *lockService) applyAcquire(name, holder string, ttl time.Duration) (uint64, bool, error) {
	if s.raft != nil {
		return s.raft.ApplyLockAcquire(name, holder, ttl)
	}
	if s.table == nil {
		return 0, false, fmt.Errorf("lock table not initialized")
	}
	return s.table.Acquire(name, holder, time.Now().Add(ttl), ttl)
}

func (s *lockService) applyRelease(name, holder string, token uint64) error {
	if s.raft != nil {
		return s.raft.ApplyLockRelease(name, holder, token)
	}
	if s.table == nil {
		return fmt.Errorf("lock table not initialized")
	}
	return s.table.Release(name, holder, token)
}

func (s *lockService) applyRenew(name, holder string, token uint64, ttl time.Duration) (time.Time, error) {
	if s.raft != nil {
		return s.raft.ApplyLockRenew(name, holder, token, ttl)
	}
	if s.table == nil {
		return time.Time{}, fmt.Errorf("lock table not initialized")
	}
	deadline := time.Now().Add(ttl)
	if err := s.table.Renew(name, holder, token, deadline, ttl); err != nil {
		return time.Time{}, err
	}
	return deadline, nil
}

func (s *lockService) forwardLock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLockServiceClient(cc).Lock(ctx, req)
}

func (s *lockService) forwardTryLock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLockServiceClient(cc).TryLock(ctx, req)
}

func (s *lockService) forwardUnlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLockServiceClient(cc).Unlock(ctx, req)
}

func (s *lockService) forwardRenew(ctx context.Context, req *pb.RenewLockRequest) (*pb.RenewLockResponse, error) {
	cc, err := s.dialLeader()
	if err != nil {
		return nil, err
	}
	defer cc.Close()
	return pb.NewLockServiceClient(cc).Renew(ctx, req)
}

func (s *lockService) dialLeader() (*grpc.ClientConn, error) {
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

func unixMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
