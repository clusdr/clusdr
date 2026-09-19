# LeaseService

LeaseService is named TTL grants backed by Raft. Use it from apps for presence-style ownership that must not block. `Grant` does not wait; if the name is taken, the call returns not-granted. Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

Same token, TTL, and name rules as locks. Store the fencing token; a stale revoke fails ([errors](../errors.md#locks-and-leases)). Observers **can** hold application leases. Presence grants use `presence.<nodeID>` and mark a dead member `dead` without `leave`.

`Grant` does not block. Same token / TTL / name rules as locks.

| RPC | Request / response | Behavior |
|---|---|---|
| `Grant` | `GrantRequest` / `GrantResponse` | One attempt; never waits |
| `Renew` | `LeaseServiceRenewRequest` / `LeaseServiceRenewResponse` | Extends deadline; `ttl_ms = 0` reuses the last TTL |
| `Revoke` | `RevokeRequest` / `RevokeResponse` | Owner + token must match |
| `ListLeases` | `ListLeasesRequest` / `ListLeasesResponse` | All current grants (`LeaseInfo`) |

**GrantRequest:** `name`, `owner` (empty → this daemon's node id), `ttl_ms` (`0` → `lease.ttl`).

**GrantResponse:** `granted`, `message`, `fencing_token`, `owner`, `deadline_unix_ms`.

**LeaseServiceRenewRequest:** `name`, `owner`, `fencing_token`, `ttl_ms`. **LeaseServiceRenewResponse:** `renewed`, `message`, `fencing_token`, `deadline_unix_ms`.

**RevokeRequest:** `name`, `owner`, `fencing_token`. **RevokeResponse:** `revoked`, `message`.

Presence leases use the name `presence.<nodeID>`. Name charset `1–128` of `A–Z a–z 0–9 . _ -`. Table cap 4096 ([errors](../errors.md#locks-and-leases)).

## See also

- [Leases](../../concepts/leases.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
- [Presence](../../concepts/presence.md)
