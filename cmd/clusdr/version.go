package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/odurgut/clusdr/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "clusdr %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.BuildTime)
		},
	}
}
