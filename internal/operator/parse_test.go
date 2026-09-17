package operator

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestClusterFromSpec(t *testing.T) {
	t.Parallel()
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "clusdr.io/v1alpha1",
		"kind":       "ClusdrCluster",
		"metadata": map[string]any{
			"name":       "clusdr",
			"namespace":  "clusdr",
			"uid":        "abc",
			"generation": int64(4),
		},
		"spec": map[string]any{
			"topology":     "DaemonSet",
			"voterCount":   int64(3),
			"image":        "durguto/clusdr:0.2.0",
			"dataDir":      "/var/lib/clusdr",
			"seedNodeName": "kind-worker",
			"leave":        []any{"node-b"},
		},
	}}
	c, err := ClusterFrom(u)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "clusdr" || c.Spec.VoterCount != 3 || c.Spec.SeedNodeName != "kind-worker" {
		t.Fatalf("%+v", c)
	}
	if c.Generation != 4 {
		t.Fatalf("generation %d", c.Generation)
	}
	if len(c.Spec.Leave) != 1 || c.Spec.Leave[0] != "node-b" {
		t.Fatalf("leave %+v", c.Spec.Leave)
	}
}
