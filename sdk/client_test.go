package clusdr

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDial_EmptyAddr(t *testing.T) {
	if _, err := Dial(""); err == nil {
		t.Fatal("expected empty address error")
	}
}

func TestDial_LazyNoReadyWait(t *testing.T) {
	t.Setenv("CLUSDR_TLS", "disabled")
	o := defaultOptions()
	o.addr = "127.0.0.1:1"
	o.readyTimeout = 0
	c, err := dial(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClose_NilConn(t *testing.T) {
	c := &client{}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWithRPC(t *testing.T) {
	c := &client{opts: options{requestTimeout: time.Second}}
	ctx, cancel := c.withRPC(context.Background())
	cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected timeout")
	}

	dead, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	got, done := c.withRPC(dead)
	done()
	if got != dead {
		t.Fatal("caller deadline must win")
	}
}

func TestNewHolderID(t *testing.T) {
	id, err := newHolderID()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "sdk-") || len(id) < 8 {
		t.Fatalf("id %q", id)
	}
}

func TestNextBackoffAndSleep(t *testing.T) {
	if got := nextBackoff(50 * time.Millisecond); got != 100*time.Millisecond {
		t.Fatalf("got %s", got)
	}
	if got := nextBackoff(2 * time.Second); got != 2*time.Second {
		t.Fatalf("cap %s", got)
	}
	if !sleepBackoff(context.Background(), time.Millisecond) {
		t.Fatal("timer should fire")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepBackoff(ctx, time.Second) {
		t.Fatal("cancelled context")
	}
}

func TestWatch_InvalidTopic(t *testing.T) {
	c := &client{}
	if _, err := c.Watch(context.Background(), WithTopics("bad topic")); err == nil {
		t.Fatal("expected error")
	}
}

func TestUnlockRenewRevoke_NotHeld(t *testing.T) {
	c := &client{
		opts:   defaultOptions(),
		held:   map[string]*Lock{},
		leased: map[string]*Lease{},
	}
	ctx := context.Background()
	if err := c.Unlock(ctx, "k"); err == nil {
		t.Fatal("unlock")
	}
	if err := c.Renew(ctx, "k"); err == nil {
		t.Fatal("renew")
	}
	if err := c.Revoke(ctx, "k"); err == nil {
		t.Fatal("revoke")
	}
}
