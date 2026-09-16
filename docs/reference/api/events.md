# EventService

## `PublishEvent`

**Signature:** `PublishEvent(PublishEventRequest) returns (PublishEventResponse)`

**Request:** `topic` (1–128: `A–Z a–z 0–9 . _ -`), `payload` (max 64 KiB), optional `event_id` / `source` / `relay` (node-to-node).

**Response:** `accepted`, `message`, `event_id`, `type` (`custom.<topic>`). Duplicate `event_id` → `accepted` with `message = duplicate`.

Not Raft-replicated.

## See also

- [Custom events](../../concepts/events.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md)
