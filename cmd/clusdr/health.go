package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func newHealthCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Call the Runtime Health RPC",
		Long: `Dial the local Runtime API and call Health.

This is the process probe (gRPC is serving). It is not clusdr status
(Unix socket file) and it is not Raft role. In this version healthy is
always true and role is always standalone.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), cfg.GRPC.RequestTimeout)
			defer cancel()

			conn, err := dialDaemon(cfg)
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := pb.NewHealthServiceClient(conn).Health(ctx, &pb.HealthRequest{})
			if err != nil {
				return fmt.Errorf("health: %w", err)
			}
			if !resp.GetHealthy() {
				return fmt.Errorf("health: not healthy")
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "healthy    : %v\n", resp.GetHealthy())
			fmt.Fprintf(out, "node id    : %s\n", resp.GetNodeId())
			fmt.Fprintf(out, "cluster id : %s\n", resp.GetClusterId())
			fmt.Fprintf(out, "role       : %s\n", resp.GetRole())
			return nil
		},
	}
}
