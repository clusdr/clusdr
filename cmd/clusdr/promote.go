package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

func newPromoteCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "promote [node-id]",
		Short: "Promote an observer to a Raft voter",
		Long: `Ask the local daemon to promote a cluster member from observer to voter.

With no argument, promotes this node. The leader adds the node to the
Raft quorum and updates membership role to voter.

Example:
  clusdr promote
  clusdr promote node-obs`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPC.RequestTimeout)
			defer cancel()

			conn, err := dialDaemon(cfg)
			if err != nil {
				return err
			}
			defer conn.Close()

			var nodeID string
			if len(args) == 1 {
				nodeID = args[0]
			}

			resp, err := pb.NewControlServiceClient(conn).
				RequestPromote(ctx, &pb.RequestPromoteRequest{NodeId: nodeID})
			if err != nil {
				return fmt.Errorf("promote: %w", err)
			}
			if !resp.Promoted {
				return fmt.Errorf("promote rejected: %s", resp.Message)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "promoted to voter")
			fmt.Fprintln(out)

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tROLE")
			for _, m := range resp.Members {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Id, m.Address, m.Status, displayMemberRole(m))
			}
			return w.Flush()
		},
	}
}
