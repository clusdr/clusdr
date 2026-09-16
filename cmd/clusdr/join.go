package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pb "github.com/clusdr/clusdr/api/clusdr/v1alpha1"
)

func newJoinCmd(f *rootFlags) *cobra.Command {
	var token string
	var observer bool

	cmd := &cobra.Command{
		Use:   "join <addr>",
		Short: "Join an existing cluster",
		Long: `Tell the local daemon to join the Clusdr cluster at the given address.

The daemon connects to <addr>, presents the join token from 'clusdr init',
and bootstraps membership plus a CA-issued node certificate.

--observer joins as a Raft non-voter: same local API, writes forward to
the leader, quorum is unchanged if this node dies.

Example:
  clusdr join --token <token> 10.0.0.1:7947
  clusdr join --observer --token <token> 10.0.0.1:7947`,
		Args: cobra.ExactArgs(1),
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

			resp, err := pb.NewControlServiceClient(conn).
				RequestJoin(ctx, &pb.RequestJoinRequest{Addr: args[0], Token: token, Observer: observer})
			if err != nil {
				return fmt.Errorf("join: %w", err)
			}
			if !resp.Joined {
				return fmt.Errorf("join rejected: %s", resp.Message)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "joined cluster")
			if resp.CertIssued {
				fmt.Fprintln(out, "node certificate issued")
			}
			fmt.Fprintln(out)

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tROLE")
			for _, m := range resp.Members {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Id, m.Address, m.Status, displayMemberRole(m))
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "cluster join token from 'clusdr init'")
	cmd.Flags().BoolVar(&observer, "observer", false, "join as a Raft non-voter (does not affect quorum)")
	return cmd
}
