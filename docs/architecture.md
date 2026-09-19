# Architecture

Clusdr is a daemon plus a local SDK, not a library you embed. The process that votes and stores the log is `clusdr start`; the application only dials that process on the same host. Embedding Raft in every app would make every crash a membership change.

Subsystems have their own [concept](concepts/) pages. Exact flags and env vars: [Configuration](reference/configuration.md).

## Process model

The **daemon** (`clusdr start`) is the cluster member. It speaks Hashicorp Raft to peers and gRPC to the local host.

The **application** is not a Raft member. It uses the SDK against the daemon on the same machine (the same split as a local Docker engine). Two apps on one host share one daemon so they see one membership list and one lock table.

```text
app A ─┐
app B ─┼─► clusdr daemon on this host ─► other clusdr daemons (Raft + gRPC)
cli   ─┘
```

Operators use the same binary: `clusdr` is daemon and CLI. A second install is not required to grow or inspect the cluster.

## Two listeners

| Listener | Default | Who uses it | Why it matters |
|---|---|---|---|
| Runtime API (TCP) | `127.0.0.1:7947` | Apps, most CLI commands, peer join / events / heartbeats | Wrong port makes `members` and `Local()` fail even when `status` looks fine. |
| Control API (Unix) | `$HOME/.clusdr/clusdr.sock` | `clusdr status` only checks that this file exists | A missing file is not a Kubernetes probe; a present file is not gRPC health. |

CLI commands other than `status` dial the **Runtime API**, not the socket. Treating `status` as “the daemon is healthy” is wrong: a process that never bound the socket looks down, and a process that bound the socket but failed gRPC looks up. Use `clusdr members` or `clusdr health`.

## What is replicated

Raft log (strong consistency; the leader is the only writer):

- Membership and leadership
- Locks and leases (including presence)

Not on the Raft log:

- Application data — Clusdr is not your database
- Custom event payloads — [events](concepts/events.md) are 1-hop gossip. A partition or a reconnect gap loses them. If you need a durable audit trail, write that elsewhere

## Data on disk

Under `data.dir` (default `$HOME/.clusdr`, or `/var/lib/clusdr` if `HOME` is unset):

| Path | Contents |
|---|---|
| `state.db` | BoltDB identity, token hash, cert material |
| `raft/` | Raft log and snapshots |
| `ca.crt`, `node.crt`, `node.key` | Cluster CA and this node's cert |

Two processes must not share one `data.dir`. BoltDB takes a flock; the second process fails to open the store. Give each daemon its own directory or you cannot form a two-node laptop cluster.

## Addresses that must be reachable

| Port / path | Default | Must be reachable by |
|---|---|---|
| Runtime gRPC | `127.0.0.1:7947` | Local apps; **peers** if you advertise it |
| Raft | `127.0.0.1:7946` | Every Raft peer |
| Control socket | `$HOME/.clusdr/clusdr.sock` | Local `status` only |

`node.addr` is what membership stores for this node. If it is `0.0.0.0:7947`, peers cannot dial you and `join` appears to hang or fail after the token check. Set a host:port they can use ([Run on other hosts](guide/other-hosts.md)).

The seed node's self-membership is written with `grpc.addr` when it becomes leader. If `grpc.addr` is loopback and you later expect another machine to join, change the file before peers exist, or they will store an address they cannot reach.

## Size

Quorum is majority. One voter: the node dies, the cluster is gone. Two voters: one death loses majority. Three voters tolerate one failure. Prefer odd voter counts so a single loss cannot tie.

[Observers](concepts/observers.md) receive the log but do not vote, so a metrics host or a fourth rack does not change failover.

## Next

- [Daemon and application](concepts/daemon.md)
- [Consistency](concepts/consistency.md)
- [Run on other hosts](guide/other-hosts.md)
- [Kubernetes](concepts/kubernetes.md)
