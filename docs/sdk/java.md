# Java SDK

Artifact `io.clusdr:clusdr`. Java 17+. Blocking gRPC client. Applications call the daemon on this host. Shared model: [SDKs](./).

```xml
<dependency>
  <groupId>io.clusdr</groupId>
  <artifactId>clusdr</artifactId>
  <version>0.1.3</version>
</dependency>
```

Same version train as the daemon.

A running daemon is required ([guide: first member](../guide/first-member.md)).

Contributor checkout: `mvn test` in the `clusdr-java` tree. Proto is compiled at build time from `proto/`.

## Connect

```java
try (Cluster c = Clusdr.local()) {
    // ...
}
```

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`readyTimeout`, default 10s).

```java
Cluster c = Clusdr.dial("127.0.0.1:8947", Options.defaults().dataDir("./data-b"));
```

`dial` is `local()` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local()` there.

Unary methods are fine concurrently. Same connection = same holder (`unlock` is process-wide for that name). Several `watch()` loops on one client are fine.

```java
Clusdr.local(Options.defaults()
    .insecure(false)
    .dataDir("")
    .holder("")
    .requestTimeout(Duration.ofSeconds(10))
    .readyTimeout(Duration.ofSeconds(10))
    .serverName(""));
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `dataDir` is empty |
| `dataDir` | Directory with `ca.crt` / `node.crt` / `node.key` |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>` |
| `requestTimeout` | Unary timeout (default 10s) |
| `readyTimeout` | Health wait on connect (default 10s). Zero skips the wait |
| `serverName` | TLS server name (peer node id). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt` |

Every unary method also takes an optional `Duration timeout` to override `requestTimeout` for that call.

## `Cluster`

```text
members() -> List<Member>
leader() -> Member
watch(filter?) -> Watch (Iterable<Event>)
publish(topic, payload?) -> void
lock(name, ttl?) -> Lock
tryLock(name, ttl?) -> Optional<Lock>
unlock(name) -> void
lease(name, ttl?) -> Lease
renew(name) -> void
revoke(name) -> void
close() -> void
```

Blocking methods. `ttl` is a `Duration` — `null` or zero sends `ttlMs = 0`; the daemon uses its default (15s).

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline.

`close()` (and try-with-resources) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures throw `ClusdrException` (or `IllegalArgumentException` for a bad publish payload).

## Membership

```java
List<Member> members = c.members();
Member leader = c.leader();
```

`Member`: `id`, `address`, `status`, `leader`, `role` (empty wire role becomes `"voter"`).

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → `ClusdrException`.

## Watch

```java
for (Event event : c.watch()) {
    if (event.type().equals("member.left")) {
        // ...
    }
    if (event.type().startsWith("custom.deployment")) {
        event.payload(); // byte[]
    }
}
```

The stream reconnects with `lastSeq` on drop (backoff 50ms → 2s). Closing the `Watch` or `close()` on the cluster ends it.

`topics` / `eventTypes` match the CLI. Empty (default) is the full bus.

```java
for (Event event : c.watch(WatchFilter.all().topics("deployment"))) {
    // ...
}
```

Non-empty `topics`: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → `ClusdrException` before the first event.

`Event`: `type`, `source`, `payload` (`byte[]`), `timestamp` (`Instant`), `seq`.

## Publish

```java
c.publish("deployment", Map.of("sha", "abc"));
c.publish("deployment", "{\"sha\":\"abc\"}");
c.publish("deployment", "{\"sha\":\"abc\"}".getBytes(StandardCharsets.UTF_8));
c.publish("ping"); // empty payload
```

| Java type | On the wire |
|---|---|
| `null` | empty bytes |
| `byte[]` | as-is |
| `String` | UTF-8 |
| `Map` with string keys | compact JSON UTF-8 (string / number / boolean / null values) |
| anything else | `IllegalArgumentException` |

SDK-side cap **64 KiB** (`ClusdrException` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`).

Not on the Raft log. `accepted=false` → `ClusdrException`.

## Locks

Exclusive name on the Raft log. Store `lk.token()` with fenced writes.

```java
Lock lk = c.lock("scheduler", Duration.ofSeconds(15));
try {
    lk.name(); lk.holder(); lk.token(); lk.deadline();
} finally {
    c.unlock("scheduler");
}
```

`lock` blocks until acquired or `timeout`. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```java
Optional<Lock> lk = c.tryLock("scheduler", Duration.ofSeconds(15));
if (lk.isEmpty()) {
    // someone else holds it — not an error
}
```

That matches Python (`None`) and Rust (`Ok(None)`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → `ClusdrException`.

Background renew: timer, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrException`).

No `listLocks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```java
Lease ls = c.lease("worker-1", Duration.ofSeconds(15));
ls.stopRenew(); // grant then expires at ls.deadline(); not a revoke
c.renew("worker-1");
c.revoke("worker-1");
```

`close()` **revokes**. Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline`.

## TLS

On unless `insecure(true)` or `CLUSDR_TLS=disabled` (and no `dataDir`).

PEMs from `dataDir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, the client throws `ClusdrException` and tells you to disable TLS. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK (same as Python, Rust, and TypeScript).

Server name: `serverName`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. gRPC uses `overrideAuthority` with that name. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrException` is the SDK failure type (unchecked). Transient gRPC codes are retried; others throw immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` |
| Bad publish type | `IllegalArgumentException` |
| Publish too large | `ClusdrException` (64 KiB) |
| `tryLock` held by other | `Optional.empty()` |
| Unlock / revoke name you do not hold | `ClusdrException`, no success path |

## Not in this package

- Join, promote, config
- An async / reactive client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md). Rust: [Rust SDK](rust.md). TypeScript: [TypeScript SDK](typescript.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
