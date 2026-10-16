# Broadcast cache invalidation

Every host keeps a local cache of something it reads often: user records, feature flags, rendered config. When one host changes the source, the others should drop their copy now rather than serve it until it expires. Publish a small event on a topic; every host that watches the topic hears it within one network hop and drops the entry.

<UseMeta />

## The idea

`Publish` hands the event to the local daemon, which emits it as `custom.<topic>` on its own bus and sends it **once** to every alive daemon. Each daemon puts it on its local bus, and every app watching that topic gets it. There is no broker to run and nothing to configure per topic.

<UseFigure name="signals" fig="Fig. A" caption="One publish, one hop to each daemon that is up. Two hosts drop the cached entry; the third has a Watch stream that is reconnecting and does not hear it." />

Custom events are signals, not messages. They are not written to the Raft log and are not replayed, so a host that is not listening at that moment never sees the event. That is fine for invalidation, **as long as the cache does not depend on every signal arriving**. The rules below make sure it does not.

## Publish after the write commits

::: code-group

```go [Go]
// after the transaction that changed user 42 has committed
payload, _ := json.Marshal(map[string]any{"id": 42, "v": newVersion})
if err := c.Publish(ctx, "cache.users", payload); err != nil {
	log.Print(err) // the write is safe; caches will catch up by TTL
}
```

```python [Python]
# after the transaction that changed user 42 has committed
try:
    c.publish("cache.users", {"id": 42, "v": new_version})
except ClusdrError as e:
    print(e)  # the write is safe; caches will catch up by TTL
```

```ts [TypeScript]
// after the transaction that changed user 42 has committed
try {
  await c.publish("cache.users", { id: 42, v: newVersion });
} catch (err) {
  console.error(err); // the write is safe; caches will catch up by TTL
}
```

:::

Publishing before the commit lets another host refill its cache from the old row between the event and the commit.

## Listen, and flush on every reconnect

```go
ch, err := c.Watch(ctx, clusdr.WithTopics("cache.users"))
if err != nil {
	log.Fatal(err)
}
synced := false
for ev := range ch {
	switch ev.Type {
	case "watch.sync":
		if synced {
			cache.Flush() // the stream was down; we may have missed events
		}
		synced = true
	case "custom.cache.users":
		var m struct {
			ID int64 `json:"id"`
			V  int64 `json:"v"`
		}
		if json.Unmarshal(ev.Payload, &m) == nil {
			cache.DropIfOlder(m.ID, m.V)
		}
	}
}
```

The SDK reconnects a dropped Watch on its own. A reconnect sends `watch.sync` again, but custom events from the gap are gone. Treat a second `watch.sync` as "I may have missed something" and flush the topic's cache.

<UseTimeline name="invalidate" fig="Seq. A" caption="host-1 writes user 42 and publishes. host-2 and host-3 drop their copy at once. host-4's stream was reconnecting and misses the event; when the stream is back, the new watch.sync makes it flush." />

## Rules that keep the cache correct

- **Keep a TTL on every entry.** The signal makes caches fresh sooner; the TTL is what bounds staleness when a signal is lost. A partition between two daemons drops the event for the hosts on the other side, and nothing retries it.
- **Carry a version.** Put the row's version or `updated_at` in the payload and drop only entries older than it. Events from different publishers can arrive in different orders.
- **Keep payloads small.** Send the key and version, not the new value. The limit is 64 KiB, and a host that missed the event will reload from the source anyway.
- **One topic per cache.** `cache.users`, `cache.flags`. A watcher with `WithTopics` gets only those topics and no membership snapshot.
- **Make handling idempotent.** Dropping an entry twice is harmless, which is part of why invalidation suits signals. Avoid handlers that do real work per event.

## When to use something else

| You need | Use |
|---|---|
| Every consumer to get every event, even after downtime | A queue or a log (Kafka, NATS JetStream, a table you poll) |
| Events in one global order | The Raft-backed primitives (locks, leases), or your database |
| Large payloads | Your store; publish only the key |
| Fan-out to clients outside the cluster | Your own push channel; apps only talk to their local daemon |

Signals suit jobs where a missed event costs a little staleness, not correctness: cache invalidation, "reload your config", "a deploy just started", nudging idle workers to poll now.

## Related

- [Custom events](../concepts/events.md) for the fan-out rules and limits
- [Watch](../concepts/watch.md) for topics, `watch.sync` and reconnects
- [Watch and publish](../guide/watch.md), the tutorial step that tries this from the CLI
- The [agent example](https://github.com/clusdr/clusdr/tree/main/examples/agent), pub/sub as signals in five languages
