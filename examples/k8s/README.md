# Kubernetes examples

These manifests apply the same Linux host model as the how-to pages: one daemon per node, `data.dir` on that node, advertised addresses peers can dial. They do **not** install the Operator or call `join` for you. Read or apply them when you want the YAML without Helm.

Same topology as the Helm chart (`oci://ghcr.io/clusdr/charts/clusdr`; in-tree [`charts/clusdr`](../../charts/clusdr)). CRD samples: [`clusdrcluster.yaml`](clusdrcluster.yaml) / [`clusdrcluster-sidecar.yaml`](clusdrcluster-sidecar.yaml). Operator install: [`config/operator`](../../config/operator). App Deployments do not join Raft.

| File | What |
|---|---|
| [namespace.yaml](namespace.yaml) | `clusdr` namespace |
| [prepare.yaml](prepare.yaml) | DaemonSet: `mkdir` + `chown 65532` on hostPath (not the daemon image) |
| [seed-init.yaml](seed-init.yaml) | Job: `clusdr init` on the seed node's hostPath |
| [seed.yaml](seed.yaml) | One voter: `clusdr start --bootstrap` |
| [daemonset.yaml](daemonset.yaml) | One daemon on every **other** node |
| [app.yaml](app.yaml) | Your app: `CLUSDR_GRPC_ADDR` = this node's Runtime. Not a member |
| [sidecar-bootstrap.yaml](sidecar-bootstrap.yaml) | Exception: PVC-0 + Job `init` then `start --bootstrap` |
| [sidecar-statefulset.yaml](sidecar-statefulset.yaml) | Exception: 3-replica STS, app + sidecar. Replica **is** the member |
| [clusdrcluster.yaml](clusdrcluster.yaml) | `ClusdrCluster` DaemonSet sample. Needs the CRD. Operator forms the cluster |
| [clusdrcluster-sidecar.yaml](clusdrcluster-sidecar.yaml) | `ClusdrCluster` Sidecar sample. PVC STS, app `127.0.0.1`. Not injection |

Image: published `durguto/clusdr` (example pins `0.2.0`). Probes call [`clusdr health`](../../docs/reference/cli/health.md) (Runtime Health RPC). Else `tcpSocket` port `7947`. Never `clusdr status` (Unix socket).

Kube's native gRPC probe speaks `grpc.health.v1`, which this daemon does not implement.

Why: [Kubernetes](../../docs/concepts/kubernetes.md). How-to: [Run on Kubernetes](../../docs/guide/kubernetes.md) · [Helm](../../docs/guide/kubernetes-helm.md) · [Operator](../../docs/guide/kubernetes-operator.md) · [Sidecar](../../docs/guide/kubernetes-sidecar.md). Addresses: [other hosts](../../docs/guide/other-hosts.md). Crash vs leave: [presence](../../docs/concepts/presence.md).

## Three-node kind (or k3s)

You want **3 members**. Seed is one voter. The DaemonSet skips the seed's node, so two workers become the other two members. Join those two as **voters**. Extra nodes later: `join --observer`.

### 1. Data dir on each node

hostPath is not chowned by `fsGroup`. Distroless runs as uid **65532**. Apply [prepare.yaml](prepare.yaml) (root `mkdir` + `chown`, busybox, not privileged). You do not `docker exec` onto the node.

```bash
kubectl apply -f examples/k8s/namespace.yaml
kubectl apply -f examples/k8s/prepare.yaml
kubectl rollout status -n clusdr ds/clusdr-prepare
```

### 2. Same node for init and seed

Pick a node (`kubectl get nodes`). Set `spec.template.spec.nodeName` (Job) and `spec.template.spec.nodeName` (Deployment) to that name in `seed-init.yaml` and `seed.yaml`. They must share one hostPath.

Control-plane tainted (kind): the files already tolerate that taint. Still set `nodeName` so Job and Deployment cannot split.

### 3. Apply

```bash
kubectl apply -f examples/k8s/seed-init.yaml
kubectl wait -n clusdr --for=condition=complete job/clusdr-seed-init --timeout=60s
kubectl logs -n clusdr job/clusdr-seed-init
```

Copy the **join token** (once). Then:

```bash
kubectl apply -f examples/k8s/seed.yaml
kubectl rollout status -n clusdr deploy/clusdr-seed
kubectl apply -f examples/k8s/daemonset.yaml
```

### 4. Join

Seed Runtime is `$(NODE_IP):7947` on the seed node (`kubectl get pod -n clusdr -o wide`).

```bash
TOKEN=$(kubectl logs -n clusdr job/clusdr-seed-init | awk '/join token/{print $NF}')
SEED="$(kubectl get pod -n clusdr -l app.kubernetes.io/component=seed -o jsonpath='{.items[0].status.hostIP}'):7947"

for p in $(kubectl get pods -n clusdr -l app.kubernetes.io/component=member -o name); do
  kubectl exec -n clusdr "$p" -- /clusdr join --token "$TOKEN" "$SEED"
done
```

On a 3-node cluster that is two joins, both voters. A 4th node: add `--observer`.

Restart of a DaemonSet pod with intact `/var/lib/clusdr` is `clusdr start` only — not another `join`.

### 5. Check

```bash
kubectl exec -n clusdr deploy/clusdr-seed -- /clusdr members
```

Three rows. Scale a workload Deployment: member count stays three.

### 6. Apps on the node

The app is not a Raft member. Keep `clusdr.Local()` (Go) / `local()`. Set `CLUSDR_GRPC_ADDR` to **this node's** Runtime, not `127.0.0.1` (that is the pod) and not a clusdr Service.

```bash
kubectl apply -f examples/k8s/app.yaml   # replace the image first
```

`app.yaml` is the same copy-paste as [from your app](../../docs/guide/from-your-app.md) and the [Operator](../../docs/guide/kubernetes-operator.md) guide: Downward API `status.hostIP` → `CLUSDR_GRPC_ADDR`, PEM hostPath or `CLUSDR_TLS=disabled` (dev). Sidecar topology still uses `Local()` on `127.0.0.1` — do not set `hostIP` there.

Full rules: [Kubernetes](../../docs/concepts/kubernetes.md).

## What these files do not do

- ClusterIP Service as the app's clusdr endpoint
- `Dial` a remote node's Runtime API as the normal app path
- Sidecar on every pod (that is the exception below, not the default)
- Growing Raft when you scale an app Deployment
- Helm that joins Raft (the [chart](../../charts/clusdr) templates these files; join stays CLI)
- Sidecar injection

## Operator (optional)

Same sequence as this README, without a human `clusdr join`. Separate image `durguto/clusdr-operator` (same tag; GHCR `ghcr.io/clusdr/clusdr-operator`).

```bash
kubectl apply -f https://clusdr.io/download/clusdr-operator-bundle.yaml
kubectl apply -f examples/k8s/clusdrcluster.yaml
# omit spec.seedNodeName — the Operator writes status.seedNodeName, then init
# sidecar instead (not next to the DaemonSet CR):
# kubectl apply -f examples/k8s/clusdrcluster-sidecar.yaml
kubectl get clusdrcluster -n clusdr   # WARNING column if both sample CRs are applied
```

Contributor: `kubectl apply -k config/crd` then `kubectl apply -k config/operator`. Local image: `docker build -f Dockerfile.operator` then `kind load`.

Crash / `kubectl delete pod` is not leave: `Members()` still has the id (`dead` then `alive`); the Operator does not `join` again.

Drop a node for good:

```bash
kubectl patch clusdrcluster clusdr -n clusdr --type merge \
  -p '{"spec":{"leave":["<clusdr-node-id>"]}}'
```

## Sidecar StatefulSet (exception)

Only when the **replica is the member**. Default remains the DaemonSet above. Do not apply this next to that DaemonSet unless you mean two clusters.

N replicas = N Raft members. `emptyDir` is forbidden (identity lives on the PVC). A Deployment sidecar is the same mistake. Distroless has no shell, so the STS template is `start` without `--bootstrap` for every ordinal. Seed ordinal 0 with a Job, then join `-1` and `-2`. Do not `clusdr init` on every pod.

Replace `spec.template.spec.containers[0].image` if you want a real app. The file ships `registry.k8s.io/pause:3.10` so the pod can become Ready. The app uses `CLUSDR_GRPC_ADDR=127.0.0.1:7947` (`Local()`). The sidecar binds `0.0.0.0:7947` and advertises headless DNS.

### 1. Namespace (if you skipped the DaemonSet walkthrough)

```bash
kubectl apply -f examples/k8s/namespace.yaml
```

### 2. Bootstrap ordinal 0

```bash
kubectl apply -f examples/k8s/sidecar-bootstrap.yaml
kubectl logs -n clusdr job/clusdr-sidecar-bootstrap -c init
```

Copy the **join token** (once). Wait until the `clusdr` container is serving (logs), then **delete the Job** so the RWO PVC is free:

```bash
kubectl delete job -n clusdr clusdr-sidecar-bootstrap
```

Do not apply the StatefulSet while the Job still holds `data-clusdr-sidecar-0`.

### 3. StatefulSet

```bash
kubectl apply -f examples/k8s/sidecar-statefulset.yaml
kubectl rollout status -n clusdr statefulset/clusdr-sidecar
```

`-0` loads Raft from the PVC (`clusdr start`, not another `--bootstrap`). `-1` and `-2` start with empty PVCs.

### 4. Join

Seed Runtime is the headless name:

```bash
TOKEN=$(kubectl logs -n clusdr job/clusdr-sidecar-bootstrap -c init | awk '/join token/{print $NF}')
SEED=clusdr-sidecar-0.clusdr-sidecar.clusdr.svc.cluster.local:7947

for o in 1 2; do
  kubectl exec -n clusdr clusdr-sidecar-$o -c clusdr -- /clusdr join --token "$TOKEN" "$SEED"
done
```

Restart of an ordinal with its PVC intact is `clusdr start` only — not another `join`.

### 5. Check

```bash
kubectl exec -n clusdr clusdr-sidecar-0 -c clusdr -- /clusdr members
```

Three rows. `kubectl scale statefulset/clusdr-sidecar --replicas=4` would be a **fourth Raft member**, not an extra app replica. Extra voters past 3: prefer `join --observer` on a new ordinal, or do not scale.

Full rules: [Sidecar](../../docs/guide/kubernetes-sidecar.md).
