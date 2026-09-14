# HeartbeatService

Node-to-node liveness pings. Applications do not call this.

Defaults: interval 2s, timeout 1s, 3 misses before a peer is marked leaving. [Presence](../../concepts/presence.md) is the faster dead path.

## See also

- [Presence](../../concepts/presence.md)
- [Configuration](../configuration.md)
