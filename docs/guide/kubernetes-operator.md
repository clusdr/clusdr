# Operator

The Operator reconciles one `ClusdrCluster` into the same host topology the YAML and the [Helm](kubernetes-helm.md) chart already describe. It talks to daemons over the Runtime/Control API. It does not embed Raft.

Crash is `member.dead`. [`clusdr leave`](../reference/cli/leave.md) is `member.left`. Status is liveness only: `alive` or `dead`.

## Install

```bash
kubectl apply -f https://clusdr.io/download/clusdr-crds.yaml
kubectl apply -f https://clusdr.io/download/clusdr-operator.yaml
kubectl apply -f https://raw.githubusercontent.com/clusdr/clusdr/main/examples/k8s/clusdrcluster.yaml
```

Pin a tag with `clusdr-crds-0.2.0.yaml` / `clusdr-operator-0.2.0.yaml` on the same origin. Fallback: GitHub Releases (`…/releases/latest/download/clusdr-crds.yaml`). Contributor checkout: `kubectl apply -k config/crd` then `kubectl apply -k config/operator`.

Image: `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`). Same tag as the daemon. Not baked into `durguto/clusdr`.

The CRD is not in the Helm chart. Sample objects: [`clusdrcluster.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster.yaml) (DaemonSet) and [`clusdrcluster-sidecar.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster-sidecar.yaml) (Sidecar). Do not apply both unless you mean **two** clusters.

## What the object means

`kubectl get clusdrcluster` works after applying the CRD.

| Field | Meaning |
|---|---|
| `spec.topology` | `DaemonSet` (default) or `Sidecar` |
| `spec.voterCount` | Odd. Extra DaemonSet nodes join as observers |
| `spec.image` | Daemon image |
| `spec.dataDir` | Node-local dir / PVC |
| `spec.seedNodeName` | Same disk for `init` and `--bootstrap` |
| `spec.leave` | clusdr `node.id` values. The Operator runs [`clusdr leave`](../reference/cli/leave.md) for those ids |

There is no `ClusdrMember`. There is no field that tells apps to `Dial` a Service.

Status is filled from [`Members()`](../reference/cli/members.md) / Health, not EndpointSlice: `clusterID`, `leader`, `members[]` (`alive` / `dead`), plus `phase` (`Pending` / `Ready` / `Error` / `Unsupported`).

## How it forms the cluster

Same CLI a human would type: `init` once, one `--bootstrap`, then `join` / `join --observer`. Distroless has **no shell**, so Jobs do seed `init` (and sidecar `--bootstrap` on PVC-0; the Job is then deleted so the StatefulSet can mount the RWO volume). The join token is a Secret, not git.

DaemonSet: Operator dials `hostIP:7947`. Sidecar: headless DNS of the ordinal. The app still uses `Local()` — node Runtime, or `127.0.0.1` in the sidecar pod ([Apps on the node](kubernetes.md#apps-on-the-node), [Sidecar](kubernetes-sidecar.md)).

A pod restart with intact `data.dir` / PVC is `clusdr start`: the id stays in `Members()` (`dead` then `alive`). The Operator does not `join` again and does not `leave`. Only `spec.leave` drops a Raft id.

hostPath still needs `mkdir` + `chown 65532` on each node. Set `spec.seedNodeName` so init and `--bootstrap` share one disk. The Operator uses tcp probes on port 7947. Helm can use `clusdr health`.

Bounce a member (same node, same hostPath) — the id stays; it must not disappear:

```bash
kubectl delete pod -n clusdr -l app.kubernetes.io/component=member
kubectl get clusdrcluster clusdr -n clusdr -o jsonpath='{.status.members}' ; echo
```

Drop a node for good (`member.left`):

```bash
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"leave":["<clusdr-node-id>"]}}'
```

A missing or restarted pod is **not** leave. Sidecar replicas = `voterCount`; scaling that StatefulSet scales Raft. Scaling a workload Deployment does not.

## What the Operator is not

- `AddVoter` when a Deployment scales
- Treating a crash or missing pod as leave
- Sidecar injection
- A replacement for the [Lease API](https://kubernetes.io/docs/concepts/architecture/leases/) (Operator HA, if added, would use that too)
- Raft inside the Operator process

In-tree: [`config/crd`](https://github.com/clusdr/clusdr/tree/main/config/crd), [`config/operator`](https://github.com/clusdr/clusdr/tree/main/config/operator). Build the image yourself is the contributor path (`docker build -f Dockerfile.operator`).
