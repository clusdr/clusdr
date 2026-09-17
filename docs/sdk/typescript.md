# TypeScript SDK

Package `clusdr` on npm. Node.js 20+. Applications call the daemon on this host. Shared model: [SDKs](./).

```bash
npm install clusdr
```

Same version train as the daemon.

A running daemon is required ([guide: first member](../guide/first-member.md)).

Contributor checkout: `npm test` in the `clusdr-js` tree. `make proto` exports [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (or sibling `../clusdr/proto/api`); the client loads that tree at runtime.

## Connect

```ts
import { local } from "clusdr";

const c = await local();
// ...
await c.close();
```

Or `await using`:

```ts
import { local } from "clusdr";

await using c = await local();
console.log(await c.members());
```

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`readyTimeout`, default 10s).

```ts
import { dial } from "clusdr";

const c = await dial("127.0.0.1:8947", { dataDir: "./data-b" });
```

`dial` is `local()` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local()` there.

Unary methods are fine concurrently. Same connection = same holder (`unlock` is process-wide for that name). Several `watch()` loops on one client are fine.

```ts
await local({
  insecure: false,
  dataDir: "",
  holder: "",
  requestTimeout: 10,
  readyTimeout: 10,
  serverName: "",
});
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `dataDir` is empty |
| `dataDir` | Directory with `ca.crt` / `node.crt` / `node.key` |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>` |
| `requestTimeout` | Unary timeout in seconds (default 10) |
| `readyTimeout` | Health wait on connect (default 10). `0` skips the wait |
| `serverName` | TLS server name (peer node id). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt` |

Every unary method also takes `timeout?: number` to override `requestTimeout` for that call.

## `Cluster`

```text
members(timeout?) -> Member[]
leader(timeout?) -> Member
watch(filter?) -> AsyncIterable<Event>
publish(topic, payload?, timeout?) -> void
lock(name, ttl?, timeout?) -> Lock
tryLock(name, ttl?, timeout?) -> Lock | null
unlock(name, timeout?) -> void
lease(name, ttl?, timeout?) -> Lease
renew(name, timeout?) -> void
revoke(name, timeout?) -> void
close() -> void
```

Every method is async except `watch`, which returns an async iterable immediately. `ttl` is seconds — `undefined` or `<= 0` sends `ttlMs = 0`; the daemon uses its default (15s).

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline.

`close()` (and disposing with `await using`) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures throw `ClusdrError` (or `TypeError` for a bad publish payload).

## Membership

```ts
const members = await c.members();
const leader = await c.leader();
```

`Member`: `id`, `address`, `status` (`alive` or `dead`), `leader`, `role` (empty wire role becomes `"voter"`).

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → `ClusdrError` wrapping Unavailable.

## Watch

```ts
for await (const event of c.watch()) {
  if (event.type === "member.dead") {
    // crash / miss; still in members()
  }
  if (event.type === "member.left") {
    // clusdr leave; gone from members()
  }
  if (event.type === "custom.deployment") {
    event.payload; // Uint8Array
  }
}
```

The stream reconnects with `lastSeq` on drop (backoff 50ms → 2s). Breaking the loop or `close()` ends it.

`topics` / `eventTypes` match the CLI. Empty (default) is the full bus.

```ts
for await (const event of c.watch({ topics: ["deployment"] })) {
  // ...
}
```

Non-empty `topics`: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → `ClusdrError` before the first event.

`Event`: `type`, `source`, `payload` (`Uint8Array`), `timestamp` (`Date`), `seq`.

## Publish

```ts
await c.publish("deployment", { sha: "abc" });
await c.publish("deployment", '{"sha":"abc"}');
await c.publish("deployment", Buffer.from('{"sha":"abc"}'));
await c.publish("ping"); // empty payload
```

| TypeScript type | On the wire |
|---|---|
| `undefined` / `null` | empty bytes |
| `Uint8Array` / `Buffer` | as-is |
| `string` | UTF-8 |
| object / array | compact JSON UTF-8 |
| anything else | `TypeError` |

SDK-side cap **64 KiB** (`ClusdrError` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`).

Not on the Raft log. `accepted=false` → `ClusdrError`.

## Locks

Exclusive name on the Raft log. Store `lk.token` with fenced writes.

```ts
const lk = await c.lock("scheduler", 15);
try {
  void [lk.name, lk.holder, lk.token, lk.deadline];
} finally {
  await c.unlock("scheduler");
}
```

`lock` blocks until acquired or `timeout`. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```ts
const lk = await c.tryLock("scheduler", 15);
if (lk === null) {
  // someone else holds it — not an error
}
```

That matches Python (`None`) and Rust (`Ok(None)`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → `ClusdrError`.

Background renew: timer, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrError`).

No `listLocks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```ts
const ls = await c.lease("worker-1", 15);
ls.stopRenew(); // grant then expires at ls.deadline; not a revoke
await c.renew("worker-1");
await c.revoke("worker-1");
```

`close()` **revokes**. Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline`.

## TLS

On unless `insecure: true` or `CLUSDR_TLS=disabled` (and no `dataDir`).

PEMs from `dataDir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, the client throws `ClusdrError` and tells you to disable TLS. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK (same as Python and Rust).

Server name: `serverName`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. gRPC uses `grpc.ssl_target_name_override` with that name. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrError` is the SDK failure type. Transient gRPC codes are retried; others throw immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` |
| Bad publish type | `TypeError` |
| Publish too large | `ClusdrError` (64 KiB) |
| `tryLock` held by other | `null` |
| Unlock / revoke name you do not hold | `ClusdrError`, no success path |

## Not in this package

- Join, promote, config
- A blocking (non-async) client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md). Rust: [Rust SDK](rust.md). Java: [Java SDK](java.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
