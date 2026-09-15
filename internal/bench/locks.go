package bench

import (
	"context"
	"errors"
	"fmt"
	"time"

	raftlib "github.com/hashicorp/raft"
)

func transientLeadership(err error) bool {
	return errors.Is(err, raftlib.ErrLeadershipLost) ||
		errors.Is(err, raftlib.ErrNotLeader) ||
		errors.Is(err, raftlib.ErrLeadershipTransferInProgress)
}

func measureLocks(ctx context.Context, cl *Cluster, ops int) (Result, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	lead, err := cl.WaitLeader(waitCtx)
	cancel()
	if err != nil {
		return Result{}, fmt.Errorf("locks: leader: %w", err)
	}

	const warmup = 5
	retries := 0
	samples := make([]time.Duration, 0, ops)
	for i := 0; i < ops+warmup; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if !lead.Raft.IsLeader() {
			waitCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
			lead, err = cl.WaitLeader(waitCtx)
			cancel()
			if err != nil {
				return Result{}, fmt.Errorf("locks sample %d: leader: %w", i, err)
			}
		}
		name := fmt.Sprintf("bench-%d", i)
		start := time.Now()
		tok, ok, err := lead.Raft.ApplyLockAcquire(name, "bench", time.Hour)
		elapsed := time.Since(start)
		if transientLeadership(err) {
			retries++
			if retries > 20 {
				return Result{}, fmt.Errorf("locks sample %d: acquire: %w", i, err)
			}
			waitCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
			lead, err = cl.WaitLeader(waitCtx)
			cancel()
			if err != nil {
				return Result{}, fmt.Errorf("locks sample %d: leader: %w", i, err)
			}
			i--
			continue
		}
		if err != nil {
			return Result{}, fmt.Errorf("locks sample %d: acquire: %w", i, err)
		}
		if !ok {
			return Result{}, fmt.Errorf("locks sample %d: not acquired", i)
		}
		retries = 0
		if i >= warmup {
			samples = append(samples, elapsed)
		}
		if err := lead.Raft.ApplyLockRelease(name, "bench", tok); err != nil {
			if !transientLeadership(err) {
				return Result{}, fmt.Errorf("locks sample %d: release: %w", i, err)
			}
			waitCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
			lead, err = cl.WaitLeader(waitCtx)
			cancel()
			if err != nil {
				return Result{}, fmt.Errorf("locks sample %d: leader: %w", i, err)
			}
			if err := lead.Raft.ApplyLockRelease(name, "bench", tok); err != nil {
				return Result{}, fmt.Errorf("locks sample %d: release: %w", i, err)
			}
		}
	}
	return summarize("locks", samples, TargetLockAcquire), nil
}
