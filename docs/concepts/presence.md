# Presence

When `lease.presence` is true (default), each daemon holds `presence.<nodeID>` (default TTL 3s).

If that lease expires, the leader removes the node (`member.left`). Heartbeats are the slower backup (interval 2s, timeout 1s, 3 misses).

Observers hold presence like voters so they disappear from the list when they die. Expiry uses `RemoveServer` (works for voters and observers).

Turn off with `lease.presence: false` or `CLUSDR_LEASE_PRESENCE`.

## Related

- [Leases](leases.md)
- [Membership](membership.md)
- [Configuration](../reference/configuration.md)
