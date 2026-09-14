// Command clusdr-bench spins an in-process cluster and measures
// election, event fanout, member-list, and lock-acquire latency.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/odurgut/clusdr/internal/bench"
)

func main() {
	if err := newCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd() *cobra.Command {
	var (
		nodes      int
		eventNodes int
		ops        int
		scenarios  string
		asJSON     bool
	)

	cmd := &cobra.Command{
		Use:   "clusdr-bench",
		Short: "Synthetic cluster load generator",
		Long: `Spin an in-process Clusdr cluster and measure:

  election   leader failover (target p95 < 500ms)
  events     custom-event fanout (target p95 < 100ms at 25 nodes)
  members    ListMembers RPC latency
  locks      Raft lock acquire (target p95 < 50ms)

The process is a load generator, not a cluster member. SIGINT/SIGTERM
stop the run and tear the in-process nodes down.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			list := splitCSV(scenarios)
			cfg := bench.Config{
				Nodes:       nodes,
				EventNodes:  eventNodes,
				ElectionOps: ops,
				EventOps:    ops,
				LockOps:     ops,
				MemberOps:   ops,
				Scenarios:   list,
				Log:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
			}

			rep, err := bench.Run(ctx, cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			} else if err := rep.WriteText(out); err != nil {
				return err
			}
			if !rep.Passed {
				return fmt.Errorf("one or more scenarios missed their target")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&nodes, "nodes", 3, "Raft voters for election and lock scenarios")
	cmd.Flags().IntVar(&eventNodes, "event-nodes", 25, "event-mesh size for fanout and member-list size")
	cmd.Flags().IntVar(&ops, "ops", 10, "samples per scenario")
	cmd.Flags().StringVar(&scenarios, "scenario", "all", "comma-separated: all,election,events,members,locks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "write the report as JSON")
	return cmd
}

func splitCSV(s string) []string {
	if s == "" || s == "all" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
