package operator

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestReadyNodesSkipsUnschedulableAndNotReady(t *testing.T) {
	t.Parallel()
	cs := k8sfake.NewSimpleClientset(
		node("worker-b", true, false),
		node("worker-a", true, true),
		node("worker-c", false, false),
	)
	got, err := (Kube{Core: cs}).ReadyNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "worker-b" {
		t.Fatalf("%v", got)
	}
}

func TestClaimSeedWritesStatus(t *testing.T) {
	t.Parallel()
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "clusdr.io/v1alpha1",
		"kind":       "ClusdrCluster",
		"metadata": map[string]any{
			"name":            "clusdr",
			"namespace":       "clusdr",
			"resourceVersion": "10",
		},
		"status": map[string]any{"phase": "Pending"},
	}}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		GVR: "ClusdrClusterList",
	}, u)
	k := Kube{Dyn: dyn}
	c := Cluster{Namespace: "clusdr", Name: "clusdr", ResourceVersion: "10"}
	if err := k.ClaimSeed(context.Background(), c, "kind-worker"); err != nil {
		t.Fatal(err)
	}
	got, err := dyn.Resource(GVR).Namespace("clusdr").Get(context.Background(), "clusdr", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	name, _, _ := unstructured.NestedString(got.Object, "status", "seedNodeName")
	if name != "kind-worker" {
		t.Fatalf("seed %q", name)
	}
}

func node(name string, ready, unschedulable bool) *corev1.Node {
	st := corev1.ConditionFalse
	if ready {
		st = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{
				Type:   corev1.NodeReady,
				Status: st,
			}},
		},
	}
}
