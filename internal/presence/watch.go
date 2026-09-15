package presence

import (
	"context"

	"github.com/durguto/clusdr/internal/eventbus"
)

// WatchExpired reads the bus and calls OnExpired for each event until ctx is done.
func WatchExpired(ctx context.Context, sub *eventbus.Subscription, selfID string, isLeader func() bool, remove func(id string)) {
	if sub == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Done:
			return
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			OnExpired(e, selfID, isLeader != nil && isLeader(), remove)
		}
	}
}
