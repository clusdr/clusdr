package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func newWatchCmd(f *rootFlags) *cobra.Command {
	var types []string
	var topics []string
	var lastSeq uint64

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream cluster events",
		Long: `Subscribe to the local daemon's Watch stream.

On connect the server sends a snapshot of current members and the leader
(seq=0), then watch.sync (bus high-water mark), then live events.
If --last-seq is set and the bus has moved past it, a watch.gap event
is sent so the client knows it missed events while disconnected.
--topic restricts the stream to custom.<topic> events (no membership snapshot).

Examples:
  clusdr watch
  clusdr watch --type member.join --type member.dead --type member.left
  clusdr watch --topic deployment
  clusdr watch --last-seq 42`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(f)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			conn, err := dialDaemon(cfg)
			if err != nil {
				return err
			}
			defer conn.Close()

			stream, err := pb.NewWatchServiceClient(conn).Watch(ctx, &pb.WatchRequest{
				EventTypes: types,
				LastSeq:    lastSeq,
				Topics:     topics,
			})
			if err != nil {
				return fmt.Errorf("watch: %w", err)
			}

			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SEQ\tTYPE\tSOURCE\tPAYLOAD")
			if err := w.Flush(); err != nil {
				return err
			}

			for {
				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF || status.Code(err) == codes.Canceled {
						return nil
					}
					if ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("watch recv: %w", err)
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
					resp.Seq, resp.Type, resp.Source, resp.Payload)
				if err := w.Flush(); err != nil {
					return err
				}
			}
		},
	}

	cmd.Flags().StringSliceVar(&types, "type", nil,
		"only receive these event types (repeatable)")
	cmd.Flags().StringSliceVar(&topics, "topic", nil,
		"only receive custom events for these topics (repeatable)")
	cmd.Flags().Uint64Var(&lastSeq, "last-seq", 0,
		"last live seq processed; used on reconnect for gap detection")
	return cmd
}
