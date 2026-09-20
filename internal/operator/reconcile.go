package operator

import (
	"context"
	"fmt"
	"sort"
)

// Cluster is one ClusdrCluster object.
type Cluster struct {
	Namespace, Name, UID string
	Generation           int64
	ResourceVersion      string
	StatusSeed           string
	Warning              string
	Spec                 Spec
}

// MemberStatus is a Runtime Members() row (liveness, not kube Ready).
type MemberStatus struct {
	ID, Address, Status, Role string
}

// Status is patched onto the CR (not EndpointSlice).
type Status struct {
	ClusterID               string
	Leader                  string
	SeedNodeName            string
	Members                 []MemberStatus
	ObservedGeneration      int64
	Phase, Message, Warning string
}

// PodAddr is a hostNetwork daemon the operator can Dial.
type PodAddr struct {
	Name, HostIP, NodeName, Addr string
	Ready                        bool
}

// Platform is the Kubernetes surface (Jobs, DaemonSet, Secrets, CR status).
type Platform interface {
	ReadyNodes(ctx context.Context) ([]string, error)
	ClaimSeed(ctx context.Context, c Cluster, node string) error
	EnsurePrepare(ctx context.Context, spec Spec, c Cluster) error
	PrepareReady(ctx context.Context, c Cluster) (bool, error)
	EnsureJob(ctx context.Context, spec Spec, c Cluster) error
	JobComplete(ctx context.Context, c Cluster) (bool, error)
	JobLogs(ctx context.Context, c Cluster) (string, error)
	Token(ctx context.Context, c Cluster) (string, bool, error)
	PutToken(ctx context.Context, c Cluster, token string) error
	EnsureSeed(ctx context.Context, spec Spec, c Cluster) error
	SeedReadyAddr(ctx context.Context, c Cluster) (string, error)
	EnsureDaemonSet(ctx context.Context, spec Spec, c Cluster) error
	MemberPods(ctx context.Context, c Cluster) ([]PodAddr, error)
	PatchStatus(ctx context.Context, c Cluster, st Status) error
	EnsurePVC(ctx context.Context, spec Spec, c Cluster) error
	EnsureBootstrapJob(ctx context.Context, spec Spec, c Cluster) error
	BootstrapReady(ctx context.Context, c Cluster) (bool, error)
	JobExists(ctx context.Context, c Cluster) (bool, error)
	DeleteJob(ctx context.Context, c Cluster) error
	EnsureHeadless(ctx context.Context, spec Spec, c Cluster) error
	EnsureStatefulSet(ctx context.Context, spec Spec, c Cluster) error
	ClusterCount(ctx context.Context) (int, error)
}

// Joiner is Runtime join + leave + membership snapshot.
type Joiner interface {
	Join(ctx context.Context, localAddr, seedAddr, token string, observer bool) error
	Leave(ctx context.Context, seedAddr, nodeID string) error
	Snapshot(ctx context.Context, seedAddr string) (Status, error)
}

// Reconciler forms a DaemonSet cluster (14.2) from a ClusdrCluster.
type Reconciler struct {
	Kube   Platform
	Joiner Joiner
}

// Reconcile forms a cluster from a ClusdrCluster (DaemonSet or Sidecar).
func (r *Reconciler) Reconcile(ctx context.Context, c Cluster) error {
	if err := r.noteWarning(ctx, &c); err != nil {
		return err
	}
	switch c.Spec.topology() {
	case topoSidecar:
		return r.reconcileSidecar(ctx, c)
	case topoDaemon:
		return r.reconcileDaemon(ctx, c)
	default:
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Error",
			Message:            fmt.Sprintf("unknown topology %q", c.Spec.Topology),
		})
	}
}

// reconcileDaemon is the 14.2 sequence: init Job, seed --bootstrap, DS start, join.
func (r *Reconciler) reconcileDaemon(ctx context.Context, c Cluster) error {
	if c.Spec.VoterCount < 1 || c.Spec.VoterCount%2 == 0 {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Error",
			Message:            "voterCount must be an odd integer >= 1",
		})
	}
	// A missing or restarted pod is not leave. spec.leave is clusdr leave.

	if err := r.resolveSeed(ctx, &c); err != nil {
		return err
	}
	if c.Spec.SeedNodeName == "" {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for a Ready node",
		})
	}

	if err := r.Kube.EnsurePrepare(ctx, c.Spec, c); err != nil {
		return err
	}
	ready, err := r.Kube.PrepareReady(ctx, c)
	if err != nil {
		return err
	}
	if !ready {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for hostPath prepare",
		})
	}

	if err := r.Kube.EnsureJob(ctx, c.Spec, c); err != nil {
		return err
	}
	done, err := r.Kube.JobComplete(ctx, c)
	if err != nil {
		return err
	}
	if !done {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for seed init",
		})
	}
	token, ok, err := r.Kube.Token(ctx, c)
	if err != nil {
		return err
	}
	if !ok {
		logs, err := r.Kube.JobLogs(ctx, c)
		if err != nil {
			return err
		}
		token, err = ParseJoinToken(logs)
		if err != nil {
			return r.patchStatus(ctx, c, Status{
				ObservedGeneration: c.Generation,
				Phase:              "Error",
				Message:            "init finished but join token missing (re-init or set secret)",
			})
		}
		if err := r.Kube.PutToken(ctx, c, token); err != nil {
			return err
		}
	}

	if err := r.Kube.EnsureSeed(ctx, c.Spec, c); err != nil {
		return err
	}
	seedAddr, err := r.Kube.SeedReadyAddr(ctx, c)
	if err != nil {
		return r.patchStatus(ctx, c, Status{
			ObservedGeneration: c.Generation,
			Phase:              "Pending",
			Message:            "waiting for seed Runtime: " + err.Error(),
		})
	}

	if err := r.Kube.EnsureDaemonSet(ctx, c.Spec, c); err != nil {
		return err
	}
	pods, err := r.Kube.MemberPods(ctx, c)
	if err != nil {
		return err
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

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

	for i, p := range pods {
		if p.HostIP == "" || !p.Ready {
			continue
		}
		if _, dropThis := drop[p.NodeName]; dropThis {
			continue
		}
		if alreadyMember(st, p) {
			continue
		}
		if err := r.Joiner.Join(ctx, joinAddr(p), seedAddr, token, MemberObserver(i, c.Spec.VoterCount)); err != nil {
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
