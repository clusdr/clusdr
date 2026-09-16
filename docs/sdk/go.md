# Go SDK

Module [`github.com/durguto/clusdr/sdk`](https://pkg.go.dev/github.com/durguto/clusdr/sdk), package `clusdr`. Applications call the daemon on this host. Shared model: [SDKs](./).

```bash
go get github.com/durguto/clusdr/sdk
```

Same version train as the daemon. Wire types live in `github.com/durguto/clusdr/api`.

**pkg.go.dev** is the API reference (package comment, examples, every exported type). This page is the walkthrough. A running daemon is required ([guide: first member](../guide/first-member.md)).

## Connect

```go
import (
    "context"
    "time"

    "github.com/durguto/clusdr/sdk"
)

c, err := clusdr.Local()
if err != nil {
    // daemon down, TLS, or Health not ready within 10s
}
defer c.Close()
```

`Local` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (10s). That wait is not configurable from a public option.

```go
c, err := clusdr.Dial("127.0.0.1:8947", clusdr.WithDataDir("./data-b"))
```

`Dial` is not wrong; it is `Local` with an explicit address. Use it in tests and when a second daemon on this host listens on another port. Do not `Dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `Local` there. The SDK never meshes with Raft peers.

| Option | Meaning |
|---|---|
| `WithInsecure()` | Plaintext. Same as `CLUSDR_TLS=disabled` |
| `WithDataDir(dir)` | Directory with `ca.crt` / `node.crt` / `node.key` |
| `WithHolder(id)` | Lock/lease identity. Empty → `sdk-<hex>` for this connection |
| `WithRequestTimeout(d)` | Used when the caller context has **no** deadline (default 10s) |

If the caller context already has a deadline, that deadline wins.

## Concurrent use

One `Cluster` is safe from many goroutines: unary RPCs, independent `Watch` calls (each has its own channel and stream), and different lock **names**.

Same connection = same holder. `Unlock("scheduler")` from any goroutine releases that name for the whole process. Two goroutines `Lock`ing the same name both see the same grant (idempotent on this holder); they do not get two exclusive owners.

Do not `range` the same Watch channel from two goroutines unless you want events split between them. `Close` cancels in-flight RPCs and Watch streams.

## `Cluster`

```text
Members(ctx) ([]Member, error)
Leader(ctx) (Member, error)
Watch(ctx, opts ...WatchOption) (<-chan Event, error)
Publish(ctx, topic string, payload []byte) error
Lock(ctx, name string, ttl time.Duration) (*Lock, error)
TryLock(ctx, name string, ttl time.Duration) (lk *Lock, ok bool, err error)
Unlock(ctx, name string) error
Lease(ctx, name string, ttl time.Duration) (*Lease, error)
Renew(ctx, name string) error
Revoke(ctx, name string) error
Close() error
```

Every RPC takes `context.Context`. Unary calls retry `Unavailable`, `Aborted`, and `ResourceExhausted` with backoff 50ms → 2s until the context deadline.

`ttl <= 0` sends `ttl_ms = 0`; the daemon uses its default (15s).

`Close` unlocks locks and revokes leases this connection still holds, then closes the gRPC conn.

## Membership

```go
members, err := c.Members(ctx)
leader, err := c.Leader(ctx)
```

`Member`:

| Field | Meaning |
|---|---|
| `ID` | Stable node id |
| `Address` | Advertised Runtime API |
| `Status` | `alive`, `leaving`, or `dead` |
| `Leader` | True if this id is the current Raft leader |
| `Role` | `voter` or `observer`. Empty from the wire becomes `voter` |

`Leader()` builds a member from `GetLeader` (`Status` is `alive`, `Role` is `voter`). No leader → gRPC `Unavailable`, wrapped as `clusdr: leader: …`.

## Watch

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

ch, err := c.Watch(ctx)
for ev := range ch {
    switch ev.Type {
    case "member.join", "member.left", "leader.changed":
        // cluster
    case "custom.deployment":
        // payload is []byte
    }
}
```

`Watch` returns immediately. A goroutine reads the stream. Cancel `ctx` to stop; the channel is then closed. On disconnect the loop reconnects (backoff 50ms → 2s).

Zero options is the full bus (snapshot, `watch.sync`, live events). `WithTopics` / `WithEventTypes` match the CLI:

```go
ch, err := c.Watch(ctx, clusdr.WithTopics("deployment"))
```

Non-empty topics: only `custom.<topic>` for those keys; **membership snapshot is omitted**. Pass `"deployment"` or `"custom.deployment"`. `WithEventTypes("member.join")` matches full type strings. Protocol events (`watch.sync`, `watch.gap`) always pass. Reconnects reuse the same filter and `last_seq`. Custom events are still not replayed. `last_seq` is bus-global, not per-topic.

Invalid topic characters fail `Watch` before the stream starts (`clusdr: watch topic …`).

Channel buffer is **64**. A slow receiver **blocks** the read loop (it does not drop on the client). The daemon bus still drops slow subscribers.

`Event`: `Type`, `Source`, `Payload` (`[]byte`), `Timestamp`, `Seq`.

## Publish

```go
err := c.Publish(ctx, "deployment", []byte(`{"sha":"abc"}`))
```

Payload is raw bytes. Max 64 KiB (daemon). Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`.

Not on the Raft log. `Accepted=false` → `clusdr: publish rejected: …`. Duplicate `event_id` is accepted with `message = duplicate` at the wire; this helper only fails when `accepted` is false.

## Locks

A lock is an exclusive name on the Raft log. Store `lk.Token` with any write that must be fenced. A stale holder cannot unlock a newer grant.

```go
lk, err := c.Lock(ctx, "scheduler", 15*time.Second)
if err != nil { /* deadline, observer, name, table full */ }
defer c.Unlock(ctx, "scheduler")

// later
_ = lk.Name
_ = lk.Holder
_ = lk.Token
_ = lk.Deadline()
```

`Lock` blocks until acquired or `ctx` is done. Calling `Lock` again for a name this connection already holds returns the existing `*Lock` and does not re-RPC.

```go
lk, ok, err := c.TryLock(ctx, "scheduler", 15*time.Second)
if err != nil { /* transport */ }
if !ok {
    // someone else holds it — not an error
    // lk may still describe the current holder
}
```

`Unlock` of a name this client does not hold → `clusdr: lock "…" is not held by this client`.

Background renew starts after acquire, interval about TTL/3 (minimum 50ms). `FailedPrecondition` or `Canceled` on renew stops the loop. `Close` and `Unlock` stop renew and release.

Observer daemon: Lock / TryLock / Unlock / Renew → `FailedPrecondition` (`observer cannot mutate locks`). List is a CLI/RPC concern; this SDK has no `ListLocks`.

Name rules and table size: [locks](../concepts/locks.md), [limits](../reference/limits.md).

## Leases

A lease is a named TTL grant. `Grant` never blocks. Many names can be held at once.

```go
life, stop := context.WithCancel(ctx)
lease, err := c.Lease(life, "worker-1", 15*time.Second)
if err != nil { /* already owned, name, table */ }

// cancel life → renew stops; grant expires at Deadline. Not a revoke.
stop()

err = c.Renew(ctx, "worker-1")
err = c.Revoke(ctx, "worker-1")
```

`Lease`’s first argument is both the grant RPC context (until the RPC returns) and the **lifetime** of background renew. Cancelling it after a successful grant stops renew only.

`Close` **revokes** remaining leases. That is not the same as cancelling the lease context.

`Renew` / `Revoke` of a name this client does not hold → error.

Observers **can** grant leases. `presence.<nodeID>` is the daemon’s own liveness lease — not an application grant.

`Lease`: `Name`, `Owner`, `Token`, `Deadline()`.

## TLS

On unless `CLUSDR_TLS=disabled` or `WithInsecure()`.

Lookup order for PEMs: `WithDataDir`, else `CLUSDR_DATA_DIR`, else `~/.clusdr`.

If that directory has no usable certs, the client falls back to **bootstrap TLS** (TLS 1.2+, skip hostname, no client cert). That is enough to reach a daemon that still accepts join-style TLS; a cluster that requires a node cert will reject you.

When PEMs load: client cert + cluster CA. Peer identity is the CA, not the dial hostname (`InsecureSkipVerify` + `VerifyPeerCertificate` against the CA).

## Errors

Returned errors are wrapped (`clusdr: members: …`, `clusdr: lock "name": …`). Unwrap to the gRPC status when you need the code.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` or `clusdr: dial …` |
| Empty `Dial("")` | `clusdr: empty dial address` |
| Transient RPC | retried until deadline |
| No leader | `Unavailable` on `Leader` |
| Observer + lock | `FailedPrecondition` |
| Unlock / revoke name you do not hold | SDK error, no RPC |

## Not in this package

- `ListLocks` / `ListLeases`
- Join, promote, config
- A public `WithReadyTimeout`

Wire shapes: [gRPC API](../reference/api/). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
