# Watch and publish

Daemons already emit a Watch stream: members going alive or dead, leadership, and optional custom publishes. The steps below attach to that stream, then put one deploy signal on it.

Keep the seed from [step 2](first-member.md) running. Two members from [step 3](grow.md) make `member.join` in the snapshot more obvious; one seed is enough to see `custom.deployment`.

## 1. Watch

```bash
clusdr watch
```

On connect the server sends a snapshot (`member.join` if alive, `member.dead` if dead, plus `leader.changed` at `seq = 0`), then `watch.sync`, then live events. If the snapshot is empty, `start` is not running or you pointed the CLI at another `--config`. Ctrl-C ends the client; the cluster does not.

Reconnect after a drop with the last sequence you saw (the number printed on each event), not a made-up cursor:

```bash
clusdr watch --last-seq 7
```

If the bus moved past that cursor you get the snapshot again and `watch.gap`. Custom events are **not** replayed — that is why publish is a signal, not a queue.

Filter live events when the full stream is too noisy:

```bash
clusdr watch --type member.join --type member.dead --type member.left
clusdr watch --topic deployment
```

`--topic deployment` means only `custom.deployment`. The membership snapshot is omitted so you do not mistake an old `member.join` for a new deploy. Protocol events (`watch.sync`, `watch.gap`) always pass.

Same filters on the SDKs: Go `Watch(ctx, clusdr.WithTopics("deployment"))`, Python `c.watch(topics=["deployment"])`.

## 2. Publish

Leave `watch` running. Another terminal:

```bash
clusdr publish deployment '{"sha":"8f3c1a2","env":"prod"}'
```

The watcher prints `custom.deployment`. Payload is raw bytes (usually JSON). Max 64 KiB — larger publishes are rejected. Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`; other characters fail validation.

`Publish` is accepted on any node. That node emits locally and fans out **one hop** to alive peers (including observers). Relays do not re-fanout, so a partitioned node misses the signal.

## Checkpoint

You saw `custom.deployment` on the stream. Membership, locks, and leases are on the Raft log; this payload is not. Watch reconnect will not give you missed publishes. If you need a queue, use a queue ([Events](../concepts/events.md)).

Next: [Use it from your app →](from-your-app.md)
