# Run on Kubernetes

Kubernetes is a place to run the same Linux host model. It is not a Kubernetes replacement, not etcd for the apiserver, and not a substitute for kube’s own coordination.

If you have not formed a cluster on Linux yet, start at [other hosts](other-hosts.md). Example manifests: [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s).

| You want | Page |
|---|---|
| One daemon per node (the default) | this page |
| Template that DaemonSet from values | [Helm](kubernetes-helm.md) |
| A `ClusdrCluster` object that forms the same topology | [Operator](kubernetes-operator.md) |
| The replica **is** the Raft member | [Sidecar](kubernetes-sidecar.md) |

## What Kubernetes already does

Use kube for kube’s objects.

| Need | Use this |
|---|---|
| Controller leader election | [Lease API](https://kubernetes.io/docs/concepts/architecture/leases/) (`coordination.k8s.io`) |
| Container liveness / readiness | probes |
| Who is a replica of this Deployment | Service / EndpointSlice |
| Watch kube objects | informers / client-go |

A three-replica Go controller that only needs “who leads this Deployment” should keep using the Lease API. Clusdr does not replace that.

## What clusdr is for

Application coordination the apiserver should not hold:

- Named [locks](../concepts/locks.md) and [leases](../concepts/leases.md) with fencing tokens (business keys, not pod names)
- Mixed members: VM + pod + agent on one membership list
- [Watch](../concepts/watch.md) and membership from SDKs that are not client-go

The daemon is still the Raft member. The app is still not.

```text
Node
├── Application pods     →  local SDK  →  this node’s daemon
└── clusdr daemon        →  Raft to other nodes’ daemons
```

## Default: one daemon per node

Same as a Linux host. One clusdr process per node. Odd **voter** count. Extra nodes [observers](../concepts/observers.md). Scaling a Deployment does **not** add Raft members.

`data.dir` lives on the node (hostPath or another node-local volume) so a DaemonSet pod restart is [`clusdr start`](../concepts/presence.md) with the same identity — not another `join`. Crash is `member.dead`; `clusdr leave` is `member.left`.

Advertise dialable `node.addr` / `raft.addr` (node IP or DNS), not `0.0.0.0`. Health and readiness use the Runtime [Health](../reference/api/health.md) RPC, not `clusdr status` on the Unix socket.

Bootstrap and join are the same CLI as on a VM. Three ways to get this topology:

- YAML under [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s)
- [Helm](kubernetes-helm.md) (`oci://ghcr.io/clusdr/charts/clusdr`) — join stays CLI
- [Operator](kubernetes-operator.md) — a `ClusdrCluster` object, same `init` / `--bootstrap` / `join`

Walkthrough (kind / k3s, hostPath, probes): the [examples/k8s README](https://github.com/clusdr/clusdr/blob/main/examples/k8s/README.md).

## Apps on the node

The SDK does not change. `Local()` / `local()` still dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`. There is no kube SDK and no `Dial` to a clusdr Service.

A pod is **not** the Linux host. `127.0.0.1:7947` inside the pod is that pod, not the node daemon — unless the app shares the node's netns (`hostNetwork`) or is a [sidecar](kubernetes-sidecar.md).

The example daemons use `hostNetwork`, so the Runtime is `$(NODE_IP):7947` on that node. Point the app there with the Downward API:

```yaml
env:
  - name: NODE_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
  - name: CLUSDR_GRPC_ADDR
    value: "$(NODE_IP):7947"
```

Same address if the daemon used `hostPort: 7947` instead of `hostNetwork`. Copy-paste Deployment: [`examples/k8s/app.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/app.yaml).

Go (one language is enough). `Local()` is unchanged; bootstrap TLS is enough when the data dir has no PEMs:

```go
c, err := clusdr.Local()
if err != nil {
    // CLUSDR_GRPC_ADDR down, TLS, or Health not ready within 10s
}
defer c.Close()
members, err := c.Members(ctx)
```

Python, Rust, TypeScript, and Java need PEMs (`CLUSDR_DATA_DIR` — the node's `data.dir`, often the same hostPath read-only) or `CLUSDR_TLS=disabled` on **both** daemon and app.

Do not create a ClusterIP/headless Service of clusdr and `Dial` it from every replica. That is Consul/etcd, and it is not `Local()`.

## Sidecar is the exception

Only when that **replica is the Raft member** (a small elected StatefulSet). Shared netns → `127.0.0.1` / `Local()`. N replicas = N members. Full page: [Sidecar](kubernetes-sidecar.md).

Do not put a clusdr sidecar on every microservice pod. Default remains [one daemon per node](#default-one-daemon-per-node).

## Helm, CRD, Operator

Packaging of this page. Optional. The daemon and the SDKs do not require kube.

| Layer | Job | Page |
|---|---|---|
| **Helm** | Template the DaemonSet from values. Join is still CLI | [Helm](kubernetes-helm.md) |
| **CRD** | One cluster object: desired **host** topology | [Operator](kubernetes-operator.md) |
| **Operator** | Reconcile that object with the same CLI (`init`, `--bootstrap`, `join`) | [Operator](kubernetes-operator.md) |

## Not this

- Replacing `coordination.k8s.io`, probes, or EndpointSlice
- etcd storage for the kube-apiserver
- A ClusterIP Service as the app’s clusdr endpoint (that is Consul/etcd)
- An Operator that grows Raft when you scale a Deployment
- A clusdr sidecar on a Deployment (or `emptyDir`) as the general app pattern
- A mutating webhook that injects a clusdr sidecar on every pod

## Related

- [Run on other hosts](other-hosts.md)
- [Helm](kubernetes-helm.md)
- [Operator](kubernetes-operator.md)
- [Sidecar](kubernetes-sidecar.md)
- [Presence](../concepts/presence.md)
- [Observers](../concepts/observers.md)
- [Use it from your app](from-your-app.md)
- [Limits](../reference/limits.md)
- [Compatibility](../reference/compatibility.md)
- [Examples: Kubernetes](https://github.com/clusdr/clusdr/tree/main/examples/k8s)
