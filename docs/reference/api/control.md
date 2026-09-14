# ControlService

CLI → local daemon.

## `RequestJoin`

**Signature:** `RequestJoin(RequestJoinRequest) returns (RequestJoinResponse)`

**Request:** `addr` (existing member Runtime API), `token`, optional `observer`.

**Response:** `joined`, `message`, `members`, `cert_issued`.

## `RequestPromote`

**Signature:** `RequestPromote(RequestPromoteRequest) returns (RequestPromoteResponse)`

**Request:** optional `node_id` (empty = local node).

**Response:** `promoted`, `message`, `members`. Unknown id → `NotFound`. Already a voter → `promoted = true`.

## See also

- [`clusdr join`](../cli/join.md)
- [`clusdr promote`](../cli/promote.md)
- [JoinService](join.md)
