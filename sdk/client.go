package clusdr

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

type client struct {
	conn *grpc.ClientConn
	opts options

	mem    pb.MembershipServiceClient
	watch  pb.WatchServiceClient
	ev     pb.EventServiceClient
	health pb.HealthServiceClient
	lock   pb.LockServiceClient
	lease  pb.LeaseServiceClient

	holder string

	mu     sync.Mutex
	held   map[string]*Lock
	leased map[string]*Lease
}

func dial(o options) (Cluster, error) {
	if o.addr == "" {
		return nil, fmt.Errorf("clusdr: empty dial address")
	}
	if !o.insecure && envInsecure() && o.dataDir == "" {
		o.insecure = true
	}
	creds, err := transportCreds(o)
	if err != nil {
		return nil, err
	}
	conn, err := dialAddr(o.addr, creds)
	if err != nil {
		return nil, fmt.Errorf("clusdr: dial %s: %w", o.addr, err)
	}
	holder := o.holder
	if holder == "" {
		id, err := newHolderID()
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("clusdr: holder: %w", err)
		}
		holder = id
	}
	c := &client{
		conn:   conn,
		opts:   o,
		mem:    pb.NewMembershipServiceClient(conn),
		watch:  pb.NewWatchServiceClient(conn),
		ev:     pb.NewEventServiceClient(conn),
		health: pb.NewHealthServiceClient(conn),
		lock:   pb.NewLockServiceClient(conn),
		lease:  pb.NewLeaseServiceClient(conn),
		holder: holder,
		held:   make(map[string]*Lock),
		leased: make(map[string]*Lease),
	}
	if o.readyTimeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), o.readyTimeout)
		err := retry(ctx, func() error {
			_, e := c.health.Health(ctx, &pb.HealthRequest{})
			return e
		})
		cancel()
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("clusdr: daemon not ready at %s: %w", o.addr, err)
		}
	}
	return c, nil
}

func transportCreds(o options) (credentials.TransportCredentials, error) {
	if o.insecure {
		return insecure.NewCredentials(), nil
	}
	dir := o.dataDir
	if dir == "" {
		dir = envDataDir()
	}
	if dir == "" || !pemFilesPresent(dir) {
		if dir == "" {
			dir = "(no data dir)"
		}
		return nil, fmt.Errorf("clusdr: TLS enabled but %s/%s/%s missing in %s; set CLUSDR_TLS=disabled or pass WithInsecure()", caFile, certFile, keyFile, dir)
	}
	return clientFromDir(dir)
}

func newHolderID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("sdk-%x", b), nil
}

func (c *client) withRPC(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.opts.requestTimeout)
}

func (c *client) Members(ctx context.Context) ([]Member, error) {
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.ListMembersResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.mem.ListMembers(ctx, &pb.ListMembersRequest{})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("clusdr: members: %w", err)
	}
	out := make([]Member, 0, len(resp.Members))
	for _, m := range resp.Members {
		role := m.GetRole()
		if role == "" {
			role = "voter"
		}
		out = append(out, Member{
			ID:      m.GetId(),
			Address: m.GetAddress(),
			Status:  m.GetStatus(),
			Leader:  m.GetLeader(),
			Role:    role,
		})
	}
	return out, nil
}

func (c *client) Leader(ctx context.Context) (Member, error) {
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.GetLeaderResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.mem.GetLeader(ctx, &pb.GetLeaderRequest{})
		return e
	})
	if err != nil {
		return Member{}, fmt.Errorf("clusdr: leader: %w", err)
	}
	return Member{
		ID:      resp.GetLeaderId(),
		Address: resp.GetAddress(),
		Status:  "alive",
		Leader:  true,
		Role:    "voter",
	}, nil
}

func (c *client) Publish(ctx context.Context, topic string, payload []byte) error {
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.PublishEventResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.ev.PublishEvent(ctx, &pb.PublishEventRequest{
			Topic:   topic,
			Payload: payload,
		})
		return e
	})
	if err != nil {
		return fmt.Errorf("clusdr: publish: %w", err)
	}
	if resp != nil && !resp.GetAccepted() {
		return fmt.Errorf("clusdr: publish rejected: %s", resp.GetMessage())
	}
	return nil
}

func (c *client) Watch(ctx context.Context, opts ...WatchOption) (<-chan Event, error) {
	wo, err := applyWatchOptions(opts)
	if err != nil {
		return nil, err
	}
	ch := make(chan Event, defaultWatchBuffer)
	go c.watchLoop(ctx, ch, wo)
	return ch, nil
}

func (c *client) watchLoop(ctx context.Context, ch chan Event, wo watchOptions) {
	defer close(ch)
	var lastSeq uint64
	backoff := 50 * time.Millisecond
	req := func(seq uint64) *pb.WatchRequest {
		return &pb.WatchRequest{LastSeq: seq, Topics: wo.topics, EventTypes: wo.eventTypes}
	}
	for {
		if ctx.Err() != nil {
			return
		}
		stream, err := c.watch.Watch(ctx, req(lastSeq))
		if err != nil {
			if ctx.Err() != nil || status.Code(err) == codes.Canceled {
				return
			}
			if status.Code(err) == codes.InvalidArgument {
				return
			}
			if !sleepBackoff(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = 50 * time.Millisecond
		for {
			resp, err := stream.Recv()
			if err != nil {
				if ctx.Err() != nil || status.Code(err) == codes.Canceled {
					return
				}
				if status.Code(err) == codes.InvalidArgument {
					return
				}
				break
			}
			if resp.GetSeq() > lastSeq {
				lastSeq = resp.GetSeq()
			}
			ev := Event{
				Type:      resp.GetType(),
				Source:    resp.GetSource(),
				Payload:   resp.GetPayload(),
				Timestamp: time.UnixMilli(resp.GetTimestampUnixMs()),
				Seq:       resp.GetSeq(),
			}
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
		if !sleepBackoff(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

func (c *client) Close() error {
	if c.conn == nil {
		return nil
	}
	c.releaseHeld()
	c.releaseLeases()
	return c.conn.Close()
}

func sleepBackoff(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > 2*time.Second {
		return 2 * time.Second
	}
	return d
}
