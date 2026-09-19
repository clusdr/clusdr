# Install with Helm

Template the DaemonSet topology from values. Join stays CLI. The chart does not install the CRD or the Operator.

Why Helm is not the Operator: [Kubernetes](../concepts/kubernetes.md). Same topology by hand: [Run on Kubernetes](kubernetes.md).

Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr). There is no `helm repo add`.

## 1. Data dir on each node

```bash
# kind, cluster name clusdr
for n in $(kind get nodes --name clusdr); do
  docker exec "$n" mkdir -p /var/lib/clusdr
  docker exec "$n" chown 65532:65532 /var/lib/clusdr
done
```

## 2. Install

```bash
kubectl get nodes
helm install clusdr oci://ghcr.io/clusdr/charts/clusdr --version 0.2.0 \
  --namespace clusdr --create-namespace \
  --set seed.nodeName=<node>
```

`ghcr.io/clusdr/clusdr` is the daemon image. The chart is `oci://ghcr.io/clusdr/charts/clusdr`.

Contributor checkout: `helm install clusdr charts/clusdr --namespace clusdr --create-namespace --set seed.nodeName=<node>`.

## 3. Join

Copy the join token from the init hook logs (NOTES print the loop). Join DaemonSet pods as **voters** until `voterCount` (default 3; the seed is already 1). Extra nodes: `join --observer`.

```bash
SEED=<seed-node-ip>:7947
TOKEN=<token-from-init>
for p in $(kubectl get pods -n clusdr -l app.kubernetes.io/component=member -o name); do
  kubectl exec -n clusdr "$p" -- /clusdr join --token "$TOKEN" "$SEED"
done
```

## Checkpoint

`kubectl exec -n clusdr deploy/clusdr-seed -- /clusdr members` shows an odd number of voters. `helm template … --set voterCount=4` fails.

| Value | Meaning |
|---|---|
| `seed.nodeName` | Same node for init Job and seed Deployment |
| `voterCount` | Odd target. Helm does not join |
| `image.tag` | Empty uses `appVersion` |
| `dataDir` | hostPath (`/var/lib/clusdr`) |
| `probes.type` | `exec` (`clusdr health`) or `tcp` (7947) |

Chart README: [`charts/clusdr`](https://github.com/clusdr/clusdr/blob/main/charts/clusdr/README.md). Automatic join: [Operator](kubernetes-operator.md).
