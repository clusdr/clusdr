# LeaseService

Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

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

Presence leases use the name `presence.<nodeID>`.

## See also

- [Leases](../../concepts/leases.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
- [Presence](../../concepts/presence.md)
