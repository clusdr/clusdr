package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newConfigCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Clusdr configuration",
	}
	cmd.AddCommand(newConfigValidateCmd(f))
	return cmd
}

func newConfigValidateCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate config and print resolved values",
		Long: `Load the config file and environment variables, then print
the fully resolved configuration. Exits 0 on success.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

			fmt.Fprintln(w, "# resolved configuration")
			fmt.Fprintln(w)

			fmt.Fprintln(w, "node:")
			fmt.Fprintf(w, "  id\t%s\n", strOrEmpty(cfg.Node.ID, "(not set — generated on init)"))
			fmt.Fprintf(w, "  addr\t%s\n", cfg.Node.Addr)

			fmt.Fprintln(w, "cluster:")
			fmt.Fprintf(w, "  id\t%s\n", strOrEmpty(cfg.Cluster.ID, "(not set — generated on init)"))

			fmt.Fprintln(w, "data:")
			fmt.Fprintf(w, "  dir\t%s\n", cfg.Data.Dir)

			fmt.Fprintln(w, "log:")
			fmt.Fprintf(w, "  level\t%s\n", cfg.Log.Level)
			fmt.Fprintf(w, "  format\t%s\n", cfg.Log.Format)

			fmt.Fprintln(w, "grpc:")
			fmt.Fprintf(w, "  addr\t%s\n", cfg.GRPC.Addr)
			fmt.Fprintf(w, "  control_socket\t%s\n", cfg.GRPC.ControlSocket)
			fmt.Fprintf(w, "  dial_timeout\t%s\n", cfg.GRPC.DialTimeout)
			fmt.Fprintf(w, "  request_timeout\t%s\n", cfg.GRPC.RequestTimeout)

			fmt.Fprintln(w, "raft:")
			fmt.Fprintf(w, "  addr\t%s\n", cfg.Raft.Addr)
			fmt.Fprintf(w, "  bootstrap\t%v\n", cfg.Raft.Bootstrap)
			fmt.Fprintf(w, "  heartbeat_timeout\t%s\n", cfg.Raft.HeartbeatTimeout)
			fmt.Fprintf(w, "  election_timeout\t%s\n", cfg.Raft.ElectionTimeout)
			fmt.Fprintf(w, "  leader_lease_timeout\t%s\n", cfg.Raft.LeaderLeaseTimeout)

			fmt.Fprintln(w, "tls:")
			fmt.Fprintf(w, "  mode\t%s\n", cfg.TLS.Mode)

			fmt.Fprintln(w, "lock:")
			fmt.Fprintf(w, "  ttl\t%s\n", cfg.Lock.TTL)
			fmt.Fprintf(w, "  expire_interval\t%s\n", cfg.Lock.ExpireInterval)

			fmt.Fprintln(w, "lease:")
			fmt.Fprintf(w, "  ttl\t%s\n", cfg.Lease.TTL)
			fmt.Fprintf(w, "  expire_interval\t%s\n", cfg.Lease.ExpireInterval)
			fmt.Fprintf(w, "  presence\t%v\n", cfg.Lease.Presence)
			fmt.Fprintf(w, "  presence_ttl\t%s\n", cfg.Lease.PresenceTTL)

			return w.Flush()
		},
	}
}

func strOrEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
