# Daemon and application

The **daemon** is the cluster member. The **application** is a client of the daemon on the same host.

This is the same split as a local Docker engine: apps do not join the cluster; the daemon does.

```text
app A ─┐
app B ─┼─► daemon on this host ─► other daemons (Raft + gRPC)
cli   ─┘
```

## Daemon

Started with `clusdr start`. It:

- Votes (or observes) in Raft
- Serves the Runtime API on TCP
- Optionally binds the Unix control socket
- Holds locks, leases, and the membership table in RAM; Raft is the writer

If `clusdr init` was never run, the process can still start and logs that identity is missing.

## Application

Uses `clusdr.Local()` (Go) or `clusdr.local()` (Python). It dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`.

It is not a voter. It does not speak Raft. Two processes on one machine share the daemon.

`Dial(addr)` exists for tests and operators. It is not the default app path.

## CLI

`clusdr` is one binary. Operator commands (`members`, `join`, `watch`, …) dial the Runtime API. Only [`status`](../reference/cli/status.md) looks at the Unix socket.

## Related

- [Architecture](../architecture.md)
- [Start the first member](../guide/first-member.md)
- [SDKs](../sdk/README.md)
