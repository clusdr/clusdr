# Run on Kubernetes

You are forming the same cluster as the [tutorial](./): one Raft member per Linux host, apps talking only to the daemon on that host. Kubernetes here is the place those hosts live — not a replacement for kube Lease, probes, or etcd. Why that split exists: [Kubernetes](../concepts/kubernetes.md).

On a three-node cluster you get three voters: one **seed** (init + `start --bootstrap` on a single node) and one daemon on each of the other nodes (a DaemonSet). Your app Deployments are still clients. Scaling them does not add Raft members.

```text
Node A (seed)     Node B              Node C
clusdr daemon     clusdr daemon       clusdr daemon
      └── Raft ───────┴──────────────────┘
app pods on A     app pods on B       app pods on C
  → this node’s daemon (not a ClusterIP of clusdr)
```

Helm templates this layout ([Install with Helm](kubernetes-helm.md)). The Operator runs `init` / `join` for you ([Install the Operator](kubernetes-operator.md)). This page is the hand-applied YAML so you can see each step.

You need `kubectl` against a cluster with at least three nodes (kind name `clusdr` in the snippets). Manifests: [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s). Image pin in those files is `durguto/clusdr:0.2.0`.

## 1. Give each node a disk the daemon can keep

A member’s identity lives on disk (`data.dir`). A pod restart with that disk is `clusdr start`, not another `join` — the same rule as a VM reboot ([presence](../concepts/presence.md)). On Kubernetes that disk is a **hostPath** on the node (`/var/lib/clusdr`), not an emptyDir and not a PVC shared across nodes.

The image is distroless and runs as uid **65532**. hostPath is not chowned by `fsGroup`. If you skip the next loop, the seed Job fails with `Permission denied` writing `/var/lib/clusdr`.

```bash
# kind, cluster name clusdr
for n in $(kind get nodes --name clusdr); do
  docker exec "$n" mkdir -p /var/lib/clusdr
  docker exec "$n" chown 65532:65532 /var/lib/clusdr
done
```

On k3s or real nodes: the same `mkdir` + `chown` once per machine, as root.

## 2. Put init and bootstrap on the same node

`clusdr init` writes the join token and CA onto that node’s hostPath. `clusdr start --bootstrap` must read the **same** directory. If the Job and the seed Deployment land on different nodes, init writes disk A and bootstrap starts empty disk B — join will never see that identity.

```bash
kubectl get nodes
```

Set `spec.template.spec.nodeName` on both [`seed-init.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/seed-init.yaml) and [`seed.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/seed.yaml) to the **same** name from that list.

## 3. Create the seed, then start a daemon on every other node

Same sequence as the laptop tutorial: identity first, then `--bootstrap` on the seed, then start joiners (they are not in Raft until step 4). Run these from a checkout that contains `examples/k8s/`, or pass the raw GitHub URLs.

```bash
kubectl apply -f examples/k8s/namespace.yaml
kubectl apply -f examples/k8s/seed-init.yaml
kubectl wait -n clusdr --for=condition=complete job/clusdr-seed-init --timeout=60s
kubectl logs -n clusdr job/clusdr-seed-init
```

Copy the `join token :` line (once). Then:

```bash
kubectl apply -f examples/k8s/seed.yaml
kubectl rollout status -n clusdr deploy/clusdr-seed
kubectl apply -f examples/k8s/daemonset.yaml
```

If the Job is not complete, `wait` times out — check `kubectl describe job -n clusdr clusdr-seed-init` (almost always the hostPath chown or `nodeName`).

## 4. Join the other nodes

The DaemonSet pods are running but not yet members — same as `clusdr start` without `join` on a second laptop process. Dial the seed’s **node IP** on 7947 (Runtime), not a Service.

```bash
TOKEN=$(kubectl logs -n clusdr job/clusdr-seed-init | awk '/join token/{print $NF}')
SEED="$(kubectl get pod -n clusdr -l app.kubernetes.io/component=seed -o jsonpath='{.items[0].status.hostIP}'):7947"
echo "SEED=$SEED"

for p in $(kubectl get pods -n clusdr -l app.kubernetes.io/component=member -o name); do
  kubectl exec -n clusdr "$p" -- /clusdr join --token "$TOKEN" "$SEED"
done
```

On a 3-node cluster that is two joins, both voters. A later extra node: add `--observer` so you do not grow quorum by accident. Empty `TOKEN` means the Job logs were already rotated — re-run init only if you mean a new cluster.

`UNAUTHORIZED` means the token does not match that seed ([Errors](../reference/errors.md#join)).

## 5. Point an app at the node Runtime

A pod’s `127.0.0.1:7947` is that pod, not the node daemon. Set the node Runtime or the SDK dials a closed port:

```yaml
env:
  - name: NODE_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
  - name: CLUSDR_GRPC_ADDR
    value: "$(NODE_IP):7947"
```

Copy-paste Deployment: [`app.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/app.yaml). Python, Rust, TypeScript, and Java need PEMs from the node’s `data.dir` (same hostPath, read-only) or `CLUSDR_TLS=disabled` on **both** daemon and app — mixed TLS fails the handshake.

Probes: `clusdr health` or `tcpSocket` port 7947. Never `clusdr status` (Unix socket is not where the probe runs).

## Checkpoint

```bash
kubectl exec -n clusdr deploy/clusdr-seed -- /clusdr members
```

Three `alive` voters. Delete the seed pod with intact hostPath: the same id returns (`dead`, then `alive`). If you get a new id, the disk was empty and you formed a second cluster.

Helm: [Install with Helm](kubernetes-helm.md). Automatic join: [Install the Operator](kubernetes-operator.md). Replica-is-member: [Run a sidecar StatefulSet](kubernetes-sidecar.md).
