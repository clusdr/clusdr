# Watch and publish

You have the seed from [step 2](first-member.md), better two members from [step 3](grow.md). This page shows the stream, then one signal.

## 1. Watch

```bash
clusdr watch
```

On connect you get a snapshot (`member.join` / `member.dead`, `leader.changed` at `seq = 0`), then `watch.sync`, then live events. Ctrl-C ends the client.

```bash
clusdr watch --last-seq 42
clusdr watch --type member.join --type member.dead --type member.left
clusdr watch --topic deployment
```

`--topic` means only `custom.deployment` (membership snapshot omitted). Protocol events (`watch.sync`, `watch.gap`) always pass.

## 2. Publish

Another terminal, while `watch` is running:

```bash
clusdr publish deployment '{"sha":"abc"}'
```

The watcher prints `custom.deployment`. Payload max 64 KiB. Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`.

## Checkpoint

You saw `custom.deployment` on the stream. Custom events are not on the Raft log and are not replayed after a gap. If you need a queue, use a queue. [Events](../concepts/events.md).

Next: [Use it from your app →](from-your-app.md)
