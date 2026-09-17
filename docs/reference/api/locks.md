# LockService

Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

Leader commits; followers forward. An **observer** rejects Lock / TryLock / Unlock / Renew (`FailedPrecondition`: `observer cannot mutate locks`). ListLocks is allowed.

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

## See also

- [Locks](../../concepts/locks.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
