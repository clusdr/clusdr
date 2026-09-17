# Architecture

This page is the technical overview. Subsystems have their own concept pages.

## Process model

The **daemon** (`clusdr start`) is the cluster member. It speaks Hashicorp Raft to peers and gRPC to the local host.

The **application** is not a Raft member. It uses the SDK against the daemon on the same machine (Docker-style). Two apps on one host share one daemon.

```text
app A ─┐
app B ─┼─► clusdr daemon on this host ─► other clusdr daemons (Raft + gRPC)
cli   ─┘
```

Operators use the same binary: `clusdr` is daemon and CLI.

## Two listeners

| Listener | Default | Who uses it |
|---|---|---|
| Runtime API (TCP) | `127.0.0.1:7947` | Apps, most CLI commands, peer join / events / heartbeats |
| Control API (Unix) | `$HOME/.clusdr/clusdr.sock` | `clusdr status` only checks that this file exists |

CLI commands other than `status` dial the **Runtime API**, not the socket.

## What is replicated

Raft log (strong consistency, leader is the only writer):

- Membership and leadership
- Locks and leases (including presence)

Not on the Raft log:

- Application data
- Custom event payloads ([events](concepts/events.md) are 1-hop gossip)

## Data on disk

Under `data.dir` (default `$HOME/.clusdr`, or `/var/lib/clusdr` if `HOME` is unset):

| Path | Contents |
|---|---|
| `state.db` | BoltDB identity, token hash, cert material |
| `raft/` | Raft log and snapshots |
| `ca.crt`, `node.crt`, `node.key` | Cluster CA and this node's cert |

Two processes must not share one `data.dir` (BoltDB flock).

## Addresses that must be reachable

| Port / path | Default | Must be reachable by |
|---|---|---|
| Runtime gRPC | `127.0.0.1:7947` | Local apps; **peers** if you advertise it |
| Raft | `127.0.0.1:7946` | Every Raft peer |
| Control socket | `$HOME/.clusdr/clusdr.sock` | Local `status` only |

`node.addr` is what membership stores for this node. If it is `0.0.0.0:7947`, peers cannot dial you. Set a host:port they can use.

The seed node's self-membership is written with `grpc.addr` when it becomes leader.

## Size

Quorum is majority. Three voters tolerate one failure. Prefer odd voter counts.

[Observers](concepts/observers.md) receive the log but do not vote.

## Next

- [Daemon and application](concepts/daemon.md)
- [Consistency](concepts/consistency.md)
- [Run on other hosts](guide/other-hosts.md)
- [Run on Kubernetes](guide/kubernetes.md)
- [Helm](guide/kubernetes-helm.md)
- [Operator](guide/kubernetes-operator.md)
