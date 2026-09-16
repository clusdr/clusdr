package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func newPublishCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "publish <topic> [payload]",
		Short: "Publish a custom cluster event",
		Long: `Publish an opaque event on a topic. Every cluster node emits it
on its Watch stream as type custom.<topic>.

Examples:
  clusdr publish deployment
  clusdr publish deployment '{"sha":"abc"}'`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			var payload []byte
			if len(args) == 2 {
				payload = []byte(args[1])
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.GRPC.RequestTimeout)
			defer cancel()

			conn, err := dialDaemon(cfg)
			if err != nil {
				return err
			}
			defer conn.Close()

			resp, err := pb.NewEventServiceClient(conn).PublishEvent(ctx, &pb.PublishEventRequest{
				Topic:   args[0],
				Payload: payload,
			})
			if err != nil {
				return fmt.Errorf("publish: %w", err)
			}
			if !resp.Accepted {
				return fmt.Errorf("publish rejected: %s", resp.Message)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "published  type=%s  event_id=%s\n",
				resp.Type, resp.EventId)
			return nil
		},
	}
}
