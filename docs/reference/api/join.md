# JoinService

JoinService is daemon-to-daemon membership on the Runtime API: join, promote, leave, and rejoin. Applications do not call this; the CLI talks to [`ControlService`](control.md) on the **local** daemon, and that daemon dials these RPCs on a seed. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal). Not in the language SDKs.

`<addr>` on the CLI is the seed **Runtime** (`grpc.addr`), not `raft.addr`. Join is first add or after Leave. A crash is `clusdr start`, not another Join.

## `Join`

**Signature:** `Join(JoinRequest) returns (JoinResponse)`

**Request:** `node_id`, `cluster_id`, `address`, `raft_addr`, `join_token`, `relay`, optional `observer`.

**Response:** `accepted`, `message`, `members`, PEM `node_cert` / `node_key` / `ca_cert`, `join_token_hash`.

The token is the plaintext `clusdr init` printed once. Invalid token → gRPC `Unauthenticated` (`UNAUTHORIZED`). Cluster id mismatch (both non-empty and different) → `accepted = false` ([errors](../errors.md#join)).

Followers forward to the leader unless `relay` is already set. `observer = true` joins as a non-voter; that node rejects lock mutations until Promote.

## `Promote`

**Signature:** `Promote(PromoteRequest) returns (PromoteResponse)`

**Request:** `node_id`, `relay`. Turns an observer into a voter on the leader (`AddVoter` + membership role). Followers forward. Unknown id → `NotFound`.

## `Leave`

**Signature:** `Leave(LeaveRequest) returns (LeaveResponse)`

**Request:** `node_id`, `relay`. The only Raft `RemoveServer`. Unknown id → `NotFound`. Already gone → `left = true`. Followers forward. Presence expiry does not call this.

## `Rejoin`

**Signature:** `Rejoin(RejoinRequest) returns (RejoinResponse)`

**Request:** `node_id`, `address`, optional `role`, `relay`. Marks an existing Raft server alive (`ApplyAddMember` only — no `AddVoter`, no join token). Not in the configuration → `FailedPrecondition` (use `Join`). Followers forward.

## See also

- [ControlService](control.md)
- [Membership](../../concepts/membership.md)
