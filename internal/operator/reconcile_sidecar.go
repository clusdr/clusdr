package operator

import (
	"context"
	"fmt"
)

// reconcileSidecar is PVC-0, bootstrap Job (init + --bootstrap), delete
// Job (RWO), headless STS, then join ordinals ≥ 1.
func (r *Reconciler) reconcileSidecar(ctx context.Context, c Cluster) error {
	if c.Spec.VoterCount < 1 || c.Spec.VoterCount%2 == 0 {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Error",
			Message:            "voterCount must be an odd integer >= 1",
		})
	}

	token, ok, err := r.Kube.Token(ctx, c)
	if err != nil {
		return err
	}
	if !ok {
		if err := r.Kube.EnsurePVC(ctx, c.Spec, c); err != nil {
			return err
		}
		if err := r.Kube.EnsureBootstrapJob(ctx, c.Spec, c); err != nil {
			return err
		}
		ready, err := r.Kube.BootstrapReady(ctx, c)
		if err != nil {
			return err
		}
		if !ready {
			return r.patchStatus(ctx, c, Status{
				ObservedGeneration: c.Generation,
				Phase:              "Pending",
				Message:            "waiting for sidecar bootstrap",
			})
		}
		logs, err := r.Kube.JobLogs(ctx, c)
		if err != nil {
			return err
		}
		token, err = ParseJoinToken(logs)
		if err != nil {
			return r.patchStatus(ctx, c, Status{
				ObservedGeneration: c.Generation,
				Phase:              "Error",
				Message:            "bootstrap finished but join token missing (re-init or set secret)",
			})
		}
		if err := r.Kube.PutToken(ctx, c, token); err != nil {
			return err
		}
		if err := r.Kube.DeleteJob(ctx, c); err != nil {
			return err
		}
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting to start StatefulSet",
		})
	}

	exists, err := r.Kube.JobExists(ctx, c)
	if err != nil {
		return err
	}
	if exists {
		if err := r.Kube.DeleteJob(ctx, c); err != nil {
			return err
		}
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for bootstrap Job to release PVC",
		})
	}

	if err := r.Kube.EnsureHeadless(ctx, c.Spec, c); err != nil {
		return err
	}
	if err := r.Kube.EnsureStatefulSet(ctx, c.Spec, c); err != nil {
		return err
	}
	seedAddr, err := r.Kube.SeedReadyAddr(ctx, c)
	if err != nil {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for sidecar-0 Runtime: " + err.Error(),
		})
	}

	pods, err := r.Kube.MemberPods(ctx, c)
	if err != nil {
		return err
	}

	st, err := r.Joiner.Snapshot(ctx, seedAddr)
	if err != nil {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "members: " + err.Error(),
		})
	}

	drop := dropSet(c.Spec.Leave)
	for id := range drop {
		if !memberListed(st, id) {
			continue
		}
		if err := r.Joiner.Leave(ctx, seedAddr, id); err != nil {
			return r.patchStatus(ctx, c, Status{
				ObservedGeneration: c.Generation,
				Phase:              "Pending",
				Message:            "leave " + id + ": " + err.Error(),
			})
		}
	}

	for _, p := range pods {
		ord := podOrdinal(p.Name)
		if ord <= 0 {
			continue // seed ordinal; restart is clusdr start
		}
		if !p.Ready {
			continue
		}
		if _, dropThis := drop[p.NodeName]; dropThis {
			continue
		}
		if alreadyMember(st, p) {
			continue
		}
		if err := r.Joiner.Join(ctx, joinAddr(p), seedAddr, token, MemberObserver(ord-1, c.Spec.VoterCount)); err != nil {
			return r.patchStatus(ctx, c, Status{
				ObservedGeneration: c.Generation,
				Phase:              "Pending",
				Message:            "join " + p.Name + ": " + err.Error(),
			})
		}
	}

	st, err = r.Joiner.Snapshot(ctx, seedAddr)
	if err != nil {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "members: " + err.Error(),
		})
	}
	st.ObservedGeneration = c.Generation
	if st.Phase == "" {
		st.Phase = "Ready"
	}
	st.Message = ""
	return r.patchStatus(ctx, c, st)
}

func sidecarPodDNS(pod string, c Cluster) string {
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local:%d", pod, headlessName(c.Name), c.Namespace, grpcPort)
}
