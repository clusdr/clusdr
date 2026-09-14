# LeaseService

`Grant` does not block. Same token / TTL / name rules as locks.

| RPC | Behavior |
|---|---|
| `Grant` | One attempt; never waits |
| `Renew` | Extends deadline |
| `Revoke` | Owner + token must match |
| `ListLeases` | All current grants |

Presence leases use the name `presence.<nodeID>`.

## See also

- [Leases](../../concepts/leases.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md)
- [Presence](../../concepts/presence.md)
