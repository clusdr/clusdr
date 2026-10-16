# Know which workers are alive

A pool of workers takes work from a coordinator: partitions to consume, tenants to serve, tasks to run. The coordinator needs to know when a worker is gone so its work can be handed to someone else. Give each worker a lease named after it. The worker's SDK renews the lease while the process runs; when the process stops, the lease runs out and every watcher gets `lease.expired`.

<UseMeta />

## The idea

- Each worker grants `worker.<id>` with a TTL when it starts. `Lease` never waits: if the name is already held (a second copy of the same worker), the call fails at once.
- The SDK renews in the background about every TTL/3, until the process exits, the lease context is cancelled, or the worker revokes it.
- The coordinator watches `lease.*`. The event's `source` is the lease name; its payload has the `owner` and the `token`.

<UseFigure name="worker-liveness" fig="Fig. A" caption="Two workers renew their leases (the pulsing rings). The third stopped renewing; its lease ran out and the coordinator was told on its Watch stream." />

This is liveness for **processes**, which the cluster's own [presence](../concepts/presence.md) does not give you: presence is per daemon, and one host can run many workers. Use leases when the thing that can die is your process, and [membership](sharding.md) when it is the host.

## The worker

::: code-group

```go [Go]
c, err := clusdr.Local(clusdr.WithHolder("worker-7"))
if err != nil {
	log.Fatal(err)
}
defer c.Close() // revokes the lease, so a clean exit is seen at once

life, stop := context.WithCancel(context.Background())
defer stop()
lease, err := c.Lease(life, "worker.ingest-7", 15*time.Second)
if err != nil {
	log.Fatal(err) // another copy of worker-7 already holds it
}
log.Printf("registered, token %d", lease.Token)
serve(lease.Token)
```

```python [Python]
import threading

from clusdr import local

c = local(holder="worker-7")
stop = threading.Event()
try:
    lease = c.lease("worker.ingest-7", ttl=15, stop=stop)
    print("registered, token", lease.token)
    serve(lease.token)
finally:
    c.close()  # revokes the lease, so a clean exit is seen at once
```

```ts [TypeScript]
import { local } from "clusdr";

const c = await local({ holder: "worker-7" });
try {
  const lease = await c.lease("worker.ingest-7", 15);
  console.log("registered, token", lease.token);
  await serve(lease.token);
} finally {
  await c.close(); // revokes the lease, so a clean exit is seen at once
}
```

:::

## The coordinator

```go
ch, err := c.Watch(ctx, clusdr.WithEventTypes("lease.granted", "lease.expired", "lease.revoked"))
if err != nil {
	log.Fatal(err)
}
for ev := range ch {
	if !strings.HasPrefix(ev.Source, "worker.") {
		continue // presence.<node> and other apps' leases share the table
	}
	id := strings.TrimPrefix(ev.Source, "worker.")
	switch ev.Type {
	case "lease.granted":
		addWorker(id)
	case "lease.expired", "lease.revoked":
		reassign(id) // hand its partitions to the workers still registered
	}
}
```

Filter on your own prefix. The lease table is shared, and the daemons' own liveness leases (`presence.<nodeID>`) show up on the same stream.

<UseTimeline name="lease" fig="Seq. A" caption="worker-7 registers and renews. When it is killed, the renewals stop; after the TTL the lease expires, and the coordinator gets lease.expired and reassigns its work." />

## Starting the coordinator late

Watch opens with a snapshot of members and the leader, not of leases. A coordinator that starts after the workers needs the current list once:

- `clusdr leases` prints every grant with its owner and deadline, for a one-off check.
- From code, call `LeaseService.ListLeases` over gRPC on the local daemon ([API](../reference/api/leases.md)). The SDKs do not wrap it yet.

Read the list, keep the `worker.*` names, then start the Watch and apply events on top. If the stream reconnects with `watch.gap`, read the list again: lease events in the gap are not replayed.

## Hung is not dead

The SDK renews from a background task. A worker whose main loop is stuck (a deadlock, a blocked call with no timeout) can keep its lease alive indefinitely, because the renew task still runs. If that matters, take the renewals into your own hands:

- Grant with a context that your work loop cancels when it stops making progress (Go `life`, Python `stop`, or `lease.stopRenew()` in TypeScript), so the lease lapses on its own.
- Or have a watchdog in the worker call `Revoke` when the loop has not ticked for longer than you accept.

Either way the lease then means "this worker is doing work", not "this process exists".

## Choosing the TTL

| TTL | Detection after a crash | Renew traffic per worker |
|---|---|---|
| 5s | about 5s | one Raft write every ~1.7s |
| 15s (default) | about 15s | one every ~5s |
| 60s | about a minute | one every ~20s |

Every renewal is a write through the Raft leader. A few hundred workers at the default are fine; for thousands, raise the TTL or group workers per host. The lease table holds at most 4096 names in total ([limits](../reference/limits.md)).

## Failure cases

| What happens | Event | Delay |
|---|---|---|
| Worker exits cleanly | `lease.revoked` (from `Close`) | Immediate |
| Worker is killed or its host dies | `lease.expired` | Up to one TTL |
| Worker hangs, renew task still runs | none | Never, unless you tie renewals to progress |
| Second copy of the same worker starts | Its `Lease` call fails | Immediate; it should exit |
| Coordinator's stream drops | `watch.gap` on reconnect if events were missed | Re-read the list |

## Related

- [Leases](../concepts/leases.md) for the API, TTL rules and token
- [Split work across live hosts](sharding.md) for host-level liveness
- [Fence off a stale writer](fencing.md): a lease token fences writes the same way a lock token does
- The [worker example](https://github.com/clusdr/clusdr/tree/main/examples/worker) in five languages
