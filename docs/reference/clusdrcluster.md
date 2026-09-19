# ClusdrCluster

CRD `clusdrclusters.clusdr.io`, kind `ClusdrCluster`, group `clusdr.io`, version `v1alpha1`. Namespaced. Status subresource enabled.

Desired **host** topology. `status.members` is a mirror of [`Members()`](cli/members.md) once the Operator is running. Raft is the member list. Apps still [`Local()`](../sdk/) — there is no Service field to Dial.

Install the CRD: `https://clusdr.io/download/clusdr-crds.yaml`. Apply a sample: [`clusdrcluster.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/clusdrcluster.yaml). How to run the Operator: [Operator](../guide/kubernetes-operator.md). Why two topologies: [Kubernetes](../concepts/kubernetes.md).

## spec

| Field | Type | Default | Meaning |
|---|---|---|---|
| `topology` | `DaemonSet` \| `Sidecar` | `DaemonSet` | Host layout |
| `voterCount` | odd integer ≥ 1 | (required) | Voter target. Extra DaemonSet nodes join as observers. CEL: `self % 2 == 1` |
| `image` | string | `durguto/clusdr:0.2.0` | Daemon image |
| `dataDir` | string | `/var/lib/clusdr` | Node-local `data.dir` (hostPath or PVC) |
| `seedNodeName` | string | | Kubernetes node for seed `init` + `--bootstrap` (shared hostPath) |
| `leave` | string[] | | clusdr `node.id` values to [`clusdr leave`](cli/leave.md). A missing pod is not leave |

## status

Filled by `clusdr-operator` from Runtime `Members` / Health, not from kube Ready or EndpointSlice.

| Field | Meaning |
|---|---|
| `clusterID` | Cluster id |
| `leader` | clusdr `node.id` of the Raft leader, if any |
| `members[]` | Mirror of `Members()` |
| `members[].id` | Node id |
| `members[].address` | Advertised `node.addr` (Runtime), not a ClusterIP |
| `members[].status` | `alive` \| `dead` |
| `members[].role` | `voter` \| `observer` \| `leader` |
| `observedGeneration` | Last reconciled generation |
| `phase` | `Pending` \| `Ready` \| `Error` \| `Unsupported` |
| `message` | Operator note |

`kubectl get clusdrcluster` columns: Topology, Voters, Phase, Leader, Age.

## printer / probes

The Operator uses TCP probes on port 7947. Helm may use `clusdr health`. Never `clusdr status` (Unix socket).
