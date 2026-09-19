# Run on Kubernetes

Run the default topology: one daemon per node, `data.dir` on hostPath. You want **3 members** (seed + two DaemonSet voters).

Why this is not a kube replacement, and when to use the sidecar instead: [Kubernetes](../concepts/kubernetes.md). Helm and Operator are sibling how-tos.

Manifests: [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s). Image pin in those files is `durguto/clusdr:0.2.0`.

## 1. Data dir on each node

hostPath is not chowned by `fsGroup`. Distroless uid **65532**.

```bash
# kind, cluster name clusdr
for n in $(kind get nodes --name clusdr); do
  docker exec "$n" mkdir -p /var/lib/clusdr
  docker exec "$n" chown 65532:65532 /var/lib/clusdr
done
```

On k3s or real nodes: the same `mkdir` + `chown` once per machine.

## 2. Pin init and seed to one node

```bash
kubectl get nodes
```

Set `spec.template.spec.nodeName` on both [`seed-init.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/seed-init.yaml) and [`seed.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/seed.yaml) to that name. They must share one hostPath.

## 3. Apply seed, then the DaemonSet

```bash
kubectl apply -f examples/k8s/namespace.yaml
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

## 4. Join DaemonSet pods as voters

Seed Runtime is `$(NODE_IP):7947` on the seed node (`kubectl get pod -n clusdr -o wide`).

```bash
SEED=<seed-node-ip>:7947
TOKEN=<token-from-init>

for p in $(kubectl get pods -n clusdr -l app.kubernetes.io/component=member -o name); do
  kubectl exec -n clusdr "$p" -- /clusdr join --token "$TOKEN" "$SEED"
done
```

On a 3-node cluster that is two joins, both voters. A later extra node: add `--observer`.

## 5. Point an app at the node Runtime

```yaml
env:
  - name: NODE_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
  - name: CLUSDR_GRPC_ADDR
    value: "$(NODE_IP):7947"
```

Copy-paste Deployment: [`app.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/app.yaml). Python, Rust, TypeScript, and Java need PEMs from the node's `data.dir` or `CLUSDR_TLS=disabled` on **both** daemon and app.

Probes: `clusdr health` or `tcpSocket` port 7947. Never `clusdr status`.

## Checkpoint

```bash
kubectl exec -n clusdr deploy/clusdr-seed -- /clusdr members
```

Three `alive` voters. A seed pod delete with intact hostPath must keep the same id (dead, then alive) — not a new `join`.

Helm: [Helm](kubernetes-helm.md). Operator (join is automatic): [Operator](kubernetes-operator.md). Replica-is-member: [Sidecar](kubernetes-sidecar.md).
