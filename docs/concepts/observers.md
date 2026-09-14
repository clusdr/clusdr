# Observers

An observer is a long-running daemon that **receives the Raft log but does not vote**.

It is not a laptop `Dial` to someone else's daemon. It is not a second cluster (that would be federation). The app model stays: app → **local** daemon → cluster.

## Why

Voters are in the quorum. A 10th voter in another region or on a metrics host makes failover slower and loss of that node affect majority. Observers scale reads and Watch without changing fault tolerance.

Use: regional apps that want a local daemon, dashboards, cross-AZ visibility.

## Consensus

Hashicorp Raft **Nonvoter** (`AddNonvoter`).

- Gets membership, locks, leases (same FSM as voters)
- Dies or joins: quorum unchanged
- Leave: `RemoveServer` + `remove_member` (same path as a voter)

Join: `clusdr join --observer` (same token, same CA). Promote: `clusdr promote [id]` (`AddVoter` on the existing Raft id).

## API on the observer

Same Runtime API except **locks**. Lock / TryLock / Unlock / Renew return `FailedPrecondition` (`observer cannot mutate locks`). No forward.

ListLocks, Watch, Publish, and leases (including presence) still work.

After promote, the node is a voter. Locks work again (forward, or local if it later leads).

`Members()` lists voters and observers. Status stays `alive` / `leaving` / `dead`.

## Events

Custom events stay off the Raft log. Observers do not change that. Watch of `custom.<topic>` is 1-hop fanout to alive peers, including observers.

## Presence

Observers hold `presence.<id>` like voters so they disappear from the list when they die.

## Not this

- Laptop without a daemon: operator `Dial` (already exists)
- Demote voter → observer (not shipped)

## Related

- [Grow the cluster](../guide/grow.md)
- [Membership](membership.md)
- [`clusdr join`](../reference/cli/join.md)
- [`clusdr promote`](../reference/cli/promote.md)
