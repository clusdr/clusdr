package bench

import (
	"context"
	"fmt"
	"time"
)

func measureElection(ctx context.Context, cl *Cluster, ops int) (Result, error) {
	samples := make([]time.Duration, 0, ops)
	for i := 0; i < ops; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		lead, err := cl.WaitLeader(waitCtx)
		cancel()
		if err != nil {
			return Result{}, fmt.Errorf("election sample %d: stable leader: %w", i, err)
		}

		majority := cl.except(lead.ID)
		start := time.Now()
		if err := cl.Isolate(lead.ID); err != nil {
			return Result{}, err
		}
		waitCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		_, err = waitFirstLeader(waitCtx, majority...)
		elapsed := time.Since(start)
		cancel()
		healErr := cl.Heal(lead.ID)
		if err != nil {
			return Result{}, fmt.Errorf("election sample %d: majority did not elect: %w", i, err)
		}
		if healErr != nil {
			return Result{}, healErr
		}
		samples = append(samples, elapsed)

		waitCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		_, err = cl.WaitLeader(waitCtx)
		cancel()
		if err != nil {
			return Result{}, fmt.Errorf("election sample %d: cluster did not heal: %w", i, err)
		}
	}
	return summarize("election", samples, TargetElection), nil
}
