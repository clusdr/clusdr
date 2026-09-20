# ClusdrCluster

`ClusdrCluster` is the Kubernetes object that declares desired **host** topology for a clusdr cluster. Applying the CRD or a sample YAML without the Operator leaves spec only — Raft is still the member list, and apps still [`Local()`](../sdk/). There is no Service field to Dial.

| | Value | Why it matters |
|---|---|---|
| Group / kind | `clusdr.io` / `ClusdrCluster` | `kubectl get clusdrcluster` is unknown until the CRD is installed. |
| Resource | `clusdrclusters.clusdr.io` | Namespaced. |
| Version | `v1alpha1` | Wire shape can still change; pin operator and CRD to the same tag. |
| Status | subresource enabled | `status.members` is filled by the Operator from Runtime `Members()`, not from kube Ready. |

`status.members` is a mirror of [`Members()`](cli/members.md) once the Operator is running. It is not kube Ready and not EndpointSlice.

Install the Operator (CRD included): `https://clusdr.io/download/clusdr-operator-bundle.yaml`. Apply a sample: [`clusdrcluster.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster.yaml). How to run the Operator: [Operator](../guide/kubernetes-operator.md). Why two topologies: [Kubernetes](../concepts/kubernetes.md).

## spec

| Field | Type | Default | Meaning |
|---|---|---|---|
| `topology` | `DaemonSet` \| `Sidecar` | `DaemonSet` | Host layout |
| `voterCount` | odd integer ≥ 1 | (required) | Voter target. Extra DaemonSet nodes join as observers. CEL: `self % 2 == 1` |
| `image` | string | `durguto/clusdr:0.2.0` | Daemon image |
| `dataDir` | string | `/var/lib/clusdr` | Node-local `data.dir` (hostPath or PVC) |
| `seedNodeName` | string | | Optional pin. Empty: the Operator picks a Ready node, writes `status.seedNodeName`, then starts init + `--bootstrap` on that node |
| `leave` | string[] | | clusdr `node.id` values to [`clusdr leave`](cli/leave.md). A missing pod is not leave |

`spec.leave` is the only RemoveServer path the Operator will take. A crashed, drained, evicted, or PreStop’d pod keeps its Raft id; restart is `clusdr start` with the same `data.dir`, not another `join`. Future leave automation (not shipped) is a **deleted Node object** only — not PreStop.

## status

Filled by `clusdr-operator` from Runtime `Members` / Health, not from kube Ready or EndpointSlice.

| Field | Meaning |
|---|---|
| `clusterID` | Cluster id |
| `seedNodeName` | Node pinned for init + `--bootstrap`. Written before the Job. The CR `resourceVersion` is the lock (not a kube Lease) |
| `leader` | clusdr `node.id` of the Raft leader, if any |
| `members[]` | Mirror of `Members()` |
| `members[].id` | Node id |
| `members[].address` | Advertised `node.addr` (Runtime), not a ClusterIP |
| `members[].status` | `alive` \| `dead` |
| `members[].role` | `voter` \| `observer` \| `leader` |
| `observedGeneration` | Last reconciled generation |
| `phase` | `Pending` \| `Ready` \| `Error` \| `Unsupported` |
| `message` | Operator note |
| `warning` | Short printer-column string. Two or more `ClusdrCluster` objects → `two clusters` (two Raft groups). Not a webhook |

`kubectl get clusdrcluster` columns: Topology, Voters, Seed, Phase, Leader, WARNING, Age.

## printer / probes

The Operator uses TCP probes on port 7947. Helm may use `clusdr health`. Never `clusdr status` (Unix socket) — that file check is not a Kubernetes probe ([errors](errors.md#daemon-and-dial)).
