# Leases

A lease is a named TTL grant that does **not** have to be exclusive in the lock sense: many names can be held at once, and `Grant` never blocks. Use it when a worker must prove it is still alive (session, presence-like app state) without waiting to acquire a mutex. Same Raft path as locks: leader writes, followers forward. Same name rules, TTL caps, table size (4096), and fencing token.

If you need “only one owner of this name,” use a [lock](locks.md). If you need “this holder is still here until TTL,” use a lease.

RPCs: `Grant`, `Renew`, `Revoke`, `ListLeases`. Over 4096 names, grant returns `too many leases` ([errors](../reference/errors.md#locks-and-leases)).

## SDK

Background renew until context cancel (Go), `stop` Event (Python), `stop_renew` (Rust), `Revoke`, or `Close`.

Cancelling the Go `Lease` context **stops renew**. The grant then expires. It does not revoke — other readers still see the name until TTL. Call `Revoke` if you need it gone now.

## Presence

The daemon's own liveness lease is named `presence.<nodeID>`. That is [presence](presence.md), not an application lease, but it uses this table. Do not grant an app lease with that prefix if you want to keep operator liveness separate.

Observers still grant and renew leases (presence must work). They only reject **locks**.

## Related

- [Locks](locks.md)
- [Presence](presence.md)
- [Go SDK](../sdk/go.md) · [Python SDK](../sdk/python.md) · [Rust SDK](../sdk/rust.md) · [TypeScript SDK](../sdk/typescript.md) · [Java SDK](../sdk/java.md)
- [LeaseService](../reference/api/leases.md)
