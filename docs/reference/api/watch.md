# WatchService

WatchService streams cluster events from the local daemon. Use it from apps and [`clusdr watch`](../../reference/cli/watch.md) to see membership, leadership, lock/lease expiry, and custom topics. Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

On every new stream the server sends a snapshot of current members, then live events. Custom events are ephemeral: a reconnect misses `custom.*` that happened while you were down ([errors](../errors.md#applications)). Crash is `member.dead`, not `member.left`.

## `Watch`

**Signature:** `Watch(WatchRequest) returns (stream WatchResponse)`

**Request**

| Field | Meaning |
|---|---|
| `event_types` | If set, live events must match one of these full type strings. Protocol events always pass |
| `last_seq` | Last live seq the client processed. `0` = first connect |
| `topics` | If set, only `custom.<topic>` for those keys; membership snapshot omitted |

**Response:** `type`, `source`, `payload`, `timestamp_unix_ms`, `seq`.

Snapshot events use `seq = 0`. Then `watch.sync`. Optional `watch.gap` if `last_seq` is behind.

**Types:** `member.join`, `member.dead`, `member.left`, `leader.changed`, `lock.expired`, `lease.granted`, `lease.expired`, `lease.revoked`, `custom.<topic>`, `watch.sync`, `watch.gap`.

Snapshot (`seq = 0`): each listed member → `member.join` if alive, `member.dead` if dead. Left ids are absent. Crash is not `member.left`.

Dial the Runtime API (`grpc.addr`). Transport failures: [errors](../errors.md#daemon-and-dial), [errors](../errors.md#tls).

## See also

- [Watch](../../concepts/watch.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
