# HeartbeatService

Node-to-node liveness pings. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal). Applications do not call this.

## `Ping`

**Signature:** `Ping(PingRequest) returns (PingResponse)`

**Request:** `sender_id`. **Response:** `node_id`, `alive`.

Defaults: interval 2s, timeout 1s, 3 misses before a peer is marked leaving. [Presence](../../concepts/presence.md) is the faster dead path.

## See also

- [Presence](../../concepts/presence.md)
- [Configuration](../configuration.md)
