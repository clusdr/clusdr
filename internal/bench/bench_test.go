package bench_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/bench"
)

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

func TestRun_MeetsTargets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	rep, err := bench.Run(ctx, bench.Config{
		Nodes:       3,
		EventNodes:  25,
		ElectionOps: 3,
		EventOps:    5,
		LockOps:     20,
		MemberOps:   50,
		Dir:         t.TempDir(),
		Log:         discard(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Log(rep.String())
	got := map[string]bench.Result{}
	for _, r := range rep.Results {
		got[r.Name] = r
		if r.N == 0 {
			t.Errorf("%s: no samples", r.Name)
		}
	}
	for _, name := range []string{"election", "events", "members", "locks"} {
		if _, ok := got[name]; !ok {
			t.Errorf("missing scenario %s", name)
		}
	}
	// Run already failed if a majority did not elect. These gates are latency.
	// raceSlack is 1 without -race and 4 with it (detector tax, not a skip).
	if e := got["election"]; e.P50 > time.Duration(raceSlack)*bench.TargetElection {
		t.Errorf("election p50 %s > %s", e.P50, time.Duration(raceSlack)*bench.TargetElection)
	}
	if e := got["election"]; e.P95 > time.Duration(raceSlack)*2*bench.TargetElection {
		t.Errorf("election p95 %s > %s", e.P95, time.Duration(raceSlack)*2*bench.TargetElection)
	}
	if e := got["events"]; e.P95 > bench.TargetEventFanout {
		t.Errorf("events p95 %s > %s", e.P95, bench.TargetEventFanout)
	}
	if e := got["locks"]; e.P50 > time.Duration(raceSlack)*bench.TargetLockAcquire {
		t.Errorf("locks p50 %s > %s", e.P50, time.Duration(raceSlack)*bench.TargetLockAcquire)
	}
	if e := got["locks"]; e.P95 > time.Duration(raceSlack)*2*bench.TargetLockAcquire {
		t.Errorf("locks p95 %s > %s", e.P95, time.Duration(raceSlack)*2*bench.TargetLockAcquire)
	}
	if e := got["members"]; e.N != 50 {
		t.Errorf("members n=%d want 50", e.N)
	}
}

func TestRun_ScenarioFilter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rep, err := bench.Run(ctx, bench.Config{
		EventNodes: 8,
		MemberOps:  10,
		Scenarios:  []string{"members"},
		Log:        discard(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 1 || rep.Results[0].Name != "members" {
		t.Fatalf("results: %+v", rep.Results)
	}
}
