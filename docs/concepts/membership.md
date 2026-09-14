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

## How a node disappears

- Operator or peer leave → leader `RemoveServer` + `remove_member`
- [Presence](presence.md) lease expiry → same leave path
- Heartbeat misses (slower backup)

## Listing

Operators: `clusdr members` / `clusdr leader`.  
Apps: `Members()` / `Leader()`.

No leader → `GetLeader` returns `Unavailable`.

## Related

- [Observers](observers.md)
- [Leadership](leadership.md)
- [Start the first member](../guide/first-member.md)
- [MembershipService](../reference/api/membership.md)
- [`clusdr members`](../reference/cli/members.md)
