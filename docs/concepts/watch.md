# Watch

`Watch` is a server stream of cluster and custom events.

## Protocol

1. Snapshot of listed members (`member.join` if alive, `member.dead` if dead) and the leader (`seq = 0`)
2. `watch.sync` (bus high-water mark)
3. Live events

Reconnect with `last_seq`. If the bus moved past that cursor, the server sends the snapshot again and `watch.gap`. Custom events are **not** replayed.

`--topic` / SDK topic filter: only `custom.<topic>` for those keys; membership snapshot omitted.

`--type` / `event_types`: live events must match one of those full type strings. Protocol events (`watch.sync`, `watch.gap`) always pass.

## Event types

`member.join`, `member.dead`, `member.left`, `leader.changed`, `lock.expired`, `lease.granted`, `lease.expired`, `lease.revoked`, `custom.<topic>`, `watch.sync`, `watch.gap`.

Crash/miss is `member.dead` (still listed). Operator leave is `member.left` (gone). There is no `member.leave`.

## Backpressure

The bus is bounded. A slow subscriber drops events. The SDK Watch buffer is 64 events.

## Related

- [Custom events](events.md)
- [Watch and publish](../guide/watch.md)
- [WatchService](../reference/api/watch.md)
- [`clusdr watch`](../reference/cli/watch.md)
