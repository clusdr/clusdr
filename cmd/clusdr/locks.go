package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

func newLocksCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "locks",
		Short: "List active distributed locks",
		Long:  "List every lock currently held in the cluster (name, holder, fencing token, deadline).",
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

			resp, err := pb.NewLockServiceClient(conn).ListLocks(ctx, &pb.ListLocksRequest{})
			if err != nil {
				return fmt.Errorf("list locks: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(resp.Locks) == 0 {
				fmt.Fprintln(out, "no locks")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tHOLDER\tTOKEN\tACQUIRED\tDEADLINE")
			for _, l := range resp.Locks {
				acquired := ""
				if l.AcquiredUnixMs > 0 {
					acquired = time.UnixMilli(l.AcquiredUnixMs).UTC().Format(time.RFC3339)
				}
				deadline := ""
				if l.DeadlineUnixMs > 0 {
					deadline = time.UnixMilli(l.DeadlineUnixMs).UTC().Format(time.RFC3339)
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", l.Name, l.Holder, l.FencingToken, acquired, deadline)
			}
			return w.Flush()
		},
	}
}
