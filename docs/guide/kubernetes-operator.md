# Install the Operator

Reconcile one `ClusdrCluster` into the host topology. The Operator talks to daemons over the Runtime API. It does not embed Raft. Join is automatic; `leave` is only `spec.leave`.

Field list: [ClusdrCluster](../reference/clusdrcluster.md). Why DaemonSet vs Sidecar: [Kubernetes](../concepts/kubernetes.md).

## 1. Install CRD and Operator

```bash
kubectl apply -f https://clusdr.io/download/clusdr-crds.yaml
kubectl apply -f https://clusdr.io/download/clusdr-operator.yaml
```

Pin a tag with `clusdr-crds-0.2.0.yaml` / `clusdr-operator-0.2.0.yaml` on the same origin. Contributor: `kubectl apply -k config/crd` then `kubectl apply -k config/operator`.

Image: `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`). Same tag as the daemon.

## 2. Data dir and seed node

hostPath still needs `mkdir` + `chown 65532` on each node. Set `spec.seedNodeName` so init and `--bootstrap` share one disk.

```bash
kubectl get nodes
```

## 3. Apply a cluster object

DaemonSet sample:

```bash
kubectl apply -f https://raw.githubusercontent.com/clusdr/clusdr/main/examples/k8s/clusdrcluster.yaml
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"seedNodeName":"<node>"}}'
```

Sidecar sample: [`clusdrcluster-sidecar.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster-sidecar.yaml). Do not apply both unless you mean **two** clusters.

The CRD is not in the Helm chart.

## 4. Watch it form

```bash
kubectl get clusdrcluster clusdr -n clusdr
```

Phase moves `Pending` → `Ready`. Token is a Secret, not git. Distroless has no shell: the Operator runs seed `init` (and sidecar `--bootstrap` on PVC-0).

## 5. Leave (only when you mean it)

A missing or restarted pod is **not** leave.

```bash
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"leave":["<clusdr-node-id>"]}}'
```

Bounce a member (same node, same hostPath) and check the id stays:

```bash
kubectl delete pod -n clusdr -l app.kubernetes.io/component=member
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
```

## Checkpoint

`kubectl get clusdrcluster` shows Phase `Ready` and a Leader. `status.members` matches `clusdr members`. Sidecar replicas = `voterCount`; scaling a workload Deployment does not add Raft members.
