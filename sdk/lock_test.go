package clusdr

import (
	"context"
	"testing"
	"time"
)

type grantStub struct {
	holder string
	token  uint64
	dead   int64
}

func (g grantStub) GetHolder() string        { return g.holder }
func (g grantStub) GetFencingToken() uint64  { return g.token }
func (g grantStub) GetDeadlineUnixMs() int64 { return g.dead }

func TestTTLMs(t *testing.T) {
	if ttlMs(0) != 0 || ttlMs(-time.Second) != 0 {
		t.Fatal("non-positive")
	}
	if ttlMs(1500*time.Millisecond) != 1500 {
		t.Fatal("ms")
	}
}

func TestRenewInterval(t *testing.T) {
	if got := renewInterval(time.Time{}, 0); got != 5*time.Second {
		t.Fatalf("default %s", got)
	}
	if got := renewInterval(time.Time{}, 90*time.Millisecond); got != 50*time.Millisecond {
		t.Fatalf("floor %s", got)
	}
	if got := renewInterval(time.Now().Add(30*time.Second), 15*time.Second); got != 5*time.Second {
		t.Fatalf("ttl/3 %s", got)
	}
}

func TestLockFromResp(t *testing.T) {
	l := lockFromResp(grantStub{}, "alpha")
	if l.Name != "alpha" || l.Holder != "" || !l.deadline.IsZero() {
		t.Fatalf("%+v", l)
	}
	l = lockFromResp(grantStub{holder: "app", token: 7, dead: 1_700_000_000_000}, "alpha")
	if l.Holder != "app" || l.Token != 7 || l.deadline.UnixMilli() != 1_700_000_000_000 {
		t.Fatalf("%+v", l)
	}
}

func TestHeldLockForget(t *testing.T) {
	c := &client{held: map[string]*Lock{}}
	if c.heldLock("k") != nil {
		t.Fatal("empty")
	}
	lk := &Lock{Name: "k", Token: 3}
	c.held["k"] = lk
	if c.heldLock("k") != lk {
		t.Fatal("cached")
	}
	c.forget("k", 99)
	if c.heldLock("k") == nil {
		t.Fatal("wrong token must keep")
	}
	c.forget("k", 3)
	if c.heldLock("k") != nil {
		t.Fatal("forgot")
	}
}

func TestLockDeadlineAndStopRenew(t *testing.T) {
	if !(*Lock)(nil).Deadline().IsZero() {
		t.Fatal("nil lock")
	}
	(*Lock)(nil).stopRenew()
	ctx, cancel := context.WithCancel(context.Background())
	l := &Lock{stop: cancel, deadline: time.Unix(10, 0)}
	if !l.Deadline().Equal(time.Unix(10, 0)) {
		t.Fatal("deadline")
	}
	l.stopRenew()
	<-ctx.Done()
	l.stopRenew()
}

func TestLockRelease_Invalid(t *testing.T) {
	if err := (*Lock)(nil).release(context.Background()); err == nil {
		t.Fatal("nil")
	}
	if err := (&Lock{}).release(context.Background()); err == nil {
		t.Fatal("no client")
	}
}

func TestLeaseFromRespAndHeld(t *testing.T) {
	if l := leaseFromResp(nil, "n"); l.Name != "n" {
		t.Fatal("nil resp")
	}
	c := &client{leased: map[string]*Lease{}}
	if c.heldLease("n") != nil {
		t.Fatal("empty")
	}
	ls := &Lease{Name: "n", Token: 2}
	c.leased["n"] = ls
	if c.heldLease("n") != ls {
		t.Fatal("cached")
	}
	c.forgetLease("n", 1)
	if c.heldLease("n") == nil {
		t.Fatal("wrong token")
	}
	c.forgetLease("n", 2)
	if c.heldLease("n") != nil {
		t.Fatal("forgot")
	}
}

func TestLeaseDeadlineAndStopRenew(t *testing.T) {
	if !(*Lease)(nil).Deadline().IsZero() {
		t.Fatal("nil lease")
	}
	(*Lease)(nil).stopRenew()
	ctx, cancel := context.WithCancel(context.Background())
	l := &Lease{stop: cancel}
	l.stopRenew()
	<-ctx.Done()
}

func TestLeaseDrop_Invalid(t *testing.T) {
	if err := (*Lease)(nil).drop(context.Background()); err == nil {
		t.Fatal("nil")
	}
	if err := (&Lease{}).drop(context.Background()); err == nil {
		t.Fatal("no client")
	}
}

func TestReleaseHeldAndLeases_Empty(t *testing.T) {
	c := &client{
		opts:   options{requestTimeout: time.Second},
		held:   map[string]*Lock{},
		leased: map[string]*Lease{},
	}
	c.releaseHeld()
	c.releaseLeases()
}
