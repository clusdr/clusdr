# Locks

A lock is an exclusive named grant on the Raft log. The leader is the only writer. Followers forward.

| | Behavior |
|---|---|
| `Lock` | Blocks until acquired or the RPC deadline |
| `TryLock` | One attempt |
| `Unlock` | `holder` + fencing token must match |
| `Renew` | Extends deadline; `ttl_ms = 0` reuses the last TTL |
| `ListLocks` | All current grants |

## Rules

- Name: 1–128 characters, letters, digits, `.`, `_`, `-`
- Default TTL 15s, max 24h, min 1ms
- Max 4096 locks in the table
- Token is monotonic. Store it with any write that must be fenced

Dead holder: TTL + leader expire scan → `lock.expired`.

## Observers

An observer **rejects** Lock / TryLock / Unlock / Renew (`FailedPrecondition`). ListLocks is allowed. See [observers](observers.md).

## SDK

The SDK renews in the background (about TTL/3). `Close` unlocks what this connection holds. Empty holder becomes a generated `sdk-<hex>` so two apps cannot unlock each other.

Go `TryLock` when held: `(nil, false, nil)` — not an error. Python `try_lock` / Rust `try_lock` return `None` / `Ok(None)`.

## Related

- [Leases](leases.md)
- [Go SDK](../sdk/go.md) · [Python SDK](../sdk/python.md) · [Rust SDK](../sdk/rust.md) · [TypeScript SDK](../sdk/typescript.md)
- [LockService](../reference/api/locks.md)
