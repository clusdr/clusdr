# Daemon and application

clusdr splits the cluster member from the program that uses it. The **daemon** (`clusdr start`) is the Raft member: it votes, stores the log, and serves the Runtime API. The **application** only calls that daemon on the same host. If you treat the app as a member, you will try to join from a laptop or a pod and wonder why Raft never sees it.

This is the same split as a local Docker engine: containers do not join the Swarm; the engine does.

```text
app A ─┐
app B ─┼─► clusdr daemon on this host ─► other clusdr daemons (Raft + gRPC)
cli   ─┘
```

## Daemon

Started with `clusdr start`. It:

- Votes (or observes) in Raft
- Serves the Runtime API on TCP so apps, the CLI, and peers can talk to this member
- Optionally binds the Unix control socket used only by `clusdr status`
- Holds locks, leases, and the membership table in RAM; Raft is the writer so a crash does not invent a second source of truth

If `clusdr init` was never run, the process can still start and logs that identity is missing. It is not a cluster until you `init` and, on the seed, `start --bootstrap` — [first member](../guide/first-member.md).

## Application

Uses `clusdr.Local()` (Go) or `clusdr.local()` (Python). It dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`. A Kubernetes pod is not the host: `127.0.0.1` inside the pod is that pod, so set `CLUSDR_GRPC_ADDR` to the node's Runtime — unless the app is a [sidecar](../guide/kubernetes-sidecar.md) in the same pod. [Kubernetes](kubernetes.md).

The app is not a voter. It does not speak Raft. Two processes on one machine share the daemon so they see the same members, locks, and Watch stream.

`Dial(addr)` exists for tests and operators (a second daemon on another port). It is not the default app path — putting every replica on a remote Runtime is Consul/etcd, not `Local()`.

## CLI

`clusdr` is one binary. Operator commands (`members`, `join`, `watch`, …) dial the Runtime API. Only [`status`](../reference/cli/status.md) looks at the Unix socket, so a missing socket makes `status` fail even when `members` would succeed.

## Related

- [Architecture](../architecture.md)
- [Start the first member](../guide/first-member.md)
- [Kubernetes](kubernetes.md)
- [SDKs](../sdk/)
