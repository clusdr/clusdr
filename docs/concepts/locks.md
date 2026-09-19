# Locks

A lock is an exclusive named grant on the Raft log: only one holder at a time for that name. Use it when a job scheduler, deployer, or failover worker must claim work so two replicas do not run the same job. The leader is the only writer. Followers forward. If you take a lock on an [observer](observers.md), the RPC fails with `FailedPrecondition` instead of forwarding — take locks on a voter.

| | Behavior |
|---|---|
| `Lock` | Blocks until acquired or the RPC deadline |
| `TryLock` | One attempt; does not wait |
| `Unlock` | `holder` + fencing token must match, so a delayed unlock cannot steal a newer grant |
| `Renew` | Extends deadline; `ttl_ms = 0` reuses the last TTL |
| `ListLocks` | All current grants |

## Rules

- Name: 1–128 characters, letters, digits, `.`, `_`, `-` (example: `scheduler.payments.nightly`)
- Default TTL 15s, max 24h, min 1ms — after TTL the leader expire scan emits `lock.expired` so a dead holder does not hold the name forever
- Max 4096 locks in the table; over that, grant returns `too many locks` ([errors](../reference/errors.md#locks-and-leases))
- Token is monotonic. Store it with any write that must be fenced (a delayed writer with an old token must not commit)

## Observers

An observer **rejects** Lock / TryLock / Unlock / Renew (`FailedPrecondition`). ListLocks is allowed. See [observers](observers.md). After `clusdr promote`, that node is a voter and lock RPCs work again.

## SDK

The SDK renews in the background (about TTL/3). `Close` unlocks what this connection holds so a process exit does not wait for TTL. Empty holder becomes a generated `sdk-<hex>` so two apps on the same host cannot unlock each other unless they share `WithHolder` / `holder=`.

Go `TryLock` when someone else holds the name: `(*Lock, false, nil)` — not an error; the `Lock` may describe the current holder. Python `try_lock` / Rust `try_lock` return `None` / `Ok(None)`.

## Related

- [Leases](leases.md)
- [Go SDK](../sdk/go.md) · [Python SDK](../sdk/python.md) · [Rust SDK](../sdk/rust.md) · [TypeScript SDK](../sdk/typescript.md) · [Java SDK](../sdk/java.md)
- [LockService](../reference/api/locks.md)
