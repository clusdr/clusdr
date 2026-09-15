// Command worker holds a named lease the way a shard worker would.
//
// A lease does not wait. If the name is taken, Grant fails immediately
// (unlike scheduler, which retries TryLock). Background renew keeps the
// grant until Ctrl-C. Close() revokes.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/worker -name shard-7 -owner worker-a
//	go run ./examples/worker -name shard-7 -owner worker-b
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/durguto/clusdr/sdk"
)

func main() {
	name := flag.String("name", "shard-7", "lease name")
	owner := flag.String("owner", "", "lease owner; empty generates sdk-<hex>")
	ttl := flag.Duration("ttl", 15*time.Second, "lease TTL (daemon default if <=0)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	opts := []clusdr.Option{}
	if *owner != "" {
		opts = append(opts, clusdr.WithHolder(*owner))
	}

	c, err := clusdr.Local(opts...)
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ls, err := c.Lease(ctx, *name, *ttl)
	if err != nil {
		log.Error("lease", "err", err)
		os.Exit(1)
	}
	log.Info("granted", "name", ls.Name, "owner", ls.Owner, "token", ls.Token, "until", ls.Deadline().UTC().Format(time.RFC3339))
	log.Info("holding shard (Ctrl-C to stop; Close revokes)")

	<-ctx.Done()
}
