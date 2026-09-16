package app_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/app"
	"github.com/clusdr/clusdr/internal/eventbus"
	"github.com/clusdr/clusdr/internal/events"
)

func nopOpts() app.Options {
	return app.Options{
		LogOutput: io.Discard,
		Getenv:    func(string) string { return "" },
	}
}

func TestInitApp_StartsInOrder(t *testing.T) {
	var order []string

	units := []app.Unit{
		{Name: "alpha", InitFn: func(_ context.Context, _ *app.App) error { order = append(order, "alpha"); return nil }},
		{Name: "beta", InitFn: func(_ context.Context, _ *app.App) error { order = append(order, "beta"); return nil }},
		{Name: "gamma", InitFn: func(_ context.Context, _ *app.App) error { order = append(order, "gamma"); return nil }},
	}

	a, err := app.InitApp(context.Background(), units, nopOpts())
	if err != nil {
		t.Fatalf("InitApp: %v", err)
	}
	a.Stop()
	_ = a.Wait()

	want := []string{"alpha", "beta", "gamma"}
	if len(order) != len(want) {
		t.Fatalf("order: got %v, want %v", order, want)
	}
	for i, v := range want {
		if order[i] != v {
			t.Errorf("step %d: got %q, want %q", i, order[i], v)
		}
	}
}

func TestInitApp_ClosesInReverseOrder(t *testing.T) {
	var order []string

	closeFn := func(name string) app.Fn {
		return func(_ context.Context, _ *app.App) error {
			order = append(order, "close:"+name)
			return nil
		}
	}

	units := []app.Unit{
		{Name: "alpha", InitFn: func(_ context.Context, _ *app.App) error { return nil }, CloseFn: closeFn("alpha")},
		{Name: "beta", InitFn: func(_ context.Context, _ *app.App) error { return nil }, CloseFn: closeFn("beta")},
		{Name: "gamma", InitFn: func(_ context.Context, _ *app.App) error { return nil }, CloseFn: closeFn("gamma")},
	}

	a, err := app.InitApp(context.Background(), units, nopOpts())
	if err != nil {
		t.Fatalf("InitApp: %v", err)
	}
	a.Stop()
	_ = a.Wait()

	want := []string{"close:gamma", "close:beta", "close:alpha"}
	if len(order) != len(want) {
		t.Fatalf("order: got %v, want %v", order, want)
	}
	for i, v := range want {
		if order[i] != v {
			t.Errorf("step %d: got %q, want %q", i, order[i], v)
		}
	}
}

func TestInitApp_FailureClosesStartedUnits(t *testing.T) {
	var closed []string
	errBoom := errors.New("boom")

	units := []app.Unit{
		{
			Name:    "first",
			InitFn:  func(_ context.Context, _ *app.App) error { return nil },
			CloseFn: func(_ context.Context, _ *app.App) error { closed = append(closed, "first"); return nil },
		},
		{
			Name:   "second",
			InitFn: func(_ context.Context, _ *app.App) error { return errBoom },
		},
		{
			// third should never be reached
			Name:    "third",
			InitFn:  func(_ context.Context, _ *app.App) error { return nil },
			CloseFn: func(_ context.Context, _ *app.App) error { closed = append(closed, "third"); return nil },
		},
	}

	_, err := app.InitApp(context.Background(), units, nopOpts())
	if !errors.Is(err, errBoom) {
		t.Fatalf("got %v, want %v", err, errBoom)
	}
	if len(closed) != 1 || closed[0] != "first" {
		t.Errorf("only first should close on failure; got %v", closed)
	}
}

func TestInitApp_CollaboratorSharing(t *testing.T) {
	// Verify that InitFn can read values written by a previous InitFn via *App.
	units := []app.Unit{
		{
			Name: "writer",
			InitFn: func(_ context.Context, a *app.App) error {
				a.Cfg.Node.ID = "test-node"
				return nil
			},
		},
		{
			Name: "reader",
			InitFn: func(_ context.Context, a *app.App) error {
				if a.Cfg.Node.ID != "test-node" {
					return errors.New("node id not propagated")
				}
				return nil
			},
		},
	}

	a, err := app.InitApp(context.Background(), units, nopOpts())
	if err != nil {
		t.Fatalf("InitApp: %v", err)
	}
	a.Stop()
	_ = a.Wait()
}

func TestInitApp_NilInitFnIsError(t *testing.T) {
	units := []app.Unit{{Name: "bad"}} // nil InitFn

	_, err := app.InitApp(context.Background(), units, nopOpts())
	if err == nil {
		t.Fatal("expected error for nil InitFn")
	}
}

// TestBus_MembershipEventsFlow verifies membership events reach the bus:
// when InitBus wires the event bus into a membership.Engine, Join and Leave
// calls publish events to all subscribers without blocking.
func TestBus_MembershipEventsFlow(t *testing.T) {
	// Build a minimal app with just membership + bus units.
	a, err := app.InitApp(context.Background(),
		[]app.Unit{app.UnitLogger, app.UnitMembership, app.UnitBus},
		nopOpts())
	if err != nil {
		t.Fatalf("InitApp: %v", err)
	}
	defer func() { a.Stop(); _ = a.Wait() }()

	sub := a.Bus.Subscribe(16)
	defer sub.Unsubscribe()

	// Trigger a join.
	if _, err := a.Membership.Join("node-1", "127.0.0.1:8001"); err != nil {
		t.Fatalf("Join: %v", err)
	}

	// Expect member.join event.
	select {
	case e := <-sub.C:
		if e.Type != events.TypeMemberJoin {
			t.Errorf("type: got %q, want %q", e.Type, events.TypeMemberJoin)
		}
		if e.Source != "node-1" {
			t.Errorf("source: got %q, want node-1", e.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("no member.join event within 1s")
	}

	// Trigger a leave.
	a.Membership.Leave("node-1")

	select {
	case e := <-sub.C:
		if e.Type != events.TypeMemberLeft {
			t.Errorf("type: got %q, want %q", e.Type, events.TypeMemberLeft)
		}
		if e.Source != "node-1" {
			t.Errorf("source: got %q, want node-1", e.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("no member.left event within 1s")
	}

	// Trigger a leader change.
	a.Membership.SetLeader("node-2")

	select {
	case e := <-sub.C:
		if e.Type != events.TypeLeaderChanged {
			t.Errorf("type: got %q, want %q", e.Type, events.TypeLeaderChanged)
		}
		if e.Source != "node-2" {
			t.Errorf("source: got %q, want node-2", e.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("no leader.changed event within 1s")
	}
}

// TestBus_SlowWatcherDoesNotBlockCluster verifies backpressure: the bus (and
// thus the cluster) does not slow down when a subscriber is lagging.
func TestBus_SlowWatcherDoesNotBlockCluster(t *testing.T) {
	b := eventbus.New()

	// Subscriber with a buffer of 1 — will overflow quickly.
	slow := b.Subscribe(1)
	defer slow.Unsubscribe()

	const N = 200
	start := time.Now()
	for i := 0; i < N; i++ {
		b.Publish(events.Event{Type: events.TypeMemberJoin, Source: "x"})
	}
	elapsed := time.Since(start)

	// All 200 publishes must complete in well under 1s.
	if elapsed > time.Second {
		t.Errorf("Publish took %v for %d events (slow subscriber blocked?)", elapsed, N)
	}
}

func TestApp_StopIsIdempotent(t *testing.T) {
	a, err := app.InitApp(context.Background(), []app.Unit{
		{Name: "noop", InitFn: func(_ context.Context, _ *app.App) error { return nil }},
	}, nopOpts())
	if err != nil {
		t.Fatalf("InitApp: %v", err)
	}

	a.Stop()
	a.Stop() // must not panic
	_ = a.Wait()
}
