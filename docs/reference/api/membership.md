# MembershipService

Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api). Available on the Runtime API.

## `ListMembers`

**Signature:** `ListMembers(ListMembersRequest) returns (ListMembersResponse)`

**Response:** `members[]` with:

| Field | Values |
|---|---|
| `id` | Node id |
| `address` | Runtime API address |
| `status` | `alive` \| `leaving` \| `dead` |
| `leader` | bool |
| `role` | `voter` \| `observer` (empty means voter) |

## `GetLeader`

**Signature:** `GetLeader(GetLeaderRequest) returns (GetLeaderResponse)`

**Response:** `leader_id`, `address`. Empty leader → `Unavailable` (`no leader elected`).

## See also

- [Membership](../../concepts/membership.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
