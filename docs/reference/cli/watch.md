# `clusdr watch`

Streams events from the local daemon. Snapshot, then `watch.sync`, then live. Ctrl-C ends the stream.

## Synopsis

```bash
clusdr watch [--type <type>]... [--topic <topic>]... [--last-seq <n>]
```

| Flag | Meaning |
|---|---|
| `--type` | Repeatable. Full type strings (`member.join`, `lock.expired`, …) |
| `--topic` | Repeatable. Custom topics only; membership snapshot omitted |
| `--last-seq` | Reconnect cursor |

## See also

- [Watch](../../concepts/watch.md)
- [WatchService](../api/watch.md)
