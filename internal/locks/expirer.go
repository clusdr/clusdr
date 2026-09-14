package locks

import (
	"context"
	"log/slog"
	"time"
)

// RunExpirer scans the table on interval and commits expire for due locks.
// Only the leader should apply; isLeader is checked each tick.
// expire is typically consensus.Node.ApplyLockExpire so followers apply via Raft.
func RunExpirer(
	ctx context.Context,
	tab *Table,
	isLeader func() bool,
	expire func(name string, token uint64) error,
	interval time.Duration,
	log *slog.Logger,
) {
	if tab == nil || expire == nil {
		return
	}
	if interval <= 0 {
		interval = DefaultExpireInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if isLeader != nil && !isLeader() {
				continue
			}
			for _, rec := range tab.Due(time.Now()) {
				if err := expire(rec.Name, rec.Token); err != nil && log != nil {
					log.Warn("lock expire apply", "name", rec.Name, "token", rec.Token, "err", err)
				}
			}
		}
	}
}
