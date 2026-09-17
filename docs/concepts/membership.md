# Membership

Membership is the set of daemons in the cluster and their liveness.

Join and leave that change the Raft configuration go through the **leader**. A follower's Join/Leave/Rejoin RPC is forwarded. The in-memory member list updates when the Raft log applies — not via gossip.

## A member

| Field | Meaning |
|---|---|
| `id` | Stable node id (`node.id`) |
| `address` | Advertised Runtime API address |
| `status` | `alive` or `dead` (liveness only) |
| `leader` | True if this id is the current Raft leader |
| `role` | `voter` or `observer` (empty means voter) |

`clusdr members` prints role as `leader`, `voter`, or `observer` (leader implies voter).

Status is **liveness**. It is not whether the id is still a Raft server. A left id is gone from the list, so it has no status.

## How a node appears

1. Seed: `clusdr start --bootstrap`. That node is a voter.
2. Later nodes: running daemon + `clusdr join` ([grow the cluster](../guide/grow.md)).
3. Observers: `clusdr join --observer`.

The first process in a new cluster must bootstrap. Later nodes must not.

`join` is once per **new** Raft member (or after `clusdr leave` already removed that id). A reboot of a node that is still in the configuration is `clusdr start` with the same `data.dir`. Details: [presence](presence.md).

## How a node disappears

Two axes. Crash is not leave.

| Fact | Status | Watch | Members() | Raft |
|---|---|---|---|---|
| Heartbeat / presence OK | `alive` | `member.join` (first time or back from dead) | listed | in config |
| Presence expiry or heartbeat miss | `dead` | `member.dead` | listed | **stays** |
| [`clusdr leave`](../reference/cli/leave.md) | — | `member.left` | **gone** | `RemoveServer` |

There is no `member.leave` event. Leave is the CLI/RPC; the bus fact is `member.left`.

A host that was **left** must `join` again. One that crashed or rebooted uses `clusdr start`.

## Listing

Operators: `clusdr members` / `clusdr leader`.  
Apps: `Members()` / `Leader()`.

No leader → `GetLeader` returns `Unavailable`.

## Related

- [Presence](presence.md)
- [Observers](observers.md)
- [Leadership](leadership.md)
- [Start the first member](../guide/first-member.md)
- [MembershipService](../reference/api/membership.md)
- [`clusdr members`](../reference/cli/members.md)
- [`clusdr leave`](../reference/cli/leave.md)
