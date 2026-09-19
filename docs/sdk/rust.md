# Rust SDK

The Rust SDK is a Tokio client that talks to the **local** daemon. The application is not a cluster member: it does not vote or speak Raft.

A daemon must already be running ([guide: first member](../guide/first-member.md)). Shared model: [SDKs](./).

| | Value | Why it matters |
|---|---|---|
| Crate | `clusdr` | `clusdr = "0.2.0"` in `Cargo.toml`. |
| Rust | 1.82+ | Older toolchains fail the edition / crate features this client uses. |
| Runtime | Tokio | There is no blocking (non-Tokio) client — `local().await` needs a Tokio runtime. |
| Version | same train as the daemon | A mismatched crate talks `clusdr.v1alpha1` stubs the running process does not serve. |

```toml
[dependencies]
clusdr = "0.2.0"
```

Contributor checkout: `cargo test` in the `clusdr-rust` tree. `make proto` exports [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (or sibling `../clusdr/proto/api`); `tonic-build` compiles that tree.

## Connect

```rust
use clusdr::{local, Options};

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    let _members = c.members().await?;
    c.close().await?;
    Ok(())
}
```

If this fails, start the **local** daemon and present PEMs from that host’s `data.dir` ([Errors](../reference/errors.md#applications), [TLS](../reference/errors.md#tls)). Rust does not skip-verify the way Go bootstrap TLS does.

`local` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`ready_timeout`, default 10s). On Kubernetes, `127.0.0.1` is the pod — set `CLUSDR_GRPC_ADDR` to the node Runtime, unless the app is a [sidecar](../guide/kubernetes-sidecar.md).

```rust
use clusdr::{dial, Options};

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = dial("127.0.0.1:8947", Options::new().data_dir("./data-b")).await?;
    c.close().await?;
    Ok(())
}
```

`dial` is `local` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local` there. Empty `dial("")` returns `clusdr: empty dial address` ([Errors](../reference/errors.md#applications)).

`Cluster` is cheap to clone (shared gRPC channel, same holder). Several Watch streams on one client are fine.

```rust
use clusdr::{local, Options};
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(
        Options::new()
            .insecure(false)
            .holder("")
            .request_timeout(Duration::from_secs(10))
            .ready_timeout(Duration::from_secs(10)),
    )
    .await?;
    c.close().await?;
    Ok(())
}
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `data_dir` is empty. Required when `start` disabled TLS, or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `data_dir` | Directory with `ca.crt` / `node.crt` / `node.key`. Missing files error; there is no skip-verify fallback. |
| `holder` | Lock/lease identity. Empty → `sdk-<hex>` (UUID without hyphens). Two processes cannot unlock each other unless they share this id ([Errors](../reference/errors.md#locks-and-leases)). |
| `request_timeout` | Unary timeout (default 10s). `lock` waits at most this long. |
| `ready_timeout` | Health wait on connect (default 10s). Zero skips the wait — the first RPC then fails if the daemon is down. |
| `server_name` | TLS server name (peer **node id**). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt`. If none resolve, connect fails ([Errors](../reference/errors.md#tls)). |

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

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the request timeout. Other codes return immediately.

`close` stops Watch, unlocks locks, revokes leases. Failures are `clusdr::Error`.

## Membership

```rust
use clusdr::{local, Options};

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    for m in c.members().await? {
        println!(
            "{} {} status={} role={} leader={}",
            m.id, m.address, m.status, m.role, m.leader
        );
    }
    let leader = c.leader().await?;
    println!("leader {} at {}", leader.id, leader.address);
    c.close().await?;
    Ok(())
}
```

`Member`: `id`, `address`, `status` (`alive` or `dead`), `leader`, `role` (empty wire role becomes `"voter"`). A left id is gone from `members()`.

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → error wrapping Unavailable ([Errors](../reference/errors.md#cluster)).

## Watch

You do not pass `last_seq`. The stream stores `event.seq` and sends it as `last_seq` on reconnect so cluster events resume after a drop. `custom.*` is still not replayed.

```rust
use clusdr::{local, Options, WatchFilter};
use tokio_stream::StreamExt;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    let mut events = c.watch(WatchFilter::default()).await?;
    while let Some(event) = events.next().await {
        let event = event?;
        match event.event_type.as_str() {
            "member.dead" => {
                println!("crash seq={} still listed src={}", event.seq, event.source)
            }
            "member.left" => {
                println!("leave seq={} gone from members src={}", event.seq, event.source)
            }
            "custom.deploy.payments.canary" => {
                println!("gossip seq={} payload={:?}", event.seq, event.payload)
            }
            other => println!("bus seq={} {} src={}", event.seq, other, event.source),
        }
    }
    c.close().await?;
    Ok(())
}
```

The stream reconnects with `last_seq` on drop (backoff 50ms → 2s). Dropping the stream or `close` ends it.

`WatchFilter::topics` / `event_types` match the CLI. Empty (default) is the full bus.

```rust
use clusdr::{local, Options, WatchFilter};
use tokio_stream::StreamExt;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    let mut events = c
        .watch(WatchFilter::new().topics(["deploy.payments.canary"]))
        .await?;
    while let Some(event) = events.next().await {
        let event = event?;
        println!("{} {} {:?}", event.event_type, event.seq, event.payload);
    }
    c.close().await?;
    Ok(())
}
```

Non-empty topics: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → error before the stream starts.

`Event`: `event_type`, `source`, `payload` (`Vec<u8>`), `timestamp` (`SystemTime`), `seq`.

## Publish

```rust
use clusdr::{local, Options};

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    c.publish(
        "deploy.payments.canary",
        br#"{"sha":"7f3a1c2","env":"prod"}"#,
    )
    .await?;
    c.publish("deploy.payments.canary", b"").await?;
    c.close().await?;
    Ok(())
}
```

Payload is bytes. SDK-side cap **64 KiB**. Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`). Over size or `accepted=false` → error ([Errors](../reference/errors.md#applications)).

Not on the Raft log. A Watch reconnect does not replay this signal.

## Locks

Exclusive name on the Raft log. Name it after the job (`scheduler.payments.nightly`). Store `lk.token` with fenced writes.

```rust
use clusdr::{local, Options};
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    let lk = c
        .lock("scheduler.payments.nightly", Some(Duration::from_secs(15)))
        .await?;
    println!(
        "held {} holder={} token={} until={:?}",
        lk.name,
        lk.holder,
        lk.token,
        lk.deadline()
    );
    c.unlock("scheduler.payments.nightly").await?;
    c.close().await?;
    Ok(())
}
```

`lock` blocks until acquired or the request timeout (default 10s). Another holder that keeps the name longer than that returns an error; it does not hang. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```rust
use clusdr::{local, Options};
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    match c
        .try_lock("scheduler.payments.nightly", Some(Duration::from_secs(15)))
        .await?
    {
        Some(lk) => {
            println!("acquired {}", lk.token);
            c.unlock("scheduler.payments.nightly").await?;
        }
        None => println!("held by another replica"),
    }
    c.close().await?;
    Ok(())
}
```

That matches Python (`None`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → error ([Errors](../reference/errors.md#locks-and-leases)).

Background renew: Tokio task, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations. Take the lock on a voter, or `clusdr promote` that node.

No `list_locks` in this crate.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```rust
use clusdr::{local, Options};
use std::time::Duration;

#[tokio::main]
async fn main() -> Result<(), clusdr::Error> {
    let c = local(Options::new()).await?;
    let ls = c
        .lease("worker.payments.ingest-1", Some(Duration::from_secs(15)))
        .await?;
    println!(
        "lease {} owner={} token={} until={:?}",
        ls.name,
        ls.owner,
        ls.token,
        ls.deadline()
    );
    ls.stop_renew();
    c.renew("worker.payments.ingest-1").await?;
    c.revoke("worker.payments.ingest-1").await?;
    c.close().await?;
    Ok(())
}
```

`stop_renew` stops background renew; the grant then expires at `ls.deadline()`. That is not a revoke. `close` **revokes**.

Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline()`.

`renew` / `revoke` of a name this client does not hold → error ([Errors](../reference/errors.md#locks-and-leases)).

## TLS

On unless `insecure(true)` or `CLUSDR_TLS=disabled` (and no `data_dir`).

PEMs from `data_dir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, the client errors and tells you to disable TLS. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK (same as Python) ([Errors](../reference/errors.md#tls)).

Server name: `server_name`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`clusdr::Error` is the SDK failure type. Transient gRPC codes are retried; others return immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` ([Errors](../reference/errors.md#applications)) |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` ([Errors](../reference/errors.md#tls)) |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` ([Errors](../reference/errors.md#tls)) |
| Publish too large | error (64 KiB) ([Errors](../reference/errors.md#applications)) |
| `try_lock` held by other | `Ok(None)` |
| Unlock / revoke name you do not hold | error, no success path ([Errors](../reference/errors.md#locks-and-leases)) |

## Not in this crate

- Join, promote, config
- A blocking (non-Tokio) client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md). TypeScript: [TypeScript SDK](typescript.md). Java: [Java SDK](java.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
