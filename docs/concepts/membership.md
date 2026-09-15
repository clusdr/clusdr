# Membership

Membership is the set of daemons in the cluster and their liveness.

Join and leave that change the Raft configuration go through the **leader**. A follower's Join RPC is forwarded. The in-memory member list updates when the Raft log applies — not via gossip.

## A member

| Field | Meaning |
|---|---|
| `id` | Stable node id (`node.id`) |
| `address` | Advertised Runtime API address |
| `status` | `alive`, `leaving`, or `dead` |
| `leader` | True if this id is the current Raft leader |
| `role` | `voter` or `observer` (empty means voter) |

`clusdr members` prints role as `leader`, `voter`, or `observer` (leader implies voter).

## How a node appears

1. Seed: `clusdr start --bootstrap`. That node is a voter.
2. Later nodes: running daemon + `clusdr join` ([grow the cluster](../guide/grow.md)).
3. Observers: `clusdr join --observer`.

The first process in a new cluster must bootstrap. Later nodes must not.

`join` is once per **new** Raft member (or after the leader already removed that id). A reboot of a node that is still in the configuration is `clusdr start` with the same `data.dir`. Details: [presence](presence.md).

## How a node disappears

There is no separate disconnect signal. Watch only has `member.left`.

- [Presence](presence.md) lease expiry → leader `RemoveServer` + `remove_member` (default TTL 3s)
- Heartbeat misses → same remove path (slower backup)

A host that was removed must `join` again. One that returns inside the TTL does not.

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
