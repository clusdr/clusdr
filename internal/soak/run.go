package soak

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config controls a soak run. Zero values become defaults.
type Config struct {
	// Duration is how long to churn. Default 24h. SIGINT/cancel ends
	// early and still audits whatever ticks completed.
	Duration time.Duration
	// Nodes is the core voter count. Must be >= 3. Default 3.
	Nodes int
	// Tick is the join/leave + lock + lease cycle. Default 2s.
	Tick time.Duration
	Dir  string
	Log  *slog.Logger
}

func (c *Config) normalize() error {
	if c.Duration <= 0 {
		c.Duration = 24 * time.Hour
	}
	if c.Nodes <= 0 {
		c.Nodes = 3
	}
	if c.Nodes < 3 {
		return fmt.Errorf("nodes must be >= 3, got %d", c.Nodes)
	}
	if c.Tick <= 0 {
		c.Tick = 2 * time.Second
	}
	return nil
}

// Run boots an in-process cluster and churns join/leave, locks, and
// leases until Duration or ctx cancel. It then checks heap, goroutines,
// and slog noise.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	if err := cfg.normalize(); err != nil {
		return nil, err
	}

	var next slog.Handler
	if cfg.Log != nil {
		next = cfg.Log.Handler()
	}
	capt := newCapture(next)
	log := capt.logger()

	dir := cfg.Dir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "clusdr-soak-")
		if err != nil {
			return nil, fmt.Errorf("temp dir: %w", err)
		}
		dir = tmp
		defer os.RemoveAll(dir)
	}

	cl, err := startCluster(dir, cfg.Nodes, log)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cl.close() }()

	expCtx, expCancel := context.WithCancel(ctx)
	defer expCancel()
	cl.startExpirers(expCtx)

	rep := &Report{Passed: true}
	started := time.Now()
	deadline := started.Add(cfg.Duration)

	// One warmup tick so Raft/Bolt caches exist before the heap baseline.
	if err := runTick(ctx, cl, dir, 0, rep); err != nil {
		return nil, err
	}

	base := snapshotMem()
	rep.HeapStart = base.Heap
	rep.GoStart = base.Goroutines

	i := 1
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			break
		}
		tickStart := time.Now()
		if err := runTick(ctx, cl, dir, i, rep); err != nil {
			if ctx.Err() != nil && i > 0 {
				break
			}
			return nil, err
		}
		i++
		sleep := cfg.Tick - time.Since(tickStart)
		if sleep <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			// Keep the last completed tick; audit what we have.
		case <-time.After(sleep):
		}
		if ctx.Err() != nil {
			break
		}
	}

	end := snapshotMem()
	rep.Duration = time.Since(started)
	rep.HeapEnd = end.Heap
	rep.GoEnd = end.Goroutines
	if end.Heap > base.Heap {
		rep.HeapGrowth = end.Heap - base.Heap
	}
	rep.Logs = capt.snapshot()

	if rep.Ticks < 1 {
		return nil, fmt.Errorf("no ticks completed")
	}
	if rep.HeapGrowth > MaxHeapGrowth {
		rep.Failures = append(rep.Failures, fmt.Sprintf("heap grew %d bytes (cap %d)", rep.HeapGrowth, MaxHeapGrowth))
	}
	if goDelta := end.Goroutines - base.Goroutines; goDelta > MaxGoroutineGrowth {
		rep.Failures = append(rep.Failures, fmt.Sprintf("goroutines grew by %d (cap %d)", goDelta, MaxGoroutineGrowth))
	}
	rep.Failures = append(rep.Failures, capt.audit(rep.Duration, rep.Ticks)...)
	if len(rep.Failures) > 0 {
		rep.Passed = false
	}
	return rep, nil
}

func runTick(ctx context.Context, cl *cluster, dir string, i int, rep *Report) error {
	opCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	id := fmt.Sprintf("extra-%d", i)
	extra, err := cl.joinExtra(opCtx, filepath.Join(dir, id), id)
	if err != nil {
		return fmt.Errorf("tick %d join: %w", i, err)
	}
	rep.Joins++
	if err := cl.leaveExtra(opCtx, extra); err != nil {
		return fmt.Errorf("tick %d leave: %w", i, err)
	}
	rep.Leaves++

	if err := churnLock(opCtx, cl, i, rep); err != nil {
		return fmt.Errorf("tick %d lock: %w", i, err)
	}
	if err := churnLease(opCtx, cl, i, rep); err != nil {
		return fmt.Errorf("tick %d lease: %w", i, err)
	}
	rep.Ticks++
	return nil
}

func churnLock(ctx context.Context, cl *cluster, i int, rep *Report) error {
	name := fmt.Sprintf("soak-lock-%d", i)
	expire := i%3 == 2
	ttl := time.Hour
	if expire {
		ttl = 150 * time.Millisecond
	}
	var tok uint64
	if err := cl.withLeader(ctx, func(lead *node) error {
		got, ok, err := lead.raft.ApplyLockAcquire(name, "soak", ttl)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("lock %s not acquired", name)
		}
		tok = got
		return nil
	}); err != nil {
		return err
	}
	rep.Locks++
	if expire {
		if err := cl.waitLockGone(ctx, name); err != nil {
			return err
		}
		rep.LockExpires++
		return nil
	}
	return cl.withLeader(ctx, func(lead *node) error {
		return lead.raft.ApplyLockRelease(name, "soak", tok)
	})
}

func churnLease(ctx context.Context, cl *cluster, i int, rep *Report) error {
	name := fmt.Sprintf("soak-lease-%d", i)
	expire := i%3 == 2
	ttl := time.Hour
	if expire {
		ttl = 150 * time.Millisecond
	}
	var tok uint64
	if err := cl.withLeader(ctx, func(lead *node) error {
		got, ok, err := lead.raft.ApplyLeaseGrant(name, "soak", ttl)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("lease %s not granted", name)
		}
		tok = got
		return nil
	}); err != nil {
		return err
	}
	rep.Leases++
	if expire {
		if err := cl.waitLeaseGone(ctx, name); err != nil {
			return err
		}
		rep.LeaseExpires++
		return nil
	}
	return cl.withLeader(ctx, func(lead *node) error {
		return lead.raft.ApplyLeaseRevoke(name, "soak", tok)
	})
}
