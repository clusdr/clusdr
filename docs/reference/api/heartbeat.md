# HeartbeatService

HeartbeatService is the node-to-node liveness ping. Applications do not call this. The daemon uses it as the slower backup to presence: missed pings mark a peer **not-alive** and keep the Raft id. Internal module [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal).

Neither heartbeat misses nor presence expiry call `RemoveServer`. A crash is `clusdr start` with the same `data.dir`, not `join`. Only [`Leave`](join.md) removes the Raft server.

## `Ping`

**Signature:** `Ping(PingRequest) returns (PingResponse)`

**Request:** `sender_id`. **Response:** `node_id`, `alive`.

Defaults: interval 2s, timeout 1s, 3 misses before a peer is marked not-alive. [Presence](../../concepts/presence.md) is the faster liveness path (`lease.presence_ttl`, default 3s).

## See also

- [Presence](../../concepts/presence.md)
- [Configuration](../configuration.md)
