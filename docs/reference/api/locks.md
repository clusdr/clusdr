# LockService

LockService is cluster-wide exclusive locks backed by Raft. Use it from apps that need a named hold with a fencing token. Leader commits; followers forward. Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

An **observer** rejects Lock / TryLock / Unlock / Renew (`FailedPrecondition`: `observer cannot mutate locks`) and does **not** forward — take locks on a voter, or promote that node ([errors](../errors.md#locks-and-leases)). ListLocks is allowed. Store the fencing token; a stale unlock fails.

| RPC | Request / response | Behavior |
|---|---|---|
| `Lock` | `LockRequest` / `LockResponse` | Blocks until acquired or the RPC deadline |
| `TryLock` | `TryLockRequest` / `TryLockResponse` | One attempt |
| `Unlock` | `UnlockRequest` / `UnlockResponse` | `holder` + `fencing_token` must match |
| `Renew` | `LockServiceRenewRequest` / `LockServiceRenewResponse` | Extends deadline; `ttl_ms = 0` reuses the last TTL |
| `ListLocks` | `ListLocksRequest` / `ListLocksResponse` | All current grants (`LockInfo`) |

**LockRequest / TryLockRequest:** `name`, `holder` (empty → this daemon's node id), `ttl_ms` (`0` → `lock.ttl`). Same fields; `TryLock` does not wait.

**LockResponse / TryLockResponse:** `acquired`, `message`, `fencing_token`, `holder`, `deadline_unix_ms`.

**UnlockRequest:** `name`, `holder`, `fencing_token`. **UnlockResponse:** `released`, `message`.

**LockServiceRenewRequest:** `name`, `holder`, `fencing_token`, `ttl_ms`. **LockServiceRenewResponse:** `renewed`, `message`, `fencing_token`, `deadline_unix_ms`.

Name charset `1–128` of `A–Z a–z 0–9 . _ -`. Table cap 4096 ([errors](../errors.md#locks-and-leases)).

## See also

- [Locks](../../concepts/locks.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
