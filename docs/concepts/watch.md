# Watch

Watch is a server stream of what changed in the cluster: members going alive or dead, leadership, lock/lease expiry, and optional custom publishes. It is not a poll of `Members()`. Use it when a process must react as facts happen — a deployer waiting for `member.dead`, a UI showing the current leader — without opening a new RPC every second.

## Protocol

1. Snapshot of listed members (`member.join` if alive, `member.dead` if dead) and the leader (`seq = 0`) so a new subscriber is not empty until the next crash
2. `watch.sync` (bus high-water mark) so you know the snapshot is complete
3. Live events

Reconnect with `last_seq` from the last event you processed. If the bus moved past that cursor, the server sends the snapshot again and `watch.gap` — you must rebuild from the snapshot; custom events are **not** replayed. How to obtain and pass `last_seq`: [Watch and publish](../guide/watch.md).

`--topic` / SDK topic filter: only `custom.<topic>` for those keys; membership snapshot omitted so a topic subscriber is not flooded with member rows.

`--type` / `event_types`: live events must match one of those full type strings. Protocol events (`watch.sync`, `watch.gap`) always pass so reconnect still works.

## Event types

`member.join`, `member.dead`, `member.left`, `leader.changed`, `lock.expired`, `lease.granted`, `lease.expired`, `lease.revoked`, `custom.<topic>`, `watch.sync`, `watch.gap`.

Crash/miss is `member.dead` (still listed). Operator leave is `member.left` (gone). There is no `member.leave`.

## Backpressure

The bus is bounded. A slow subscriber drops events; the next reconnect may see `watch.gap`. The SDK Watch buffer is 64 events. Do not do blocking I/O inside the Watch loop if you need every membership fact — handle the event and move on, or you will miss `member.dead`.

## Related

- [Custom events](events.md)
- [Watch and publish](../guide/watch.md)
- [WatchService](../reference/api/watch.md)
- [`clusdr watch`](../reference/cli/watch.md)
