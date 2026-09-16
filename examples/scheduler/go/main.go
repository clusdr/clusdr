// Command scheduler holds one exclusive cluster lock and “dispatches” work.
//
// Run two copies with different -holder values. Only one replica holds
// "scheduler" at a time. The fencing token (lk.Token) is what you would
// store next to a fenced write; a stale holder cannot unlock a newer grant.
//
// The app still does not join the cluster. Close() unlocks.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/scheduler/go -holder replica-a
//	go run ./examples/scheduler/go -holder replica-b
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/durguto/clusdr/sdk"
)

func main() {
	name := flag.String("name", "scheduler", "lock name (1–128, A–Z a–z 0–9 . _ -)")
	holder := flag.String("holder", "", "lock identity; empty generates sdk-<hex>")
	work := flag.Duration("work", 3*time.Second, "how long to hold the lock while dispatching")
	wait := flag.Duration("wait", time.Second, "pause after a failed try or a completed shift")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	opts := []clusdr.Option{}
	if *holder != "" {
		opts = append(opts, clusdr.WithHolder(*holder))
	}

	c, err := clusdr.Local(opts...)
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("scheduler replica", "lock", *name, "work", work.String())
	for ctx.Err() == nil {
		if err := shift(ctx, c, log, *name, *work, *wait); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("shift", "err", err)
			os.Exit(1)
		}
	}
}

func shift(ctx context.Context, c clusdr.Cluster, log *slog.Logger, name string, work, wait time.Duration) error {
	try, cancel := context.WithTimeout(ctx, 5*time.Second)
	lk, ok, err := c.TryLock(try, name, 15*time.Second)
	cancel()
	if err != nil {
		return err
	}
	if !ok {
		who := "another replica"
		tok := uint64(0)
		if lk != nil {
			who = lk.Holder
			tok = lk.Token
		}
		log.Info("waiting", "held_by", who, "token", tok)
		return sleep(ctx, wait)
	}

	log.Info("held", "name", lk.Name, "holder", lk.Holder, "token", lk.Token, "until", lk.Deadline().UTC().Format(time.RFC3339))
	dispatch(lk)
	if err := sleep(ctx, work); err != nil {
		return c.Unlock(context.Background(), name)
	}
	if err := c.Unlock(ctx, name); err != nil {
		return err
	}
	log.Info("released", "name", name)
	return sleep(ctx, wait)
}

func dispatch(lk *clusdr.Lock) {
	// Pretend to write through a store that checks the fencing token.
	fmt.Printf("dispatch job=rollout fence=%d holder=%s\n", lk.Token, lk.Holder)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
