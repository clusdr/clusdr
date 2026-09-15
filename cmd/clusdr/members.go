package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pb "github.com/durguto/clusdr/api/clusdr/v1alpha1"
)

func displayMemberRole(m *pb.Member) string {
	if m.GetLeader() {
		return "leader"
	}
	if m.GetRole() == "observer" {
		return "observer"
	}
	return "voter"
}

func newMembersCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "members",
		Short: "List cluster members",
		Long:  "List all known members of the cluster with their current status.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			resp, err := pb.NewMembershipServiceClient(conn).
				ListMembers(ctx, &pb.ListMembersRequest{})
			if err != nil {
				return fmt.Errorf("list members: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(resp.Members) == 0 {
				fmt.Fprintln(out, "no members")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tROLE")
			for _, m := range resp.Members {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Id, m.Address, m.Status, displayMemberRole(m))
			}
			return w.Flush()
		},
	}
}
