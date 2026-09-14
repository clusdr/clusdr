package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/odurgut/clusdr/internal/app"
)

func newStartCmd(f *rootFlags) *cobra.Command {
	var bootstrap bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Clusdr daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Build a getenv that forwards CLUSDR_RAFT_BOOTSTRAP when --bootstrap
			// is set on the CLI, without mutating the real environment.
			getenv := os.Getenv
			if bootstrap {
				getenv = func(key string) string {
					if key == "CLUSDR_RAFT_BOOTSTRAP" {
						return "true"
					}
					return os.Getenv(key)
				}
			}

			a, err := app.InitApp(context.Background(), app.DefaultUnits(), app.Options{
				ConfigPath: f.configPath,
				Getenv:     getenv,
				LogOutput:  os.Stderr,
			})
			if err != nil {
				// Log to stderr before the structured logger is ready.
				slog.Error("failed to start", "err", err)
				return err
			}

			go a.WatchShutdown()
			return a.Wait()
		},
	}

	cmd.Flags().BoolVar(&bootstrap, "bootstrap", false,
		"bootstrap this node as the first Raft member (seed node only)")
	return cmd
}
