// Command who prints cluster membership and then follows Watch.
//
// The process is an application. It does not vote. It talks only to the
// daemon on this host (CLUSDR_GRPC_ADDR or 127.0.0.1:7947).
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/who/go
//	go run ./examples/who/go -once
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
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

	fmt.Fprintln(os.Stderr, "watching cluster events (Ctrl-C to stop)")
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
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tROLE\tLEADER")
	for _, m := range members {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n", m.ID, m.Address, m.Status, role(m), m.Leader)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	leader, err := c.Leader(q)
	if err != nil {
		return fmt.Errorf("leader: %w", err)
	}
	fmt.Printf("leader %s at %s\n", leader.ID, leader.Address)
	return nil
}

func role(m clusdr.Member) string {
	if m.Role == "" {
		return "voter"
	}
	return m.Role
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
	payload := ""
	if len(ev.Payload) > 0 {
		payload = " " + string(ev.Payload)
	}
	fmt.Printf("%s seq=%d %s src=%s%s\n", kind, ev.Seq, ev.Type, ev.Source, payload)
}
