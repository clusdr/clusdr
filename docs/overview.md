# Overview

Clusdr answers four questions for applications on a cluster:

- Who is in the cluster?
- Who is the leader?
- Who is still alive?
- What just changed?

It does that without being a general-purpose coordination suite.

## Use it when

- Agents or services need membership and a watch stream
- Services need leader election without standing up etcd or Consul as a product
- A node dying should produce `member.left` without a graceful leave ([presence](concepts/presence.md))
- You need exclusive [locks](concepts/locks.md) or named TTL [leases](concepts/leases.md) with fencing tokens
- Extra hosts should see state without changing quorum ([observers](concepts/observers.md))
- You need small signals (`publish` → `custom.<topic>`), not a durable log

## Do not use it when

- You need to store application state or files
- You need at-least-once or durable messaging (custom events are not on the Raft log)
- You need multi-cluster federation (not in this version)
- You want to replace etcd as Kubernetes storage

Clusdr is not a database, queue, workflow engine, service mesh, or Kubernetes.

## Versus common alternatives

| Need | Clusdr | Typical alternative |
|---|---|---|
| Who is alive + who leads | Primary job | etcd or Consul plus extra wiring |
| Distributed lock / lease | Raft lock and lease APIs | etcd lock, Consul session |
| KV, DNS catalog, ACLs | Not provided | etcd, Consul |
| Durable log of user events | Not provided | a queue, or another system's Raft log |

If you already run etcd for KV, adding Clusdr only for membership is optional.

Boundaries of this version: [Limits](reference/limits.md). Mental model: [Architecture](architecture.md). To run it: [guide](guide/README.md).
