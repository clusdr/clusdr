# Watch and publish

You have at least the seed from [step 2](first-member.md), better two members from [step 3](grow.md). Now look at the stream those daemons already emit, then put your own signal on it.

## Watch is a stream, not a poll

```bash
clusdr watch
```

On connect the server sends:

1. A snapshot of listed members (`member.join` if alive, `member.dead` if dead) and the leader (`seq = 0`)
2. `watch.sync` (bus high-water mark)
3. Live events

Ctrl-C ends the client. The cluster does not.

Reconnect with a cursor if you had one:

```bash
clusdr watch --last-seq 42
```

If the bus moved past that cursor you get the snapshot again and `watch.gap`. Custom events are **not** replayed.

Filter live events:

```bash
clusdr watch --type member.join --type member.dead --type member.left
clusdr watch --topic deployment
```

`--topic` means only `custom.deployment`. The membership snapshot is omitted. `--type` matches full type strings. Protocol events (`watch.sync`, `watch.gap`) always pass.

Same filters on the SDKs: Go `Watch(ctx, clusdr.WithTopics("deployment"))`, Python `c.watch(topics=["deployment"])`, Rust `c.watch(WatchFilter::new().topics(["deployment"]))`.

Event types you will see: `member.join`, `member.dead`, `member.left`, `leader.changed`, `lock.expired`, `lease.granted`, `lease.expired`, `lease.revoked`, `custom.<topic>`, plus the two protocol events. Crash is `member.dead`; leave is `member.left`.

The bus is bounded. A slow subscriber **drops** events. The cluster does not wait.

## Publish a signal

From another terminal, while `watch` is running:

```bash
clusdr publish deployment '{"sha":"abc"}'
```

The watcher prints `custom.deployment`. Payload is raw bytes (usually JSON text). Max 64 KiB. Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`.

`Publish` is accepted on any node. That node emits locally and fans out **one hop** to alive peers (including observers). Relays do not re-fanout.

## This is not Raft

Membership, locks, and leases are on the Raft log. The leader is the only writer.

`custom.<topic>` is **not** on that log. Treat it as a signal: “a deploy happened”, not “here is a durable audit trail”. Watch reconnect will not give you missed publishes. If you need a queue, use a queue.

## Next

Same cluster, now from a program: [Use it from your app →](from-your-app.md)
