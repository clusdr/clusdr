# Kubernetes

Kubernetes is a place to run the same Linux host model you already ran on VMs. It is not a Kubernetes replacement, not etcd for the apiserver, and not a substitute for kube’s own coordination. If you treat clusdr as “the cluster for kube objects,” you will fight Lease, probes, and EndpointSlice instead of using them.

The daemon is still the Raft member. The application is still not. `Local()` / `local()` still means “the daemon on **this host**.”

```text
Node
├── Application pods     →  local SDK  →  this node’s daemon
└── clusdr daemon        →  Raft to other nodes’ daemons
```

## What Kubernetes already does

Use kube for kube’s objects.

| Need | Use this |
|---|---|
| Controller leader election | [Lease API](https://kubernetes.io/docs/concepts/architecture/leases/) (`coordination.k8s.io`) |
| Container liveness / readiness | probes |
| Who is a replica of this Deployment | Service / EndpointSlice |
| Watch kube objects | informers / client-go |

A three-replica Go controller that only needs “who leads this Deployment” should keep using the Lease API.

## Two topologies

**DaemonSet (default).** One daemon per node. Odd voter count. Extra nodes are [observers](observers.md). `data.dir` is node-local (hostPath). A pod restart with that disk is [`clusdr start`](presence.md), not another `join`. Scaling a workload Deployment does **not** add Raft members.

Apps are not on the node’s loopback. `127.0.0.1:7947` inside a pod is that pod. Point the app at the node Runtime (`status.hostIP:7947`) with the Downward API, or put the app on `hostNetwork`. Do not create a ClusterIP of clusdr and `Dial` it from every replica. That is Consul/etcd, and it is not `Local()`.

**Sidecar (exception).** Only when that **replica is the Raft member** (a small elected StatefulSet). Shared netns → `127.0.0.1` / `Local()`. N replicas = N members. Headless DNS is the advertised peer address. The Operator may Dial that name to `join`; the app still uses `Local()`.

Do not put a clusdr sidecar on every microservice pod — that grows Raft with every deploy replica.

## Helm vs Operator

Packaging of the same host model. The daemon and the SDKs do not require kube.

| Layer | Job |
|---|---|
| **Helm** | Template the DaemonSet. Join stays CLI so you see the token and the seed address. **Never** carries CRDs |
| **CRD** | Desired **host** topology. Raft stays the member list. Installed with the Operator bundle, not the chart |
| **Operator** | Same CLI in-cluster: writes `status.seedNodeName` then `init`, one `--bootstrap`, `join` / `join --observer`. Token is a Secret. `leave` is only `spec.leave` |

Helm does not form Raft. The Operator does. A crash or missing pod is not leave — do not patch `spec.leave` to “fix” a bounce.

The DaemonSet chart **never** carries CRDs. Permanent architecture, same split as cert-manager and prometheus-operator: Helm’s CRD lifecycle drifts on upgrade and delete. Not “we will fold them in later.” A Helm chart of the Operator is also not this product.

## Crash, drain, leave

Cordon, drain, eviction, and a pod PreStop are a **reboot**: `member.dead`, disk stays, [`clusdr start`](presence.md). They are not `leave`. Today the only RemoveServer path is `spec.leave` / [`clusdr leave`](../reference/cli/leave.md). A later automation (not shipped) may `leave` only when the Kubernetes **Node object is deleted** (autoscaler decommission, `kubectl delete node`). PreStop must not call `leave`.

## Not this

- Replacing `coordination.k8s.io`, probes, or EndpointSlice
- etcd storage for the kube-apiserver
- A ClusterIP Service as the app’s clusdr endpoint
- Growing Raft when you scale a Deployment
- Sidecar injection / a mutating webhook on every pod
- Treating StatefulSet ordinals as the default DaemonSet seed (members are **nodes**)
- CRDs inside the DaemonSet Helm chart
- PreStop / eviction / drain → `clusdr leave`

How to apply YAML, Helm, the Operator, or the sidecar: [Run on Kubernetes](../guide/kubernetes.md) and the sibling how-to pages.
