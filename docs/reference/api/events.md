# EventService

EventService publishes custom cluster-wide events. Use it from apps and [`clusdr publish`](../../reference/cli/publish.md) to fan out an opaque payload as `custom.<topic>` on every alive peer’s Watch stream. Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

A PublishEvent call is accepted on any node, including observers. Custom events are **not** Raft-replicated and are not replayed on Watch reconnect. Over 64 KiB or an illegal topic, the RPC is rejected ([errors](../errors.md#applications)).

## `PublishEvent`

**Signature:** `PublishEvent(PublishEventRequest) returns (PublishEventResponse)`

**Request:** `topic` (1–128: `A–Z a–z 0–9 . _ -`), `payload` (max 64 KiB), optional `event_id` / `source` / `relay` (node-to-node).

**Response:** `accepted`, `message`, `event_id`, `type` (`custom.<topic>`). Duplicate `event_id` → `accepted` with `message = duplicate`.

Empty `event_id` / `source` on a client call: the receiving node fills them. Peers emit locally and do not re-fanout (`relay = true`).

Dial the Runtime API (`grpc.addr`). Transport failures: [errors](../errors.md#daemon-and-dial).

## See also

- [Custom events](../../concepts/events.md)
- [Go SDK](../../sdk/go.md) · [Python SDK](../../sdk/python.md) · [Rust SDK](../../sdk/rust.md) · [TypeScript SDK](../../sdk/typescript.md) · [Java SDK](../../sdk/java.md)
