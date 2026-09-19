# ControlService

ControlService is the operator path from the CLI to the **local** daemon. Use it when you implement `clusdr join` / `promote` / `leave` behavior; applications use the SDKs instead. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal) (same file as JoinService). Not in the language SDKs.

The shipped CLI dials Runtime TCP (`grpc.addr`, default `127.0.0.1:7947`), not the Unix socket. `RequestJoin` then dials the seed Runtime at `addr`. Join is once unless Leave already removed the id.

## `RequestJoin`

**Signature:** `RequestJoin(RequestJoinRequest) returns (RequestJoinResponse)`

**Request:** `addr` (existing member Runtime API), `token`, optional `observer`.

**Response:** `joined`, `message`, `members`, `cert_issued`.

Token is the plaintext `clusdr init` printed once. Invalid token → `UNAUTHORIZED`. `addr` is Runtime, not Raft ([errors](../errors.md#join)). `observer = true` joins as a non-voter; lock mutations fail until RequestPromote ([errors](../errors.md#locks-and-leases)).

## `RequestPromote`

**Signature:** `RequestPromote(RequestPromoteRequest) returns (RequestPromoteResponse)`

**Request:** optional `node_id` (empty = local node). IDs come from `ListMembers` / `clusdr members`.

**Response:** `promoted`, `message`, `members`. Unknown id → `NotFound`. Already a voter → `promoted = true`.

## `RequestLeave`

**Signature:** `RequestLeave(RequestLeaveRequest) returns (RequestLeaveResponse)`

**Request:** optional `node_id` (empty = local node).

**Response:** `left`, `message`, `members`. Unknown id → `NotFound`. Already gone → `left = true`. After leave, the same `data.dir` is not a member; join again. A crash is `clusdr start`, not leave.

## See also

- [`clusdr join`](../cli/join.md)
- [`clusdr promote`](../cli/promote.md)
- [`clusdr leave`](../cli/leave.md)
- [JoinService](join.md)
