# LockService

Leader commits; followers forward. An **observer** rejects Lock / TryLock / Unlock / Renew (`FailedPrecondition`: `observer cannot mutate locks`). ListLocks is allowed.

| RPC | Behavior |
|---|---|
| `Lock` | Blocks until acquired or the RPC deadline |
| `TryLock` | One attempt |
| `Unlock` | `holder` + `fencing_token` must match |
| `Renew` | Extends deadline; `ttl_ms = 0` reuses the last TTL |
| `ListLocks` | All current grants |

**LockRequest / TryLockRequest:** `name`, `holder` (empty → this daemon's node id), `ttl_ms` (`0` → `lock.ttl`). Same fields; `TryLock` does not wait.

**LockResponse:** `acquired`, `message`, `fencing_token`, `holder`, `deadline_unix_ms`.

## See also

- [Locks](../../concepts/locks.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
