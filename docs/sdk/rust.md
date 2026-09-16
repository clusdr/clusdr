# Rust SDK

Crate `clusdr`. Rust 1.82+. Tokio. Applications call the daemon on this host. Shared model: [SDKs](./).

```toml
[dependencies]
clusdr = { git = "https://github.com/clusdr/clusdr-rust" }
```

Same version train as the daemon.

A running daemon is required ([guide: first member](../guide/first-member.md)).

Contributor checkout: `cargo test` in the `clusdr-rust` tree. Proto is compiled at build time from `proto/`.

## Connect

```rust
use clusdr::{local, Options};

let c = local(Options::new()).await?;
// ...
c.close().await?;
```

`local` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`ready_timeout`, default 10s).

```rust
use clusdr::{dial, Options};

let c = dial("127.0.0.1:8947", Options::new().data_dir("./data-b")).await?;
```

`dial` is `local` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local` there.

`Cluster` is cheap to clone (shared gRPC channel, same holder). Several Watch streams on one client are fine.

```rust
local(Options::new()
    .insecure(false)
    .holder("")
    .request_timeout(Duration::from_secs(10))
    .ready_timeout(Duration::from_secs(10)))
.await?;
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `data_dir` is empty |
| `data_dir` | Directory with `ca.crt` / `node.crt` / `node.key` |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>` |
| `request_timeout` | Unary timeout (default 10s) |
| `ready_timeout` | Health wait on connect (default 10s). Zero skips the wait |
| `server_name` | TLS server name (peer node id). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt` |

## `Cluster`

```text
members() -> Vec<Member>
leader() -> Member
watch(WatchFilter) -> EventStream
publish(topic, payload) -> ()
lock(name, ttl) -> Arc<Lock>
try_lock(name, ttl) -> Option<Arc<Lock>>
unlock(name) -> ()
lease(name, ttl) -> Arc<Lease>
renew(name) -> ()
revoke(name) -> ()
close() -> ()
```

Every method is async. `ttl: Option<Duration>` — `None` or zero sends `ttl_ms = 0`; the daemon uses its default (15s).

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the request timeout.

`close` stops Watch, unlocks locks, revokes leases. Failures are `clusdr::Error`.

## Membership

```rust
let members = c.members().await?;
let leader = c.leader().await?;
```

`Member`: `id`, `address`, `status`, `leader`, `role` (empty wire role becomes `"voter"`).

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → error wrapping Unavailable.

## Watch

```rust
use clusdr::WatchFilter;
use tokio_stream::StreamExt;

let mut events = c.watch(WatchFilter::default()).await?;
while let Some(event) = events.next().await {
    let event = event?;
    match event.event_type.as_str() {
        "member.left" => {}
        t if t.starts_with("custom.") => {
            let _ = event.payload;
        }
        _ => {}
    }
}
```

The stream reconnects with `last_seq` on drop (backoff 50ms → 2s). Dropping the stream or `close` ends it.

`WatchFilter::topics` / `event_types` match the CLI. Empty (default) is the full bus.

```rust
let mut events = c.watch(WatchFilter::new().topics(["deployment"])).await?;
```

Non-empty topics: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → error before the stream starts.

`Event`: `event_type`, `source`, `payload` (`Vec<u8>`), `timestamp` (`SystemTime`), `seq`.

## Publish

```rust
c.publish("deployment", br#"{"sha":"abc"}"#).await?;
c.publish("ping", b"").await?;
```

Payload is bytes. SDK-side cap **64 KiB**. Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`).

Not on the Raft log. `accepted=false` → error.

## Locks

Exclusive name on the Raft log. Store `lk.token` with fenced writes.

```rust
let lk = c.lock("scheduler", Some(Duration::from_secs(15))).await?;
let _ = (lk.name.as_str(), lk.holder.as_str(), lk.token, lk.deadline());
c.unlock("scheduler").await?;
```

`lock` blocks until acquired or the request timeout. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```rust
match c.try_lock("scheduler", Some(Duration::from_secs(15))).await? {
    Some(lk) => { let _ = lk; }
    None => { /* someone else holds it — not an error */ }
}
```

That matches Python (`None`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → error.

Background renew: Tokio task, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations.

No `list_locks` in this crate.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```rust
let ls = c.lease("worker-1", Some(Duration::from_secs(15))).await?;
ls.stop_renew(); // grant then expires at ls.deadline(); not a revoke
c.renew("worker-1").await?;
c.revoke("worker-1").await?;
```

`close` **revokes**. Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline()`.

## TLS

On unless `insecure(true)` or `CLUSDR_TLS=disabled` (and no `data_dir`).

PEMs from `data_dir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, the client errors and tells you to disable TLS. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK (same as Python).

Server name: `server_name`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`clusdr::Error` is the SDK failure type. Transient gRPC codes are retried; others return immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` |
| Publish too large | error (64 KiB) |
| `try_lock` held by other | `Ok(None)` |
| Unlock / revoke name you do not hold | error, no success path |

## Not in this crate

- Join, promote, config
- A blocking (non-Tokio) client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md).
