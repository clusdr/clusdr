# ControlService

CLI → local daemon. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal) (same file as JoinService). Not in the language SDKs.

## `RequestJoin`

**Signature:** `RequestJoin(RequestJoinRequest) returns (RequestJoinResponse)`

**Request:** `addr` (existing member Runtime API), `token`, optional `observer`.

**Response:** `joined`, `message`, `members`, `cert_issued`.

## `RequestPromote`

**Signature:** `RequestPromote(RequestPromoteRequest) returns (RequestPromoteResponse)`

**Request:** optional `node_id` (empty = local node).

**Response:** `promoted`, `message`, `members`. Unknown id → `NotFound`. Already a voter → `promoted = true`.

## `RequestLeave`

**Signature:** `RequestLeave(RequestLeaveRequest) returns (RequestLeaveResponse)`

**Request:** optional `node_id` (empty = local node).

**Response:** `left`, `message`, `members`. Unknown id → `NotFound`. Already gone → `left = true`.

## See also

- [`clusdr join`](../cli/join.md)
- [`clusdr promote`](../cli/promote.md)
- [`clusdr leave`](../cli/leave.md)
- [JoinService](join.md)
