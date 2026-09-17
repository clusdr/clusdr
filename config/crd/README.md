# ClusdrCluster CRD

Kind `ClusdrCluster`, group `clusdr.io`, namespaced. Desired **host** topology. Raft stays the member list. There is no `ClusdrMember`.

```bash
kubectl apply -f https://clusdr.io/download/clusdr-crds.yaml
# contributor: kubectl apply -k config/crd
kubectl apply -f examples/k8s/clusdrcluster.yaml
kubectl get clusdrcluster -n clusdr
```

Applying a `ClusdrCluster` forms a cluster when [`clusdr-operator`](../operator) is running (`DaemonSet` default, or `Sidecar` STS). `spec.leave` is `clusdr leave` (`member.left`); a missing or restarted pod is not leave. Apps still `Local()` — the spec has no ClusterIP to `Dial`.

Helm chart does not install this CRD.
