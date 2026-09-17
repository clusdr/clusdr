# Guide

Read these pages **in order**. Each one leaves a cluster you use on the next page. Do not skip ahead to a flag or an RPC.

```text
1. Install the binary
2. Start the first member
3. Grow the cluster
4. Watch and publish
5. Use it from your app
6. Run on other hosts
```

1. [Install the binary](install.md) — `clusdr` on `PATH`
2. [Start the first member](first-member.md) — identity, Raft, one row in `members`
3. [Grow the cluster](grow.md) — a second voter, then an observer
4. [Watch and publish](watch.md) — what changed, and a signal that is not Raft
5. [Use it from your app](from-your-app.md) — local SDK, locks, leases
6. [Run on other hosts](other-hosts.md) — real addresses, Docker Hub, health

Same host model on a node (not a sequential step; not a kube replacement): [Run on Kubernetes](kubernetes.md) · [Helm](kubernetes-helm.md) · [Operator](kubernetes-operator.md) · [Sidecar](kubernetes-sidecar.md).

After the guide: [Overview](../overview.md) if you are still deciding, [concepts](../concepts/) for guarantees, [reference](../reference/) for a flag or RPC. Something failed: [Errors](../reference/errors.md).

Contributor builds stay in [CONTRIBUTING.md](https://github.com/clusdr/clusdr/blob/main/CONTRIBUTING.md).
