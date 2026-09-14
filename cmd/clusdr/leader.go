package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

func newLeaderCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "leader",
		Short: "Show the current cluster leader",
		Long:  "Print the node ID and address of the current Raft leader.",
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
				GetLeader(ctx, &pb.GetLeaderRequest{})
			if err != nil {
				return fmt.Errorf("get leader: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "leader id : %s\n", resp.LeaderId)
			fmt.Fprintf(out, "address   : %s\n", resp.Address)
			return nil
		},
	}
}
