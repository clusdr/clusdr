# JoinService

Daemon → daemon. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal). Not in the language SDKs.

## `Join`

**Signature:** `Join(JoinRequest) returns (JoinResponse)`

**Request:** `node_id`, `cluster_id`, `address`, `raft_addr`, `join_token`, `relay`, optional `observer`.

**Response:** `accepted`, `message`, `members`, PEM `node_cert` / `node_key` / `ca_cert`, `join_token_hash`.

Invalid token → gRPC `Unauthenticated` (`UNAUTHORIZED`). Cluster id mismatch (both non-empty and different) → `accepted = false`.

Followers forward to the leader unless `relay` is already set.

## `Promote`

**Signature:** `Promote(PromoteRequest) returns (PromoteResponse)`

**Request:** `node_id`, `relay`. Turns an observer into a voter on the leader (`AddVoter` + membership role). Followers forward.

## `Leave`

**Signature:** `Leave(LeaveRequest) returns (LeaveResponse)`

**Request:** `node_id`, `relay`. The only Raft `RemoveServer`. Unknown id → `NotFound`. Already gone → `left = true`. Followers forward.

## `Rejoin`

**Signature:** `Rejoin(RejoinRequest) returns (RejoinResponse)`

**Request:** `node_id`, `address`, optional `role`, `relay`. Marks an existing Raft server alive (`ApplyAddMember` only — no `AddVoter`, no join token). Not in the configuration → `FailedPrecondition` (use `Join`). Followers forward.

## See also

- [ControlService](control.md)
- [Membership](../../concepts/membership.md)
