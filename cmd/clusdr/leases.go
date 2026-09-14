package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	pb "github.com/odurgut/clusdr/api/clusdr/v1alpha1"
)

func newLeasesCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "leases",
		Short: "List active leases",
		Long:  "List every lease currently held in the cluster (name, owner, fencing token, deadline).",
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

			resp, err := pb.NewLeaseServiceClient(conn).ListLeases(ctx, &pb.ListLeasesRequest{})
			if err != nil {
				return fmt.Errorf("list leases: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(resp.Leases) == 0 {
				fmt.Fprintln(out, "no leases")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tOWNER\tTOKEN\tGRANTED\tDEADLINE")
			for _, l := range resp.Leases {
				granted := ""
				if l.GrantedUnixMs > 0 {
					granted = time.UnixMilli(l.GrantedUnixMs).UTC().Format(time.RFC3339)
				}
				deadline := ""
				if l.DeadlineUnixMs > 0 {
					deadline = time.UnixMilli(l.DeadlineUnixMs).UTC().Format(time.RFC3339)
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", l.Name, l.Owner, l.FencingToken, granted, deadline)
			}
			return w.Flush()
		},
	}
}
