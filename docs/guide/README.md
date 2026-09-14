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
6. [Run on other hosts](other-hosts.md) — real addresses, GHCR, health

After the guide: [Overview](../overview.md) if you are still deciding, [concepts](../concepts/README.md) for guarantees, [reference](../reference/README.md) for a flag or RPC.

Contributor builds stay in [CONTRIBUTING.md](../../CONTRIBUTING.md).
