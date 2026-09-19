# Install the Operator

`clusdr-operator` reconciles one `ClusdrCluster` by talking to daemons over the Runtime API and running the same `init` / `--bootstrap` / `join` sequence a human would. It does not embed Raft. Join is automatic; `leave` is only `spec.leave` — deleting a pod is not leave, so a bounce must not shrink quorum.

Field list: [ClusdrCluster](../reference/clusdrcluster.md). Why DaemonSet vs Sidecar: [Kubernetes](../concepts/kubernetes.md).

## 1. Install CRD and Operator

```bash
kubectl apply -f https://clusdr.io/download/clusdr-crds.yaml
kubectl apply -f https://clusdr.io/download/clusdr-operator.yaml
```

Pin a tag with `clusdr-crds-0.2.0.yaml` / `clusdr-operator-0.2.0.yaml` on the same origin if you need a known CRD. Contributor: `kubectl apply -k config/crd` then `kubectl apply -k config/operator`.

Image: `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`). Same tag as the daemon. The CRD is not in the Helm chart — installing only the chart leaves `kubectl get clusdrcluster` unknown.

## 2. Data dir and seed node

hostPath still needs `mkdir` + `chown 65532` on each node or the seed Job cannot write identity ([Run on Kubernetes](kubernetes.md#1-data-dir-on-each-node)).

```bash
kubectl get nodes
```

You will set `spec.seedNodeName` to one of those names so init and `--bootstrap` share one disk.

## 3. Apply a cluster object

```bash
kubectl apply -f https://raw.githubusercontent.com/clusdr/clusdr/main/examples/k8s/clusdrcluster.yaml
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p "{\"spec\":{\"seedNodeName\":\"${NODE}\"}}"
```

Pick the node you actually prepared. Sidecar sample: [`clusdrcluster-sidecar.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster-sidecar.yaml). Do not apply both unless you mean **two** clusters (two Raft groups, two tokens).

`voterCount` must be odd; the CRD CEL rule rejects even values at apply time.

## 4. Watch it form

```bash
kubectl get clusdrcluster clusdr -n clusdr -w
```

Phase moves `Pending` → `Ready`. Token is a Secret, not git. Distroless has no shell: the Operator runs seed `init` (and sidecar `--bootstrap` on PVC-0, then deletes that Job so the RWO volume can mount).

If phase stays `Pending` with “waiting for sidecar bootstrap” or seed init, the Job failed — describe it. `Error` + “join token missing” means init logs were empty; fix the Job, do not hand-edit Raft.

## 5. Leave (only when you mean it)

A missing or restarted pod is **not** leave. To drop a Raft id for good:

```bash
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"leave":["<id-from-status.members>"]}}'
```

Bounce a member (same node, same hostPath) and check the id stays:

```bash
kubectl delete pod -n clusdr -l app.kubernetes.io/component=member
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
```

If the id disappeared, you patched `leave` or the disk was empty.

## Checkpoint

`kubectl get clusdrcluster` shows Phase `Ready` and a Leader. `status.members` matches `clusdr members` on the seed. Sidecar replicas = `voterCount`; scaling a workload Deployment does not add Raft members.
