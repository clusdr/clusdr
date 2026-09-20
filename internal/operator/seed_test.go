package operator

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestReconcileWaitsForReadyNode(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{readyNodes: []string{}}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.claims != 0 || kube.ensureJob != 0 || kube.ensurePrepare != 0 {
		t.Fatalf("claims=%d job=%d prepare=%d", kube.claims, kube.ensureJob, kube.ensurePrepare)
	}
	if kube.status.Phase != "Pending" || kube.status.Message != "waiting for a Ready node" {
		t.Fatalf("status %+v", kube.status)
	}
}

func TestReconcileClaimsSeedBeforeJob(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		readyNodes: []string{"worker-b", "worker-a"},
		jobDone:    false,
	}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.claims != 1 || kube.claimedSeed != "worker-a" {
		t.Fatalf("claim %d %q", kube.claims, kube.claimedSeed)
	}
	if kube.ensureJob != 1 || kube.ensureSeed != 0 {
		t.Fatalf("job=%d seed=%d", kube.ensureJob, kube.ensureSeed)
	}
	if kube.status.SeedNodeName != "worker-a" {
		t.Fatalf("status seed %q", kube.status.SeedNodeName)
	}
}

func TestReconcileRestartKeepsClaimedSeed(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		readyNodes: []string{"worker-a", "worker-c"},
		jobDone:    false,
	}
	c := testCluster()
	c.StatusSeed = "worker-b"
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.claims != 0 {
		t.Fatalf("must not claim again: %d %q", kube.claims, kube.claimedSeed)
	}
	if kube.ensureJob != 1 {
		t.Fatalf("job=%d", kube.ensureJob)
	}
	if kube.status.SeedNodeName != "worker-b" {
		t.Fatalf("status seed %q", kube.status.SeedNodeName)
	}
}

func TestReconcileSpecSeedWins(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		readyNodes: []string{"worker-a"},
		jobDone:    false,
	}
	c := testCluster()
	c.Spec.SeedNodeName = "pin-me"
	c.StatusSeed = "auto-old"
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.claimedSeed != "pin-me" || kube.claims != 1 {
		t.Fatalf("claim %d %q", kube.claims, kube.claimedSeed)
	}
	if kube.status.SeedNodeName != "pin-me" {
		t.Fatalf("status seed %q", kube.status.SeedNodeName)
	}
}

func TestReconcileClaimConflictStops(t *testing.T) {
	t.Parallel()
	conflict := apierrors.NewConflict(schema.GroupResource{Group: "clusdr.io", Resource: "clusdrclusters"}, "clusdr", errors.New("seed claimed"))
	kube := &fakePlatform{
		readyNodes: []string{"worker-a"},
		claimErr:   conflict,
	}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err == nil {
		t.Fatal("expected conflict")
	}
	if kube.ensureJob != 0 || kube.ensurePrepare != 0 {
		t.Fatal("loser must not start init")
	}
}

func TestReconcileSidecarSkipsAutoSeed(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{readyNodes: []string{"worker-a"}}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), sidecarCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.claims != 0 {
		t.Fatalf("sidecar must not claim a node: %d", kube.claims)
	}
}

func TestReadyStatusKeepsSeed(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
	}
	c := testCluster()
	c.Spec.SeedNodeName = "kind-worker"
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{snap: Status{ClusterID: "cid"}}}
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.status.Phase != "Ready" || kube.status.SeedNodeName != "kind-worker" {
		t.Fatalf("status %+v", kube.status)
	}
}
