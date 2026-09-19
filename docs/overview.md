# Overview

Clusdr answers four questions for applications on a cluster — who is in the cluster, who is the leader, who is still alive, and what just changed — without asking you to run etcd or Consul as a product you operate. The daemon on each host is the Raft member. Your application talks only to that local daemon.

This is a fit check, not an install guide. [Get started](./) if you already decided.

## Use it when

- Agents or services need a membership list and a watch stream, and you do not want each app to speak Raft
- Services need leader election or exclusive work (a scheduler claiming a job name) without standing up etcd or Consul
- A crash should show `member.dead` without removing the Raft id. The id stays until `clusdr leave` (`member.left`). A reboot of the same disk is `clusdr start`, not another `join` ([presence](concepts/presence.md)) — otherwise every restart looks like a new node and quorum thrashes
- You need exclusive [locks](concepts/locks.md) or named TTL [leases](concepts/leases.md) with fencing tokens, so a stale holder cannot unlock a newer grant
- Extra hosts should see the same state without changing quorum ([observers](concepts/observers.md))
- You need a small signal (`publish` → `custom.<topic>`), not a durable log. Missed publishes are gone; if you need a queue, use a queue

## Do not use it when

- You need to store application state or files — there is no KV or blob store
- You need at-least-once or durable messaging — custom events are not on the Raft log
- You need multi-cluster federation — not in this version
- You want to replace etcd as Kubernetes storage
- You want to replace the Kubernetes Lease API, probes, or EndpointSlice. A Go controller that only needs “who leads this Deployment” should keep using `coordination.k8s.io`

Clusdr is not a database, queue, workflow engine, service mesh, or Kubernetes. It **runs on** Kubernetes as a Linux host ([Kubernetes](concepts/kubernetes.md)). If you already run etcd for KV and only wanted membership, adding Clusdr is a second control plane — do that only when the local-daemon model is the point.

Boundaries of this version: [Limits](reference/limits.md). Process model: [Architecture](architecture.md). To run it: [Tutorial](guide/).
