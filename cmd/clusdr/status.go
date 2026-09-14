package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newStatusCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Clusdr daemon status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			socket := cfg.GRPC.ControlSocket

			if _, err := os.Stat(socket); err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(out, "daemon not running (socket not found: %s)\n", socket)
					// Exit 1 so scripts can check daemon presence.
					os.Exit(1)
				}
				return fmt.Errorf("stat %q: %w", socket, err)
			}

			fmt.Fprintf(out, "daemon running (socket: %s)\n", socket)
			return nil
		},
	}
}
