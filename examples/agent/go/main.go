// Command agent publishes and/or listens for custom.agent.task.
//
// Gossip through the local daemon. Not a queue. A later subscriber does not
// see old custom events.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/agent/go -mode listen
//	go run ./examples/agent/go -mode emit -from mapper
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

const topic = "agent.task"

func main() {
	mode := flag.String("mode", "both", "listen, emit, or both")
	source := flag.String("from", fmt.Sprintf("pid-%d", os.Getpid()), "payload from= field when emitting")
	every := flag.Duration("every", 2*time.Second, "emit interval")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	switch *mode {
	case "listen", "emit", "both":
	default:
		log.Error("mode must be listen, emit, or both")
		os.Exit(2)
	}

	c, err := clusdr.Local()
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *mode == "emit" || *mode == "both" {
		go emitLoop(ctx, c, log, *source, *every)
		fmt.Printf("emitting custom.%s every %s from=%s\n", topic, every.String(), *source)
	}

	if *mode == "listen" || *mode == "both" {
		fmt.Printf("listening for custom.%s (Ctrl-C to stop)\n", topic)
		ch, err := c.Watch(ctx, clusdr.WithTopics(topic))
		if err != nil {
			log.Error("watch", "err", err)
			os.Exit(1)
		}
		for ev := range ch {
			body := ""
			if len(ev.Payload) > 0 {
				body = string(ev.Payload)
			}
			ts := ""
			if !ev.Timestamp.IsZero() {
				ts = ev.Timestamp.UTC().Format("15:04:05")
			}
			fmt.Printf("%s seq=%d %s src=%s %s\n", ts, ev.Seq, ev.Type, ev.Source, body)
		}
		return
	}

	fmt.Fprintln(os.Stderr, "emitting (Ctrl-C to stop)")
	<-ctx.Done()
}

func emitLoop(ctx context.Context, c clusdr.Cluster, log *slog.Logger, source string, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n++
			payload, err := json.Marshal(map[string]any{
				"from": source,
				"n":    n,
				"pid":  os.Getpid(),
			})
			if err != nil {
				log.Error("payload", "err", err)
				return
			}
			if err := c.Publish(ctx, topic, payload); err != nil {
				log.Error("publish", "err", err)
				return
			}
			fmt.Printf("sent n=%d from=%s\n", n, source)
		}
	}
}
