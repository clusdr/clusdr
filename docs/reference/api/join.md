# JoinService

Daemon → daemon.

## `Join`

**Request:** `node_id`, `cluster_id`, `address`, `raft_addr`, `join_token`, `relay`, optional `observer`.

**Response:** `accepted`, `message`, `members`, PEM `node_cert` / `node_key` / `ca_cert`, `join_token_hash`.

Invalid token → gRPC `Unauthenticated` (`UNAUTHORIZED`). Cluster id mismatch (both non-empty and different) → `accepted = false`.

Followers forward to the leader unless `relay` is already set.

## `Promote`

**Request:** `node_id`, `relay`. Turns an observer into a voter on the leader (`AddVoter` + membership role). Followers forward.

## See also

- [ControlService](control.md)
- [Membership](../../concepts/membership.md)
