# Observers

An observer is a long-running daemon that **receives the Raft log but does not vote**. Add one when a region, a metrics host, or a dashboard needs a local daemon and Watch without making failover slower. A 10th **voter** in another region counts in quorum: losing that node can lose majority. An observer never does.

It is not a laptop `Dial` to someone else's daemon. It is not a second cluster (that would be federation). The app model stays: app → **local** daemon → cluster.

## Why

Voters are in the quorum. Extra voters make elections wait on more peers and make one down host more likely to break majority. Observers scale reads and Watch without changing fault tolerance.

Use: regional apps that want a local daemon, dashboards, cross-AZ visibility.

## Consensus

Hashicorp Raft **Nonvoter** (`AddNonvoter`).

- Gets membership, locks, leases (same FSM as voters) so `Members()` and Watch match the cluster
- Dies or joins: quorum unchanged (observers never counted)
- Crash: presence marks not-alive; the Raft id stays (same as a voter) — restart is `clusdr start`, not another `join`
- Leave: `clusdr leave` is `RemoveServer` + `remove_member` (same path as a voter)

Join: `clusdr join --observer` (same token, same CA). Promote: `clusdr promote [id]` (`AddVoter` on the existing Raft id) when you decide this host should vote. There is no demote voter → observer in this version.

## API on the observer

Same Runtime API except **locks**. Lock / TryLock / Unlock / Renew return `FailedPrecondition` (`observer cannot mutate locks`) and do **not** forward — take locks on a voter, or promote this node. ListLocks, Watch, Publish, and leases (including presence) still work.

After promote, the node is a voter. Locks work again (forward, or local if it later leads).

`Members()` lists voters and observers. Status is `alive` or `dead`.

## Events

Custom events stay off the Raft log. Observers do not change that. Watch of `custom.<topic>` is 1-hop fanout to alive peers, including observers, so a dashboard observer still sees publishes.

## Presence

Observers hold `presence.<id>` like voters so they disappear from the **alive** list when they die. That is not `RemoveServer`. Restart with the same `data.dir` is `clusdr start`.

## Not this

- Laptop without a daemon: operator `Dial` (already exists)
- Demote voter → observer (not shipped)

## Related

- [Grow the cluster](../guide/grow.md)
- [Membership](membership.md)
- [Presence](presence.md)
- [`clusdr join`](../reference/cli/join.md)
- [`clusdr promote`](../reference/cli/promote.md)
- [`clusdr leave`](../reference/cli/leave.md)
