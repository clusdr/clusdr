# Custom events

`Publish` is a fire-and-forget signal to every alive daemon: deploy started, cache invalidate, “run this job now.” Any node accepts it, emits `custom.<topic>` locally, and fans out **one hop** to alive peers (including observers). Relays do not re-fanout, so a partition drops the signal instead of inventing a second hop storm.

These events are **not** written to the Raft log. Treat them as signals, not a durable log. A Watch reconnect does not replay missed publishes — if you need that fact after a gap, publish again or store it yourself. Putting user payloads on Raft would make every publish a quorum write and bloat snapshots; that is a non-goal ([consistency](consistency.md)).

## Limits

- Topic: 1–128 characters, `A–Z a–z 0–9 . _ -` (example: `deploy.payments.canary`)
- Payload: max 64 KiB — over that, the RPC is `clusdr: publish rejected` ([errors](../reference/errors.md#applications))
- Duplicate `event_id` → accepted with `message = duplicate` so a retry does not create a second fact

A slow watcher can drop events (bounded bus). Watch reconnect does not replay custom events.

## Related

- [Watch](watch.md)
- [Consistency](consistency.md)
- [Watch and publish](../guide/watch.md)
- [EventService](../reference/api/events.md)
