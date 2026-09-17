package operator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ClusterFrom maps a ClusdrCluster unstructured object to Cluster.
func ClusterFrom(u *unstructured.Unstructured) (Cluster, error) {
	if u == nil {
		return Cluster{}, fmt.Errorf("nil ClusdrCluster")
	}
	c := Cluster{
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
		UID:        string(u.GetUID()),
		Generation: u.GetGeneration(),
	}
	spec, _, _ := unstructured.NestedMap(u.Object, "spec")
	c.Spec.Topology, _, _ = unstructured.NestedString(spec, "topology")
	c.Spec.Image, _, _ = unstructured.NestedString(spec, "image")
	c.Spec.DataDir, _, _ = unstructured.NestedString(spec, "dataDir")
	c.Spec.SeedNodeName, _, _ = unstructured.NestedString(spec, "seedNodeName")
	if n, ok, _ := unstructured.NestedInt64(spec, "voterCount"); ok {
		c.Spec.VoterCount = int(n)
	}
	if raw, ok, _ := unstructured.NestedSlice(spec, "leave"); ok {
		for _, v := range raw {
			s, ok := v.(string)
			if ok && s != "" {
				c.Spec.Leave = append(c.Spec.Leave, s)
			}
		}
	}
	return c, nil
}

func statusMap(st Status) map[string]any {
	members := make([]any, 0, len(st.Members))
	for _, m := range st.Members {
		members = append(members, map[string]any{
			"id":      m.ID,
			"address": m.Address,
			"status":  m.Status,
			"role":    m.Role,
		})
	}
	out := map[string]any{
		"clusterID":          st.ClusterID,
		"leader":             st.Leader,
		"members":            members,
		"observedGeneration": st.ObservedGeneration,
		"phase":              st.Phase,
		"message":            st.Message,
	}
	return out
}
