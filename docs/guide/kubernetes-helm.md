# Install with Helm

Helm templates the DaemonSet topology from chart values. Join stays CLI — Helm does not call `clusdr join`. It does not install the CRD or the Operator. If you want automatic join, use [Install the Operator](kubernetes-operator.md) instead.

Why that split exists: [Kubernetes](../concepts/kubernetes.md). Same topology by hand: [Run on Kubernetes](kubernetes.md).

| | Value | Why it matters |
|---|---|---|
| Chart | `oci://ghcr.io/clusdr/charts/clusdr` | There is no `helm repo add`; a repo URL will not resolve this OCI chart. |
| Catalog | [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr) | Browse values and versions; install still uses the OCI URL above. |
| Schema | `values.schema.json` | `helm template … --set voterCount=4` fails before you deploy an even quorum. |
| Signed | Cosign on the OCI digest ([verify](verify-release.md)) | Artifact Hub Signed needs a tag cut after signing landed; `0.2.0` is checksum-only. |

Official is not a chart file. Request it from Artifact Hub after Verified publisher ([template](https://github.com/artifacthub/hub/issues/new?template=official-status.yml)).

## 1. Give each node a disk the daemon can keep

Skip this and the init Job fails with `Permission denied` on `/var/lib/clusdr` (distroless uid **65532**, hostPath is not chowned by `fsGroup`).

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
  --set seed.nodeName=<name-from-get-nodes>
```

`seed.nodeName` must be a real node. If you omit it, init and `--bootstrap` can land on different nodes and will not share hostPath.

`ghcr.io/clusdr/clusdr` is the daemon image. The chart is `oci://ghcr.io/clusdr/charts/clusdr`. Pushing the chart to the daemon repository overwrites the image.

Contributor checkout: `helm install clusdr charts/clusdr --namespace clusdr --create-namespace --set seed.nodeName=<node>`.

`voterCount` must be odd. `helm template … --set voterCount=4` fails at template time so you do not deploy an even quorum.

## 3. Join

NOTES after install print the hostPath loop and the join hint. The token is in the init hook logs:

```bash
TOKEN=$(kubectl logs -n clusdr job/clusdr-seed-init | awk '/join token/{print $NF}')
SEED="$(kubectl get pod -n clusdr -l app.kubernetes.io/component=seed -o jsonpath='{.items[0].status.hostIP}'):7947"

for p in $(kubectl get pods -n clusdr -l app.kubernetes.io/component=member -o name); do
  kubectl exec -n clusdr "$p" -- /clusdr join --token "$TOKEN" "$SEED"
done
```

Join DaemonSet pods as **voters** until `voterCount` (default 3; the seed is already 1). Extra nodes: add `--observer`. A restart with intact `dataDir` is `clusdr start`, not another `join`.

`UNAUTHORIZED`: [Errors](../reference/errors.md#join).

## Checkpoint

```bash
kubectl exec -n clusdr deploy/clusdr-seed -- /clusdr members
```

An odd number of `alive` voters. Scaling a workload Deployment does not add rows — that is not a bug.

| Value | Meaning |
|---|---|
| `seed.nodeName` | Same node for init Job and seed Deployment |
| `voterCount` | Odd target. Helm does not join |
| `image.tag` | Empty uses `appVersion` |
| `dataDir` | hostPath (`/var/lib/clusdr`) |
| `probes.type` | `exec` (`clusdr health`) or `tcp` (7947). Never `clusdr status` |

Chart README: [`charts/clusdr`](https://github.com/clusdr/clusdr/blob/main/charts/clusdr/README.md).
