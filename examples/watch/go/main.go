// Command watch prints a membership snapshot, holds a worker lease, publishes
// custom.hello, then follows the full Watch bus.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/watch/go
//	go run ./examples/watch/go -name edge-1
//
// In another terminal: clusdr publish ping '{"from":"cli"}'
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	name := flag.String("name", fmt.Sprintf("pid-%d", os.Getpid()), "lease name suffix (worker.<name>)")
	once := flag.Bool("once", false, "print members and the leader, then exit")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	c, err := clusdr.Local()
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := snapshot(ctx, c); err != nil {
		log.Error("snapshot", "err", err)
		os.Exit(1)
	}
	if *once {
		return
	}

	leaseName := "worker." + *name
	ls, err := c.Lease(ctx, leaseName, 15*time.Second)
	if err != nil {
		log.Error("lease", "err", err)
		os.Exit(1)
	}
	until := "?"
	if d := ls.Deadline(); !d.IsZero() {
		until = d.UTC().Format(time.RFC3339)
	}
	fmt.Printf("lease %s owner=%s token=%d until %s\n", ls.Name, ls.Owner, ls.Token, until)

	payload, err := json.Marshal(map[string]any{
		"worker": *name,
		"pid":    os.Getpid(),
		"lease":  leaseName,
	})
	if err != nil {
		log.Error("payload", "err", err)
		os.Exit(1)
	}
	if err := c.Publish(ctx, "hello", payload); err != nil {
		log.Error("publish", "err", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "watching (Ctrl-C to stop); try: clusdr publish ping '{\"from\":\"cli\"}'")
	ch, err := c.Watch(ctx)
	if err != nil {
		log.Error("watch", "err", err)
		os.Exit(1)
	}
	for ev := range ch {
		printEvent(ev)
	}
}

func snapshot(ctx context.Context, c clusdr.Cluster) error {
	q, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	members, err := c.Members(q)
	if err != nil {
		return err
	}
	fmt.Printf("%-24s %-22s %-8s %-10s LEADER\n", "ID", "ADDRESS", "STATUS", "ROLE")
	for _, m := range members {
		role := m.Role
		if role == "" {
			role = "voter"
		}
		fmt.Printf("%-24s %-22s %-8s %-10s %v\n", m.ID, m.Address, m.Status, role, m.Leader)
	}
	leader, err := c.Leader(q)
	if err != nil {
		return fmt.Errorf("leader: %w", err)
	}
	fmt.Printf("leader %s at %s\n", leader.ID, leader.Address)
	return nil
}

func printEvent(ev clusdr.Event) {
	kind := "bus"
	switch {
	case ev.Type == "member.join", ev.Type == "leader.changed":
		kind = "cluster"
	case ev.Type == "member.dead":
		kind = "dead"
	case ev.Type == "member.left":
		kind = "left"
	case strings.HasPrefix(ev.Type, "custom."):
		kind = "gossip"
	case ev.Type == "watch.sync", ev.Type == "watch.gap":
		kind = "watch"
	}
	ts := ""
	if !ev.Timestamp.IsZero() {
		ts = ev.Timestamp.UTC().Format("15:04:05")
	}
	payload := ""
	if len(ev.Payload) > 0 {
		payload = " " + string(ev.Payload)
	}
	fmt.Printf("%s %s seq=%d %s src=%s%s\n", kind, ts, ev.Seq, ev.Type, ev.Source, payload)
}
