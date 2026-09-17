package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/clusdr/clusdr/internal/config"
)

// rootFlags holds persistent flags available to all subcommands.
type rootFlags struct {
	configPath string
	logLevel   string
	logFormat  string
}

func newRootCmd() *cobra.Command {
	f := &rootFlags{}

	cmd := &cobra.Command{
		Use:   "clusdr",
		Short: "Distributed runtime for cluster awareness",
		Long: `Clusdr gives applications cluster awareness out of the box.

A single binary that provides membership, leadership, presence, and
real-time events — without etcd, Consul, ZooKeeper, or custom heartbeats.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&f.configPath, "config", "clusdr.yaml", "config file path")
	cmd.PersistentFlags().StringVar(&f.logLevel, "log-level", "", "log level override (debug|info|warn|error)")
	cmd.PersistentFlags().StringVar(&f.logFormat, "log-format", "", "log format override (text|json)")

	cmd.AddCommand(
		newVersionCmd(),
		newConfigCmd(f),
		newInitCmd(f),
		newStartCmd(f),
		newStatusCmd(f),
		newHealthCmd(f),
		newJoinCmd(f),
		newPromoteCmd(f),
		newLeaveCmd(f),
		newMembersCmd(f),
		newLeaderCmd(f),
		newWatchCmd(f),
		newPublishCmd(f),
		newCertsCmd(f),
		newLocksCmd(f),
		newLeasesCmd(f),
	)

	return cmd
}

// loadConfig loads configuration respecting flags and environment.
func loadConfig(f *rootFlags) (config.Config, error) {
	cfg, err := config.LoadFrom(f.configPath, os.Getenv)
	if err != nil {
		return config.Config{}, err
	}
	// Flag overrides take precedence over env and YAML.
	if f.logLevel != "" {
		cfg.Log.Level = f.logLevel
	}
	if f.logFormat != "" {
		cfg.Log.Format = f.logFormat
	}
	return cfg, nil
}
