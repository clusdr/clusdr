# TypeScript SDK

The TypeScript SDK talks to the **local** daemon from a Node.js process. The application is not a cluster member: it does not vote or speak Raft.

A daemon must already be running ([guide: first member](../guide/first-member.md)). Shared model: [SDKs](./).

| | Value | Why it matters |
|---|---|---|
| Package | `clusdr` on npm | `npm install clusdr`. |
| Runtime | Node.js 20+ | Older Node lacks the runtime APIs this client uses; there is no blocking (non-async) client. |
| Version | same train as the daemon | A mismatched install talks `clusdr.v1alpha1` stubs the running process does not serve. |

```bash
npm install clusdr
```

Contributor checkout: `npm test` in the `clusdr-js` tree. `make proto` exports [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (or sibling `../clusdr/proto/api`); the client loads that tree at runtime.

## Connect

```ts
import { ClusdrError, local } from "clusdr";

const c = await local().catch((err: unknown) => {
  const message = err instanceof ClusdrError ? err.message : String(err);
  throw new Error(`connect failed: ${message}`, { cause: err });
});
console.log(await c.members());
await c.close();
```

Or `await using` (`close()` runs on dispose):

```ts
import { ClusdrError, local } from "clusdr";

try {
  await using c = await local();
  console.log(await c.members());
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(`connect failed: ${err.message}`, { cause: err });
  }
  throw err;
}
```

If this fails, start the **local** daemon and present PEMs from that host’s `data.dir` ([Errors](../reference/errors.md#applications), [TLS](../reference/errors.md#tls)). TypeScript does not skip-verify the way Go bootstrap TLS does.

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`readyTimeout`, default 10s). On Kubernetes, `127.0.0.1` is the pod — set `CLUSDR_GRPC_ADDR` to the node Runtime, unless the app is a [sidecar](../guide/kubernetes-sidecar.md).

```ts
import { ClusdrError, dial } from "clusdr";

const c = await dial("127.0.0.1:8947", { dataDir: "./data-b" }).catch((err: unknown) => {
  const message = err instanceof ClusdrError ? err.message : String(err);
  throw new Error(`connect failed: ${message}`, { cause: err });
});
await c.close();
```

`dial` is `local()` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local()` there. Empty `dial("")` throws `clusdr: empty dial address` ([Errors](../reference/errors.md#applications)).

Unary methods are fine concurrently. Same connection = same holder (`unlock` is process-wide for that name). Several `watch()` loops on one client are fine.

```ts
import { local } from "clusdr";

const c = await local({
  insecure: false,
  dataDir: "",
  holder: "",
  requestTimeout: 10,
  readyTimeout: 10,
  serverName: "",
});
await c.close();
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `dataDir` is empty. Required when `start` disabled TLS, or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `dataDir` | Directory with `ca.crt` / `node.crt` / `node.key`. Missing files throw; there is no skip-verify fallback. |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>`. Two processes cannot unlock each other unless they share this id ([Errors](../reference/errors.md#locks-and-leases)). |
| `requestTimeout` | Unary timeout in seconds (default 10). `lock` waits at most this long unless you pass `timeout`. |
| `readyTimeout` | Health wait on connect (default 10). `0` skips the wait — the first RPC then fails if the daemon is down. |
| `serverName` | TLS server name (peer **node id**). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt`. If none resolve, connect fails ([Errors](../reference/errors.md#tls)). |

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

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline. Other codes throw immediately.

`close()` (and disposing with `await using`) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures throw `ClusdrError` (or `TypeError` for a bad publish payload).

## Membership

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  for (const m of await c.members()) {
    console.log(`${m.id} ${m.address} status=${m.status} role=${m.role} leader=${m.leader}`);
  }
  const leader = await c.leader();
  console.log(`leader ${leader.id} at ${leader.address}`);
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

`Member`: `id`, `address`, `status` (`alive` or `dead`), `leader`, `role` (empty wire role becomes `"voter"`). A left id is gone from `members()`.

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → `ClusdrError` wrapping Unavailable ([Errors](../reference/errors.md#cluster)).

## Watch

You do not pass `lastSeq`. The iterable stores `event.seq` and sends it as `lastSeq` on reconnect so cluster events resume after a drop. `custom.*` is still not replayed.

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  for await (const event of c.watch()) {
    if (event.type === "member.dead") {
      console.log(`crash seq=${event.seq} still listed src=${event.source}`);
    } else if (event.type === "member.left") {
      console.log(`leave seq=${event.seq} gone from members src=${event.source}`);
    } else if (event.type === "custom.deploy.payments.canary") {
      console.log(`gossip seq=${event.seq} payload=${Buffer.from(event.payload).toString("utf8")}`);
    } else {
      console.log(`bus seq=${event.seq} ${event.type} src=${event.source}`);
    }
  }
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

The stream reconnects with `lastSeq` on drop (backoff 50ms → 2s). Breaking the loop or `close()` ends it.

`topics` / `eventTypes` match the CLI. Empty (default) is the full bus.

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  for await (const event of c.watch({ topics: ["deploy.payments.canary"] })) {
    console.log(event.type, event.seq, Buffer.from(event.payload).toString("utf8"));
  }
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

Non-empty `topics`: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → `ClusdrError` before the first event.

`Event`: `type`, `source`, `payload` (`Uint8Array`), `timestamp` (`Date`), `seq`.

## Publish

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  await c.publish("deploy.payments.canary", { sha: "7f3a1c2", env: "prod" });
  await c.publish("deploy.payments.canary", '{"sha":"7f3a1c2","env":"prod"}');
  await c.publish("deploy.payments.canary", Buffer.from('{"sha":"7f3a1c2","env":"prod"}'));
  await c.publish("deploy.payments.canary");
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

| TypeScript type | On the wire |
|---|---|
| `undefined` / `null` | empty bytes |
| `Uint8Array` / `Buffer` | as-is |
| `string` | UTF-8 |
| object / array | compact JSON UTF-8 |
| anything else | `TypeError` |

SDK-side cap **64 KiB** (`ClusdrError` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`). Over size or `accepted=false` → `ClusdrError` ([Errors](../reference/errors.md#applications)).

Not on the Raft log. A Watch reconnect does not replay this signal.

## Locks

Exclusive name on the Raft log. Name it after the job (`scheduler.payments.nightly`). Store `lk.token` with fenced writes.

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  const lk = await c.lock("scheduler.payments.nightly", 15);
  console.log(lk.name, lk.holder, lk.token, lk.deadline);
  await c.unlock("scheduler.payments.nightly");
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

`lock` blocks until acquired or `timeout` (default `requestTimeout`, 10s). Another holder that keeps the name longer than that throws; it does not hang. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  const lk = await c.tryLock("scheduler.payments.nightly", 15);
  if (lk === null) {
    console.log("held by another replica");
  } else {
    console.log("acquired", lk.token);
    await c.unlock("scheduler.payments.nightly");
  }
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

That matches Python (`None`) and Rust (`Ok(None)`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → `ClusdrError` ([Errors](../reference/errors.md#locks-and-leases)).

Background renew: timer, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrError`). Take the lock on a voter, or `clusdr promote` that node.

No `listLocks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```ts
import { ClusdrError, local } from "clusdr";

const c = await local();
try {
  const ls = await c.lease("worker.payments.ingest-1", 15);
  console.log(ls.name, ls.owner, ls.token, ls.deadline);
  ls.stopRenew();
  await c.renew("worker.payments.ingest-1");
  await c.revoke("worker.payments.ingest-1");
} catch (err) {
  if (err instanceof ClusdrError) {
    throw new Error(err.message, { cause: err });
  }
  throw err;
} finally {
  await c.close();
}
```

`stopRenew` stops background renew; the grant then expires at `ls.deadline`. That is not a revoke. `close()` **revokes**.

Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline`.

`renew` / `revoke` of a name this client does not hold → `ClusdrError` ([Errors](../reference/errors.md#locks-and-leases)).

## TLS

On unless `insecure: true` or `CLUSDR_TLS=disabled` (and no `dataDir`).

PEMs from `dataDir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, the client throws `ClusdrError` and tells you to disable TLS. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK (same as Python and Rust) ([Errors](../reference/errors.md#tls)).

Server name: `serverName`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. gRPC uses `grpc.ssl_target_name_override` with that name. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrError` is the SDK failure type. Transient gRPC codes are retried; others throw immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` ([Errors](../reference/errors.md#applications)) |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` ([Errors](../reference/errors.md#tls)) |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` ([Errors](../reference/errors.md#tls)) |
| Bad publish type | `TypeError` |
| Publish too large | `ClusdrError` (64 KiB) ([Errors](../reference/errors.md#applications)) |
| `tryLock` held by other | `null` |
| Unlock / revoke name you do not hold | `ClusdrError`, no success path ([Errors](../reference/errors.md#locks-and-leases)) |

## Not in this package

- Join, promote, config
- A blocking (non-async) client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md). Rust: [Rust SDK](rust.md). Java: [Java SDK](java.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
