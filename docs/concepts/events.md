# Custom events

`Publish` is accepted on any node. That node emits `custom.<topic>` locally and fans out **one hop** to alive peers (including observers). Relays do not re-fanout.

These events are **not** written to the Raft log. Treat them as signals, not a log.

## Limits

- Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`
- Payload: max 64 KiB
- Duplicate `event_id` → accepted with `message = duplicate`

A slow watcher can drop events (bounded bus). Watch reconnect does not replay custom events.

## Related

- [Watch](watch.md)
- [Consistency](consistency.md)
- [Watch and publish](../guide/watch.md)
- [EventService](../reference/api/events.md)
