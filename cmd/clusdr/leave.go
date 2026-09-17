package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func newLeaveCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "leave [node-id]",
		Short: "Remove a member from the Raft cluster",
		Long: `Ask the local daemon to remove a cluster member from Raft.

This is the only RemoveServer. Presence expiry and heartbeat misses mark a
node not-alive but keep its Raft id. Crash and reboot are not leave.

With no argument, removes this node. Unknown id is an error. Already gone
is success.

After leave, the same data.dir is not a member: run clusdr join to add it
again. A process that was only marked not-alive uses clusdr start.

Example:
  clusdr leave
  clusdr leave node-b`,
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
				RequestLeave(ctx, &pb.RequestLeaveRequest{NodeId: nodeID})
			if err != nil {
				return fmt.Errorf("leave: %w", err)
			}
			if !resp.Left {
				return fmt.Errorf("leave rejected: %s", resp.Message)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "left cluster")
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
