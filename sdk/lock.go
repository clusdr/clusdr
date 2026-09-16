package clusdr

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func (c *client) Lock(ctx context.Context, name string, ttl time.Duration) (*Lock, error) {
	if l := c.heldLock(name); l != nil {
		return l, nil
	}
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.LockResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.lock.Lock(ctx, &pb.LockRequest{
			Name:   name,
			Holder: c.holder,
			TtlMs:  ttlMs(ttl),
		})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("clusdr: lock %q: %w", name, err)
	}
	if resp == nil || !resp.GetAcquired() {
		msg := "not acquired"
		if resp != nil && resp.GetMessage() != "" {
			msg = resp.GetMessage()
		}
		return nil, fmt.Errorf("clusdr: lock %q: %s", name, msg)
	}
	return c.adopt(resp, name, ttl), nil
}

func (c *client) TryLock(ctx context.Context, name string, ttl time.Duration) (*Lock, bool, error) {
	if l := c.heldLock(name); l != nil {
		return l, true, nil
	}
	ctx, cancel := c.withRPC(ctx)
	defer cancel()
	var resp *pb.LockResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = c.lock.TryLock(ctx, &pb.LockRequest{
			Name:   name,
			Holder: c.holder,
			TtlMs:  ttlMs(ttl),
		})
		return e
	})
	if err != nil {
		return nil, false, fmt.Errorf("clusdr: trylock %q: %w", name, err)
	}
	if resp == nil || !resp.GetAcquired() {
		cur := lockFromResp(resp, name)
		return cur, false, nil
	}
	return c.adopt(resp, name, ttl), true, nil
}

func (c *client) Unlock(ctx context.Context, name string) error {
	l := c.heldLock(name)
	if l == nil {
		return fmt.Errorf("clusdr: lock %q is not held by this client", name)
	}
	return l.release(ctx)
}

func (l *Lock) release(ctx context.Context) error {
	if l == nil || l.c == nil {
		return fmt.Errorf("clusdr: invalid lock")
	}
	l.stopRenew()
	ctx, cancel := l.c.withRPC(ctx)
	defer cancel()
	var resp *pb.UnlockResponse
	err := retry(ctx, func() error {
		var e error
		resp, e = l.c.lock.Unlock(ctx, &pb.UnlockRequest{
			Name:         l.Name,
			Holder:       l.Holder,
			FencingToken: l.Token,
		})
		return e
	})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			l.c.forget(l.Name, l.Token)
			return fmt.Errorf("clusdr: unlock %q: %w", l.Name, err)
		}
		return fmt.Errorf("clusdr: unlock %q: %w", l.Name, err)
	}
	if resp != nil && !resp.GetReleased() {
		return fmt.Errorf("clusdr: unlock %q: %s", l.Name, resp.GetMessage())
	}
	l.c.forget(l.Name, l.Token)
	return nil
}

func (c *client) adopt(resp *pb.LockResponse, name string, ttl time.Duration) *Lock {
	l := lockFromResp(resp, name)
	l.c = c
	c.mu.Lock()
	if existing, ok := c.held[name]; ok && existing.Token == l.Token {
		c.mu.Unlock()
		return existing
	}
	c.held[name] = l
	c.mu.Unlock()
	c.startRenew(l, ttl)
	return l
}

func (c *client) startRenew(l *Lock, ttl time.Duration) {
	interval := renewInterval(l.Deadline(), ttl)
	ctx, cancel := context.WithCancel(context.Background())
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
				resp, err := c.lock.Renew(rctx, &pb.RenewLockRequest{
					Name:         l.Name,
					Holder:       l.Holder,
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

func (l *Lock) stopRenew() {
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

func (c *client) heldLock(name string) *Lock {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.held[name]
}

func (c *client) forget(name string, token uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.held[name]; ok && l.Token == token {
		delete(c.held, name)
	}
}

func (c *client) releaseHeld() {
	c.mu.Lock()
	held := make([]*Lock, 0, len(c.held))
	for _, l := range c.held {
		held = append(held, l)
	}
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.requestTimeout)
	defer cancel()
	for _, l := range held {
		_ = l.release(ctx)
	}
}

func lockFromResp(resp *pb.LockResponse, name string) *Lock {
	l := &Lock{Name: name}
	if resp == nil {
		return l
	}
	if resp.GetHolder() != "" {
		l.Holder = resp.GetHolder()
	}
	l.Token = resp.GetFencingToken()
	if resp.GetDeadlineUnixMs() > 0 {
		l.deadline = time.UnixMilli(resp.GetDeadlineUnixMs())
	}
	return l
}

func ttlMs(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return d.Milliseconds()
}

func renewInterval(deadline time.Time, ttl time.Duration) time.Duration {
	d := ttl
	if d <= 0 {
		d = time.Until(deadline)
	}
	if d <= 0 {
		d = 15 * time.Second
	}
	iv := d / 3
	if iv < 50*time.Millisecond {
		return 50 * time.Millisecond
	}
	return iv
}
