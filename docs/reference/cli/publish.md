# `clusdr publish`

`clusdr publish` sends a custom event on a topic. Use it to fan out an opaque payload (usually JSON text) to every alive peer’s Watch stream as `custom.<topic>`. Optional payload is raw bytes. Max 64 KiB.

Custom events are **not** Raft-replicated and are not replayed on Watch reconnect. If you need a durable audit trail, write that elsewhere. Over 64 KiB or an illegal topic, the RPC is rejected ([errors](../errors.md#applications)).

## Synopsis

```bash
clusdr publish deployment
clusdr publish deployment '{"sha":"abc"}'
clusdr publish --config /etc/clusdr/clusdr.yaml deployment
```

Topic: `1–128` of `A–Z a–z 0–9 . _ -`. The CLI dials `grpc.addr` (default `127.0.0.1:7947`).

Success prints `published  type=custom.<topic>  event_id=…`.

## Errors

| You see | What to do |
|---|---|
| `publish rejected` / payload too large | Shrink the payload below 64 KiB; check the topic charset ([errors](../errors.md#applications)) |
| `dial daemon at …` | Runtime is down ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Custom events](../../concepts/events.md)
- [EventService](../api/events.md)
