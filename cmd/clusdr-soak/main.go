// Command clusdr-soak spins an in-process cluster and churns
// join/leave, locks, and leases for a long run. It then checks
// heap growth, goroutine growth, and slog noise.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/clusdr/clusdr/internal/soak"
)

func main() {
	if err := newCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd() *cobra.Command {
	var (
		nodes    int
		duration time.Duration
		tick     time.Duration
		asJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "clusdr-soak",
		Short: "Long-running in-process stability run",
		Long: `Spin an in-process 3-voter Clusdr cluster and, for --duration:

  - join a transient voter, then clusdr-leave it (RemoveServer + drop)
  - acquire and release locks; let some expire
  - grant and revoke leases; let some expire

Then check heap and goroutine growth, and audit slog for errors and
unexpected warn spam.

The process is a load generator, not a cluster member. SIGINT/SIGTERM
stop the run, print the report so far, and tear the nodes down. Exit 1
if a check misses. Not a release artifact.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			cfg := soak.Config{
				Nodes:    nodes,
				Duration: duration,
				Tick:     tick,
				Log:      slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
			}

			rep, err := soak.Run(ctx, cfg)
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
				return fmt.Errorf("soak checks missed")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&nodes, "nodes", 3, "Raft voters (core set; extras join and leave)")
	cmd.Flags().DurationVar(&duration, "duration", 24*time.Hour, "how long to churn")
	cmd.Flags().DurationVar(&tick, "tick", 2*time.Second, "join/leave + lock + lease cycle")
	cmd.Flags().BoolVar(&asJSON, "json", false, "write the report as JSON")
	return cmd
}
