# `clusdr watch`

`clusdr watch` streams events from the local daemon. Use it to see membership, leadership, lock/lease expiry, and custom topics on this node. On connect the server sends a snapshot, then `watch.sync`, then live events. Ctrl-C ends the stream.

Snapshot (`seq = 0`): each listed member → `member.join` if alive, `member.dead` if dead. Left ids are absent. A crash is `member.dead`, not `member.left`. Custom events are ephemeral: a reconnect misses `custom.*` that happened while you were down ([errors](../errors.md#applications)).

## Synopsis

```bash
clusdr watch
clusdr watch --type member.join --type member.dead --type member.left
clusdr watch --topic deployment
clusdr watch --last-seq 42
clusdr watch --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`).

| Flag | Meaning |
|---|---|
| `--type` | Repeatable. Full type strings (`member.join`, `lock.expired`, …) |
| `--topic` | Repeatable. Custom topics only; membership snapshot omitted |
| `--last-seq` | Reconnect cursor |

Columns: `SEQ`, `TYPE`, `SOURCE`, `PAYLOAD`.

## Errors

| You see | What to do |
|---|---|
| `dial daemon at …` | Runtime is down ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS mismatch ([errors](../errors.md#tls)) |
| Reconnect misses `custom.*` | Expected. Custom events are not replayed ([errors](../errors.md#applications)) |

## See also

- [Watch](../../concepts/watch.md)
- [WatchService](../api/watch.md)
