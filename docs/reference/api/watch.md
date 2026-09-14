# WatchService

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

**Types:** `member.join`, `member.left`, `leader.changed`, `lock.expired`, `lease.granted`, `lease.expired`, `lease.revoked`, `custom.<topic>`, `watch.sync`, `watch.gap`.

## See also

- [Watch](../../concepts/watch.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md)
