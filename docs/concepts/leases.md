# Leases

A lease is a named TTL grant. Many names can be held at once. `Grant` never blocks.

Same Raft path as locks: leader writes, followers forward. Same name rules, TTL caps, table size (4096), and fencing token.

RPCs: `Grant`, `Renew`, `Revoke`, `ListLeases`.

## SDK

Background renew until context cancel (Go), `stop` Event (Python), `stop_renew` (Rust), `Revoke`, or `Close`.

Cancelling the Go `Lease` context **stops renew**. The grant then expires. It does not revoke.

## Presence

The daemon's own liveness lease is named `presence.<nodeID>`. That is [presence](presence.md), not an application lease, but it uses this table.

Observers still grant and renew leases (presence must work). They only reject **locks**.

## Related

- [Locks](locks.md)
- [Presence](presence.md)
- [Go SDK](../sdk/go.md) · [Python SDK](../sdk/python.md) · [Rust SDK](../sdk/rust.md) · [TypeScript SDK](../sdk/typescript.md) · [Java SDK](../sdk/java.md)
- [LeaseService](../reference/api/leases.md)
