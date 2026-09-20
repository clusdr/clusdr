package soak_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/clusdr/clusdr/internal/soak"
)

func TestRun_Normalize(t *testing.T) {
	_, err := soak.Run(context.Background(), soak.Config{Nodes: 2, Duration: time.Second})
	if err == nil {
		t.Fatal("expected nodes < 3 to fail")
	}
}

func TestRun_ShortStability(t *testing.T) {
	dur := 4 * time.Second
	if s := os.Getenv("CLUSDR_SOAK_DURATION"); s != "" {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("CLUSDR_SOAK_DURATION: %v", err)
		}
		dur = parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur+60*time.Second)
	defer cancel()

	rep, err := soak.Run(ctx, soak.Config{
		Duration: dur,
		Nodes:    3,
		Tick:     400 * time.Millisecond,
		Dir:      t.TempDir(),
		Log:      slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 10})),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Log(rep.String())
	if !rep.Passed {
		t.Fatalf("soak failed: %v", rep.Failures)
	}
	if rep.Ticks < 3 {
		t.Fatalf("ticks=%d want >= 3", rep.Ticks)
	}
	if rep.Joins != rep.Leaves || rep.Joins != rep.Ticks {
		t.Fatalf("join/leave/ticks = %d/%d/%d", rep.Joins, rep.Leaves, rep.Ticks)
	}
	if rep.Locks < 3 || rep.Leases < 3 {
		t.Fatalf("locks=%d leases=%d", rep.Locks, rep.Leases)
	}
	if dur >= 1200*time.Millisecond && (rep.LockExpires == 0 || rep.LeaseExpires == 0) {
		t.Fatalf("expected at least one lock and lease expiry; lock_expires=%d lease_expires=%d",
			rep.LockExpires, rep.LeaseExpires)
	}
	if rep.Logs.Error != 0 {
		t.Fatalf("error logs: %d", rep.Logs.Error)
	}
}
