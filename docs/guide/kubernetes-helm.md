# Helm

The chart templates the [default topology](kubernetes.md#default-one-daemon-per-node): one daemon per node, `data.dir` on hostPath. It does not join Raft, does not install the CRD, and does not run the Operator.

Install is OCI. There is no `helm repo add`. Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr).

```bash
helm install clusdr oci://ghcr.io/clusdr/charts/clusdr --version 0.2.0 \
  --namespace clusdr --create-namespace \
  --set seed.nodeName=<node>
```

`ghcr.io/clusdr/clusdr` is the **daemon image**, not this chart. The chart lives at `oci://ghcr.io/clusdr/charts/clusdr`.

Contributor checkout: `helm install clusdr charts/clusdr --namespace clusdr --create-namespace --set seed.nodeName=<node>`. In-tree chart: [`charts/clusdr`](https://github.com/clusdr/clusdr/tree/main/charts/clusdr). Checkout only if you are changing the chart.

## After install

hostPath still needs `mkdir` + `chown 65532` on each node. NOTES print the loop. Copy the join token from the init hook logs, then join DaemonSet pods as **voters** until `voterCount` (default 3; the seed is already 1). Extra nodes: `join --observer`.

Helm does not `init` DaemonSet pods and does not `--bootstrap` every ordinal. A restart with intact `dataDir` is `clusdr start`, not another `join`. Scaling a workload Deployment does not add Raft members.

## Values that matter

| Value | Meaning |
|---|---|
| `image.repository` | Published `durguto/clusdr` |
| `image.tag` | Empty uses `appVersion` (the daemon tag). Override to pin |
| `dataDir` | hostPath on the node (`/var/lib/clusdr`) |
| `voterCount` | Odd target. Helm does not join |
| `seed.nodeName` | Same node for init Job and seed Deployment |
| `probes.type` | `exec` (`clusdr health`) or `tcp` (7947) |

`voterCount` must be odd (`helm template … --set voterCount=4` fails).

The init Job is `pre-install,pre-upgrade` so the seed does not start on an empty disk. `clusdr init` is success when identity already exists.

## What this chart is not

- Join token, Raft add/remove, or a second control plane
- Sidecar as the default — that is [Sidecar](kubernetes-sidecar.md) (YAML / Operator, not this chart)
- The [Operator](kubernetes-operator.md) or the `ClusdrCluster` CRD
- `helm repo add`

Same topology without Helm: [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s). Chart README: [`charts/clusdr`](https://github.com/clusdr/clusdr/blob/main/charts/clusdr/README.md).
