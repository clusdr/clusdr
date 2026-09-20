package operator

import (
	"context"
	"testing"
)

func TestReconcileTwoClustersSetsWarning(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{clusterCount: 2, prepareBlocked: true}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), testCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.status.Warning != warnTwoClusters {
		t.Fatalf("warning %q", kube.status.Warning)
	}
	if kube.status.Phase != "Pending" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
}

func TestReconcileOneClusterClearsWarning(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{
		jobDone:  true,
		tokenOK:  true,
		token:    "tok-1",
		seedAddr: "10.0.0.1:7947",
	}
	c := testCluster()
	c.Spec.SeedNodeName = "kind-worker"
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{snap: Status{ClusterID: "cid", Warning: "stale"}}}
	if err := r.Reconcile(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if kube.status.Phase != "Ready" {
		t.Fatalf("phase %q", kube.status.Phase)
	}
	if kube.status.Warning != "" {
		t.Fatalf("one CR must not warn: %q", kube.status.Warning)
	}
}

func TestReconcileSidecarTwoClustersSetsWarning(t *testing.T) {
	t.Parallel()
	kube := &fakePlatform{clusterCount: 2}
	r := &Reconciler{Kube: kube, Joiner: &fakeJoiner{}}
	if err := r.Reconcile(context.Background(), sidecarCluster()); err != nil {
		t.Fatal(err)
	}
	if kube.status.Warning != warnTwoClusters {
		t.Fatalf("warning %q", kube.status.Warning)
	}
}
