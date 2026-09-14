package presence

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/odurgut/clusdr/internal/mtls"
	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

// RaftMutator is the leader-side lease surface. Implemented by *consensus.Node.
type RaftMutator interface {
	IsLeader() bool
	ApplyLeaseGrant(name, owner string, ttl time.Duration) (token uint64, granted bool, err error)
	ApplyLeaseRenew(name, owner string, token uint64, ttl time.Duration) (deadline time.Time, err error)
}

// LeaderFinder is membership read used to dial the Raft leader.
type LeaderFinder interface {
	SelfID() string
	Leader() (id string, addr string, ok bool)
}

// RaftHolder grants/renews locally on the leader and forwards otherwise.
type RaftHolder struct {
	Raft      RaftMutator
	Nodes     LeaderFinder
	PeerCreds func() credentials.TransportCredentials
}

// Grant implements Holder.
func (h *RaftHolder) Grant(ctx context.Context, name, owner string, ttl time.Duration) (uint64, error) {
	if h.Raft != nil && h.Raft.IsLeader() {
		tok, ok, err := h.Raft.ApplyLeaseGrant(name, owner, ttl)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, ErrHeld
		}
		return tok, nil
	}
	cc, err := h.dialLeader()
	if err != nil {
		return 0, err
	}
	defer cc.Close()
	resp, err := pb.NewLeaseServiceClient(cc).Grant(ctx, &pb.GrantLeaseRequest{
		Name:  name,
		Owner: owner,
		TtlMs: ttl.Milliseconds(),
	})
	if err != nil {
		return 0, err
	}
	if !resp.GetGranted() {
		return 0, ErrHeld
	}
	return resp.GetFencingToken(), nil
}

// Renew implements Holder.
func (h *RaftHolder) Renew(ctx context.Context, name, owner string, token uint64, ttl time.Duration) error {
	if h.Raft != nil && h.Raft.IsLeader() {
		_, err := h.Raft.ApplyLeaseRenew(name, owner, token, ttl)
		return err
	}
	cc, err := h.dialLeader()
	if err != nil {
		return err
	}
	defer cc.Close()
	resp, err := pb.NewLeaseServiceClient(cc).Renew(ctx, &pb.RenewLeaseRequest{
		Name:         name,
		Owner:        owner,
		FencingToken: token,
		TtlMs:        ttl.Milliseconds(),
	})
	if err != nil {
		return err
	}
	if !resp.GetRenewed() {
		return fmt.Errorf("presence renew rejected")
	}
	return nil
}

func (h *RaftHolder) dialLeader() (*grpc.ClientConn, error) {
	if h.Nodes == nil {
		return nil, fmt.Errorf("NOT_LEADER")
	}
	id, addr, ok := h.Nodes.Leader()
	if !ok || addr == "" || id == h.Nodes.SelfID() {
		return nil, fmt.Errorf("NOT_LEADER")
	}
	var creds credentials.TransportCredentials
	if h.PeerCreds != nil {
		creds = h.PeerCreds()
	}
	cc, err := mtls.Dial(addr, creds)
	if err != nil {
		return nil, fmt.Errorf("dial leader %s: %w", id, err)
	}
	return cc, nil
}
