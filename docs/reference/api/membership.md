# MembershipService

MembershipService is the cluster view over the Runtime API: who is in Raft, who is alive, who leads. Use it from apps and from [`clusdr members`](../../reference/cli/members.md) / [`clusdr leader`](../../reference/cli/leader.md). It is not [`clusdr status`](../../reference/cli/status.md) (Unix socket file) and not Health `role` (always `standalone`). Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

`address` on each member is the advertised Runtime API (`node.addr`), not `raft.addr`.

## `ListMembers`

**Signature:** `ListMembers(ListMembersRequest) returns (ListMembersResponse)`

**Response:** `members[]` with:

| Field | Values |
|---|---|
| `id` | Node id |
| `address` | Runtime API address |
| `status` | `alive` \| `dead` (liveness; legacy `leaving` is read as `dead`) |
| `leader` | bool |
| `role` | `voter` \| `observer` (empty means voter) |

A crash marks `dead` and keeps the Raft id. `clusdr leave` removes the row. Copy `id` from this list for Leave and Promote.

## `GetLeader`

**Signature:** `GetLeader(GetLeaderRequest) returns (GetLeaderResponse)`

**Response:** `leader_id`, `address`. Empty leader → `Unavailable` (`no leader elected`). One voter: that process must be up. Three voters: majority must be up ([errors](../errors.md#daemon-and-dial)).

## See also

- [Membership](../../concepts/membership.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
