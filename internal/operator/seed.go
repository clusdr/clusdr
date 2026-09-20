package operator

import (
	"context"
	"sort"
)

func (r *Reconciler) patchStatus(ctx context.Context, c Cluster, st Status) error {
	if st.SeedNodeName == "" {
		st.SeedNodeName = c.Spec.SeedNodeName
	}
	st.Warning = c.Warning
	return r.Kube.PatchStatus(ctx, c, st)
}

// resolveSeed pins one Kubernetes node for init + --bootstrap.
// spec.seedNodeName wins. Else status.seedNodeName (a prior claim).
// Else pick the first Ready node, write it on the CR, then use it.
// The write is the lock: a resourceVersion conflict means another
// reconcile already claimed; the loser must not start a second init.
func (r *Reconciler) resolveSeed(ctx context.Context, c *Cluster) error {
	want := c.Spec.SeedNodeName
	if want == "" {
		want = c.StatusSeed
	}
	if want == "" {
		nodes, err := r.Kube.ReadyNodes(ctx)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			return nil
		}
		sort.Strings(nodes)
		want = nodes[0]
		if err := r.Kube.ClaimSeed(ctx, *c, want); err != nil {
			return err
		}
		c.Spec.SeedNodeName = want
		c.StatusSeed = want
		return nil
	}
	c.Spec.SeedNodeName = want
	if c.StatusSeed != want {
		if err := r.Kube.ClaimSeed(ctx, *c, want); err != nil {
			return err
		}
		c.StatusSeed = want
	}
	return nil
}
