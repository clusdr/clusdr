# Go SDK

The Go SDK talks to the **local** daemon from your process. The application is not a cluster member: it does not vote or speak Raft. Every exported type lives on [pkg.go.dev](https://pkg.go.dev/github.com/clusdr/clusdr/sdk); the snippets below are the pasteable walkthrough.

A daemon must already be running ([guide: first member](../guide/first-member.md)). Shared model: [SDKs](./).

| | Value | Why it matters |
|---|---|---|
| Module | [`github.com/clusdr/clusdr/sdk`](https://pkg.go.dev/github.com/clusdr/clusdr/sdk) | `go get` this path. Wire types are `github.com/clusdr/clusdr/api`. |
| Package | `clusdr` | Import name in application code. |
| Version | same train as the daemon | A mismatched SDK talks `clusdr.v1alpha1` stubs the running process does not serve. |

```bash
go get github.com/clusdr/clusdr/sdk
```

## Connect

```go
package main

import (
	"log"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err) // daemon down, TLS mismatch, or Health not ready within 10s
	}
	defer c.Close()
}
```

If this fails, start the **local** daemon and match TLS to `clusdr start` ([Errors](../reference/errors.md#applications), [TLS](../reference/errors.md#tls)).

`Local` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (10s). That wait is not configurable from a public option. On Kubernetes, `127.0.0.1` is the pod — set `CLUSDR_GRPC_ADDR` to the node Runtime, unless the app is a [sidecar](../guide/kubernetes-sidecar.md) ([Kubernetes](../concepts/kubernetes.md)). An unset address in a Deployment pod fails Health even when the node daemon is up.

```go
package main

import (
	"log"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	// Second daemon on this host (its own grpc.addr + data.dir), or a test listener.
	c, err := clusdr.Dial("127.0.0.1:8947", clusdr.WithDataDir("./data-b"))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
}
```

`Dial` is `Local` with an explicit address. Use it in tests and when a second daemon on this host listens on another port. Do not `Dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `Local` there. Empty `Dial("")` returns `clusdr: empty dial address` ([Errors](../reference/errors.md#applications)). The SDK never meshes with Raft peers.

| Option | Meaning |
|---|---|
| `WithInsecure()` | Plaintext. Same as `CLUSDR_TLS=disabled`. Required when `start` disabled TLS, or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `WithDataDir(dir)` | Directory with `ca.crt` / `node.crt` / `node.key`. Point it at **this host’s** `data.dir` so the app presents the cluster CA. |
| `WithHolder(id)` | Lock/lease identity. Empty → `sdk-<hex>` for this connection. Two processes cannot unlock each other unless they share this id ([Errors](../reference/errors.md#locks-and-leases)). |
| `WithRequestTimeout(d)` | Used when the caller context has **no** deadline (default 10s). `Lock` and unary RPCs then wait at most this long. |

If the caller context already has a deadline, that deadline wins.

## Concurrent use

One `Cluster` is safe from many goroutines: unary RPCs, independent `Watch` calls (each has its own channel and stream), and different lock **names**.

Same connection = same holder. `Unlock("scheduler.payments.nightly")` from any goroutine releases that name for the whole process. Two goroutines `Lock`ing the same name both see the same grant (idempotent on this holder); they do not get two exclusive owners.

Do not `range` the same Watch channel from two goroutines unless you want events split between them. `Close` cancels in-flight RPCs and Watch streams, then unlocks and revokes what this connection still holds so a process exit does not wait for TTL.

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

Every RPC takes `context.Context`. Unary calls retry `Unavailable`, `Aborted`, and `ResourceExhausted` with backoff 50ms → 2s until the context deadline. Other codes fail immediately so an observer lock or a bad name is not hidden by retry.

`ttl <= 0` sends `ttl_ms = 0`; the daemon uses its default (15s).

`Close` unlocks locks and revokes leases this connection still holds, then closes the gRPC conn.

## Membership

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	members, err := c.Members(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range members {
		fmt.Printf("%s %s status=%s role=%s leader=%v\n", m.ID, m.Address, m.Status, m.Role, m.Leader)
	}

	leader, err := c.Leader(ctx)
	if err != nil {
		log.Fatal(err) // no current leader → Unavailable
	}
	fmt.Printf("leader %s at %s\n", leader.ID, leader.Address)
}
```

`Members` is the last applied list on this daemon (it can be stale on a partitioned follower). Compare it to `clusdr members` on the same host.

`Member`:

| Field | Meaning |
|---|---|
| `ID` | Stable node id |
| `Address` | Advertised Runtime API |
| `Status` | `alive` or `dead` (liveness). A left id is gone from the list |
| `Leader` | True if this id is the current Raft leader |
| `Role` | `voter` or `observer`. Empty from the wire becomes `voter` |

`Leader()` builds a member from `GetLeader` (`Status` is `alive`, `Role` is `voter`). No leader → gRPC `Unavailable`, wrapped as `clusdr: leader: …` ([Errors](../reference/errors.md#cluster)).

## Watch

You do not pass `last_seq`. The SDK stores `Event.Seq` and sends it as `last_seq` on reconnect so cluster events resume after a drop. `custom.*` is still not replayed.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch, err := c.Watch(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for ev := range ch {
		switch ev.Type {
		case "member.join", "leader.changed":
			fmt.Printf("cluster seq=%d %s src=%s\n", ev.Seq, ev.Type, ev.Source)
		case "member.dead":
			fmt.Printf("crash seq=%d still listed src=%s\n", ev.Seq, ev.Source)
		case "member.left":
			fmt.Printf("leave seq=%d gone from members src=%s\n", ev.Seq, ev.Source)
		case "custom.deploy.payments.canary":
			fmt.Printf("gossip seq=%d payload=%s\n", ev.Seq, ev.Payload)
		case "watch.sync", "watch.gap":
			fmt.Printf("watch seq=%d %s\n", ev.Seq, ev.Type)
		default:
			fmt.Printf("bus seq=%d %s src=%s\n", ev.Seq, ev.Type, ev.Source)
		}
	}
}
```

`Watch` returns immediately. A goroutine reads the stream. Cancel `ctx` to stop; the channel is then closed. On disconnect the loop reconnects (backoff 50ms → 2s) with the last `Seq` it saw.

Zero options is the full bus (snapshot, `watch.sync`, live events). `WithTopics` / `WithEventTypes` match the CLI:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch, err := c.Watch(ctx, clusdr.WithTopics("deploy.payments.canary"))
	if err != nil {
		log.Fatal(err) // invalid topic characters fail here, before the stream starts
	}
	for ev := range ch {
		fmt.Println(ev.Type, ev.Seq, string(ev.Payload))
	}
}
```

Non-empty topics: only `custom.<topic>` for those keys; **membership snapshot is omitted**, so this listener will not see `member.dead`. Pass `"deploy.payments.canary"` or `"custom.deploy.payments.canary"`. `WithEventTypes("member.join")` matches full type strings. Protocol events (`watch.sync`, `watch.gap`) always pass. Reconnects reuse the same filter and `last_seq`. `last_seq` is bus-global, not per-topic.

Invalid topic characters fail `Watch` before the stream starts (`clusdr: watch topic …`).

Channel buffer is **64**. A slow receiver **blocks** the read loop (it does not drop on the client). The daemon bus still drops slow subscribers.

`Event`: `Type`, `Source`, `Payload` (`[]byte`), `Timestamp`, `Seq`.

## Publish

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = c.Publish(ctx, "deploy.payments.canary", []byte(`{"sha":"7f3a1c2","env":"prod"}`))
	if err != nil {
		log.Fatal(err)
	}
}
```

Payload is raw bytes. Max 64 KiB (daemon). Topic: 1–128 characters, `A–Z a–z 0–9 . _ -`. Over size or a rejected topic returns `clusdr: publish rejected: …` ([Errors](../reference/errors.md#applications)).

Not on the Raft log. A Watch reconnect does not replay this signal — publish again if the fact still matters. This helper does not set `event_id`; it only fails when the daemon returns `accepted=false`.

## Locks

A lock is an exclusive name on the Raft log. Name it after the job (`scheduler.payments.nightly`), not a generic token. Store `lk.Token` with any write that must be fenced. A stale holder cannot unlock a newer grant.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	lk, err := c.Lock(ctx, "scheduler.payments.nightly", 15*time.Second)
	if err != nil {
		log.Fatal(err) // deadline, observer, illegal name, or table full
	}
	fmt.Printf("held %s holder=%s token=%d until %s\n", lk.Name, lk.Holder, lk.Token, lk.Deadline().UTC().Format(time.RFC3339))
	if err := c.Unlock(ctx, "scheduler.payments.nightly"); err != nil {
		log.Fatal(err)
	}
}
```

`Lock` blocks until acquired or `ctx` is done. If `ctx` has no deadline, `WithRequestTimeout` (default 10s) is the wait cap — another holder that keeps the name longer than that returns a deadline error, not a hang. Calling `Lock` again for a name this connection already holds returns the existing `*Lock` and does not re-RPC.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lk, ok, err := c.TryLock(ctx, "scheduler.payments.nightly", 15*time.Second)
	if err != nil {
		log.Fatal(err) // transport / observer / name — not "held by other"
	}
	if !ok {
		who := "another replica"
		if lk != nil && lk.Holder != "" {
			who = lk.Holder
		}
		fmt.Println("held by", who)
		return
	}
	fmt.Println("acquired", lk.Token)
	if err := c.Unlock(ctx, "scheduler.payments.nightly"); err != nil {
		log.Fatal(err)
	}
}
```

`Unlock` of a name this client does not hold → `clusdr: lock "…" is not held by this client` ([Errors](../reference/errors.md#locks-and-leases)).

Background renew starts after acquire, interval about TTL/3 (minimum 50ms). `FailedPrecondition` or `Canceled` on renew stops the loop. `Close` and `Unlock` stop renew and release.

Observer daemon: Lock / TryLock / Unlock / Renew → `FailedPrecondition` (`observer cannot mutate locks`). Take the lock on a voter, or `clusdr promote` that node ([Errors](../reference/errors.md#locks-and-leases)). List is a CLI/RPC concern; this SDK has no `ListLocks`.

Name rules and table size: [locks](../concepts/locks.md), [limits](../reference/limits.md).

## Leases

A lease is a named TTL grant. `Grant` never blocks. Many names can be held at once. If the name is already owned, the call fails immediately (no wait).

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	life, stop := context.WithCancel(ctx)
	lease, err := c.Lease(life, "worker.payments.ingest-1", 15*time.Second)
	if err != nil {
		log.Fatal(err) // already owned, illegal name, or table full
	}
	fmt.Printf("lease %s owner=%s token=%d until %s\n", lease.Name, lease.Owner, lease.Token, lease.Deadline().UTC().Format(time.RFC3339))

	// cancel life → renew stops; grant expires at Deadline. Not a revoke.
	stop()

	if err := c.Renew(ctx, "worker.payments.ingest-1"); err != nil {
		log.Fatal(err)
	}
	if err := c.Revoke(ctx, "worker.payments.ingest-1"); err != nil {
		log.Fatal(err)
	}
}
```

`Lease`’s first argument is both the grant RPC context (until the RPC returns) and the **lifetime** of background renew. Cancelling it after a successful grant stops renew only. Other readers still see the name until TTL.

`Close` **revokes** remaining leases. That is not the same as cancelling the lease context.

`Renew` / `Revoke` of a name this client does not hold → error ([Errors](../reference/errors.md#locks-and-leases)).

Observers **can** grant leases. `presence.<nodeID>` is the daemon’s own liveness lease — not an application grant. Do not reuse that prefix if you want operator liveness separate from app sessions.

`Lease`: `Name`, `Owner`, `Token`, `Deadline()`.

## TLS

On unless `CLUSDR_TLS=disabled` or `WithInsecure()`. A mixed cluster (one side plaintext) fails the handshake ([Errors](../reference/errors.md#tls)).

Lookup order for PEMs is the same in every official SDK: `WithDataDir`, else `CLUSDR_DATA_DIR`, else `~/.clusdr`. Callers do not pass PEM bytes. All SDKs locate TLS material the same way by default — see [Security](../concepts/security.md).

If that directory has no usable certs, connect fails — same as Python, Rust, TypeScript, and Java. Set `CLUSDR_TLS=disabled` or `WithInsecure()` only for plaintext ([Errors](../reference/errors.md#tls)).

When PEMs load: client cert + cluster CA. Peer identity is the CA, not the dial hostname (`InsecureSkipVerify` + `VerifyPeerCertificate` against the CA). Go does not read `CLUSDR_TLS_SERVER_NAME`.

## Errors

Returned errors are wrapped (`clusdr: members: …`, `clusdr: lock "name": …`). Unwrap to the gRPC status when you need the code.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` or `clusdr: dial …` ([Errors](../reference/errors.md#applications)) |
| Empty `Dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` ([Errors](../reference/errors.md#tls)) |
| Transient RPC | retried until deadline |
| No leader | `Unavailable` on `Leader` ([Errors](../reference/errors.md#cluster)) |
| Observer + lock | `FailedPrecondition` ([Errors](../reference/errors.md#locks-and-leases)) |
| Unlock / revoke name you do not hold | SDK error, no RPC |

## Not in this package

- `ListLocks` / `ListLeases`
- Join, leave, promote, config
- A public `WithReadyTimeout`

Wire shapes: [gRPC API](../reference/api/). Python: [Python SDK](python.md). Rust: [Rust SDK](rust.md). TypeScript: [TypeScript SDK](typescript.md). Java: [Java SDK](java.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
