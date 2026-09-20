package operator

import "context"

// warnTwoClusters is the printer-column string. Two ClusdrCluster
// objects are two Raft groups. Not a webhook; the column is the warning.
const warnTwoClusters = "two clusters"

func (r *Reconciler) noteWarning(ctx context.Context, c *Cluster) error {
	n, err := r.Kube.ClusterCount(ctx)
	if err != nil {
		return err
	}
	if n >= 2 {
		c.Warning = warnTwoClusters
	}
	return nil
}
