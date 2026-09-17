package clusdr

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func (c *client) Lease(ctx context.Context, name string, ttl time.Duration) (*Lease, error) {
	if l := c.heldLease(name); l != nil {
		return l, nil
	}
	rpcCtx, cancel := c.withRPC(ctx)
	var resp *pb.GrantResponse
	err := retry(rpcCtx, func() error {
		var e error
		resp, e = c.lease.Grant(rpcCtx, &pb.GrantRequest{
			Name:  name,
			Owner: c.holder,
			TtlMs: ttlMs(ttl),
		})
		return e
	})
	cancel()
	if err != nil {
		return nil, fmt.Errorf("clusdr: lease %q: %w", name, err)
	}
	if resp == nil || !resp.GetGranted() {
		owner := ""
		msg := "not granted"
		if resp != nil {
			owner = resp.GetOwner()
			if resp.GetMessage() != "" {
				msg = resp.GetMessage()
			}
		}
		if owner != "" {
			return nil, fmt.Errorf("clusdr: lease %q: %s (owner %s)", name, msg, owner)
		}
		return nil, fmt.Errorf("clusdr: lease %q: %s", name, msg)
	}
	return c.adoptLease(ctx, resp, name, ttl), nil
}

func (c *client) Renew(ctx context.Context, name string) error {
	l := c.heldLease(name)
	if l == nil {
		return fmt.Errorf("clusdr: lease %q is not held by this client", name)
	}
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.LeaseServiceRenewResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.lease.Renew(ctx, &pb.LeaseServiceRenewRequest{
			Name:         l.Name,
			Owner:        l.Owner,
			FencingToken: l.Token,
		})
		return e
	})
	if err != nil {
		return fmt.Errorf("clusdr: renew %q: %w", name, err)
	}
	if resp != nil && !resp.GetRenewed() {
		return fmt.Errorf("clusdr: renew %q: %s", name, resp.GetMessage())
	}
	if resp != nil && resp.GetDeadlineUnixMs() > 0 {
		l.mu.Lock()
		l.deadline = time.UnixMilli(resp.GetDeadlineUnixMs())
		l.mu.Unlock()
	}
	return nil
}

func (c *client) Revoke(ctx context.Context, name string) error {
	l := c.heldLease(name)
	if l == nil {
		return fmt.Errorf("clusdr: lease %q is not held by this client", name)
	}
	return l.drop(ctx)
}

func (l *Lease) drop(ctx context.Context) error {
	if l == nil || l.c == nil {
		return fmt.Errorf("clusdr: invalid lease")
	}
	l.stopRenew()
	ctx, cancel := l.c.withRPC(ctx)
	defer cancel()
	var resp *pb.RevokeResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = l.c.lease.Revoke(ctx, &pb.RevokeRequest{
			Name:         l.Name,
			Owner:        l.Owner,
			FencingToken: l.Token,
		})
		return e
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			l.c.forgetLease(l.Name, l.Token)
			return fmt.Errorf("clusdr: revoke %q: %w", l.Name, err)
		}
		return fmt.Errorf("clusdr: revoke %q: %w", l.Name, err)
	}
	if resp != nil && !resp.GetRevoked() {
		return fmt.Errorf("clusdr: revoke %q: %s", l.Name, resp.GetMessage())
	}
	l.c.forgetLease(l.Name, l.Token)
	return nil
}

func (c *client) adoptLease(life context.Context, resp *pb.GrantResponse, name string, ttl time.Duration) *Lease {
	l := leaseFromResp(resp, name)
	l.c = c
	c.mu.Lock()
	if existing, ok := c.leased[name]; ok && existing.Token == l.Token {
		c.mu.Unlock()
		return existing
	}
	c.leased[name] = l
	c.mu.Unlock()
	c.startLeaseRenew(life, l, ttl)
	return l
}

func (c *client) startLeaseRenew(life context.Context, l *Lease, ttl time.Duration) {
	interval := renewInterval(l.Deadline(), ttl)
	ctx, cancel := context.WithCancel(life)
	l.mu.Lock()
	l.stop = cancel
	l.mu.Unlock()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				rctx, rcancel := context.WithTimeout(ctx, c.opts.requestTimeout)
				resp, err := c.lease.Renew(rctx, &pb.LeaseServiceRenewRequest{
					Name:         l.Name,
					Owner:        l.Owner,
					FencingToken: l.Token,
				})
				rcancel()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					if status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.Canceled {
						return
					}
					continue
				}
				if resp != nil && resp.GetDeadlineUnixMs() > 0 {
					l.mu.Lock()
					l.deadline = time.UnixMilli(resp.GetDeadlineUnixMs())
					l.mu.Unlock()
				}
			}
		}
	}()
}

func (l *Lease) stopRenew() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop != nil {
		l.stop()
		l.stop = nil
	}
}

func (c *client) heldLease(name string) *Lease {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.leased[name]
}

func (c *client) forgetLease(name string, token uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.leased[name]; ok && l.Token == token {
		delete(c.leased, name)
	}
}

func (c *client) releaseLeases() {
	c.mu.Lock()
	held := make([]*Lease, 0, len(c.leased))
	for _, l := range c.leased {
		held = append(held, l)
	}
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.requestTimeout)
	defer cancel()
	for _, l := range held {
		_ = l.drop(ctx)
	}
}

func leaseFromResp(resp *pb.GrantResponse, name string) *Lease {
	l := &Lease{Name: name}
	if resp == nil {
		return l
	}
	if resp.GetOwner() != "" {
		l.Owner = resp.GetOwner()
	}
	l.Token = resp.GetFencingToken()
	if resp.GetDeadlineUnixMs() > 0 {
		l.deadline = time.UnixMilli(resp.GetDeadlineUnixMs())
	}
	return l
}
