# Java SDK

The Java SDK is a blocking gRPC client that talks to the **local** daemon. The application is not a cluster member: it does not vote or speak Raft.

A daemon must already be running ([guide: first member](../guide/first-member.md)). Shared model: [SDKs](./).

| | Value | Why it matters |
|---|---|---|
| Artifact | `io.clusdr:clusdr` | Pin the same version train as the daemon or the stubs and the running process disagree. |
| Language | Java 17+ | Older JDKs will not compile or run this client. |
| I/O | Blocking gRPC | Calls occupy a thread until they return — do not run `watch()` on a request thread you need back immediately. |

```xml
<dependency>
  <groupId>io.clusdr</groupId>
  <artifactId>clusdr</artifactId>
  <version>0.2.0</version>
</dependency>
```

Contributor checkout: `mvn test` in the `clusdr-java` tree. `make proto` exports [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (or sibling `../clusdr/proto/api`), then injects `java_package`.

## Connect

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;

public final class Connect {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      System.out.println(c.members());
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e.getMessage());
      System.exit(1);
    }
  }
}
```

If this fails, start the **local** daemon and match TLS to `clusdr start` ([Errors](../reference/errors.md#applications), [TLS](../reference/errors.md#tls)). All SDKs locate PEMs the same way ([Security](../concepts/security.md)).

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`readyTimeout`, default 10s). On Kubernetes, `127.0.0.1` is the pod — set `CLUSDR_GRPC_ADDR` to the node Runtime, unless the app is a [sidecar](../guide/kubernetes-sidecar.md).

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Options;

public final class DialSecondDaemon {
  public static void main(String[] args) {
    try {
      Cluster c = Clusdr.dial("127.0.0.1:8947", Options.defaults().dataDir("./data-b"));
      c.close();
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e.getMessage());
      System.exit(1);
    }
  }
}
```

`dial` is `local()` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local()` there. Empty `dial("")` throws `clusdr: empty dial address` ([Errors](../reference/errors.md#applications)).

Unary methods are fine concurrently. Same connection = same holder (`unlock` is process-wide for that name). Several `watch()` loops on one client are fine.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Options;
import java.time.Duration;

public final class ConnectOptions {
  public static void main(String[] args) {
    try (Cluster c =
        Clusdr.local(
            Options.defaults()
                .insecure(false)
                .dataDir("")
                .holder("")
                .requestTimeout(Duration.ofSeconds(10))
                .readyTimeout(Duration.ofSeconds(10))
                .serverName(""))) {
      System.out.println(c.members());
    } catch (ClusdrException e) {
      System.err.println("connect failed: " + e.getMessage());
      System.exit(1);
    }
  }
}
```

Same fields on `dial(addr, opts)`.

| Option | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `dataDir` is empty. Required when `start` disabled TLS, or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `dataDir` | Optional override for `ca.crt` / `node.crt` / `node.key`. Default: `CLUSDR_DATA_DIR` or `~/.clusdr`. Missing files throw. |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>`. Two processes cannot unlock each other unless they share this id ([Errors](../reference/errors.md#locks-and-leases)). |
| `requestTimeout` | Unary timeout (default 10s). `lock` waits at most this long unless you pass a per-call `Duration`. |
| `readyTimeout` | Health wait on connect (default 10s). Zero skips the wait — the first RPC then fails if the daemon is down. |
| `serverName` | TLS server name (peer **node id**). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt`. If none resolve, connect fails ([Errors](../reference/errors.md#tls)). |

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

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline. Other codes throw immediately.

`close()` (and try-with-resources) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures throw `ClusdrException` (or `IllegalArgumentException` for a bad publish payload).

## Membership

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Member;

public final class Membership {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      for (Member m : c.members()) {
        System.out.printf(
            "%s %s status=%s role=%s leader=%s%n",
            m.id(), m.address(), m.status(), m.role(), m.leader());
      }
      Member leader = c.leader();
      System.out.println("leader " + leader.id() + " at " + leader.address());
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

`Member`: `id`, `address`, `status` (`alive` or `dead`), `leader`, `role` (empty wire role becomes `"voter"`). A left id is gone from `members()`.

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=true`). No leader → `ClusdrException` ([Errors](../reference/errors.md#cluster)).

## Watch

You do not pass `lastSeq`. The stream stores `event.seq()` and sends it as `lastSeq` on reconnect so cluster events resume after a drop. `custom.*` is still not replayed.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Event;
import java.nio.charset.StandardCharsets;

public final class WatchBus {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      for (Event event : c.watch()) {
        if (event.type().equals("member.dead")) {
          System.out.println("crash seq=" + event.seq() + " still listed src=" + event.source());
        } else if (event.type().equals("member.left")) {
          System.out.println("leave seq=" + event.seq() + " gone from members src=" + event.source());
        } else if (event.type().equals("custom.deploy.payments.canary")) {
          System.out.println(
              "gossip seq="
                  + event.seq()
                  + " payload="
                  + new String(event.payload(), StandardCharsets.UTF_8));
        } else {
          System.out.println("bus seq=" + event.seq() + " " + event.type() + " src=" + event.source());
        }
      }
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

The stream reconnects with `lastSeq` on drop (backoff 50ms → 2s). Closing the `Watch` or `close()` on the cluster ends it.

`topics` / `eventTypes` match the CLI. Empty (default) is the full bus.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Event;
import io.clusdr.WatchFilter;
import java.nio.charset.StandardCharsets;

public final class WatchTopic {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      for (Event event : c.watch(WatchFilter.all().topics("deploy.payments.canary"))) {
        System.out.println(
            event.type()
                + " "
                + event.seq()
                + " "
                + new String(event.payload(), StandardCharsets.UTF_8));
      }
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

Non-empty `topics`: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → `ClusdrException` before the first event.

`Event`: `type`, `source`, `payload` (`byte[]`), `timestamp` (`Instant`), `seq`.

## Publish

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import java.nio.charset.StandardCharsets;
import java.util.Map;

public final class PublishCanary {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      c.publish("deploy.payments.canary", Map.of("sha", "7f3a1c2", "env", "prod"));
      c.publish("deploy.payments.canary", "{\"sha\":\"7f3a1c2\",\"env\":\"prod\"}");
      c.publish(
          "deploy.payments.canary",
          "{\"sha\":\"7f3a1c2\",\"env\":\"prod\"}".getBytes(StandardCharsets.UTF_8));
      c.publish("deploy.payments.canary");
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

| Java type | On the wire |
|---|---|
| `null` | empty bytes |
| `byte[]` | as-is |
| `String` | UTF-8 |
| `Map` with string keys | compact JSON UTF-8 (string / number / boolean / null values) |
| anything else | `IllegalArgumentException` |

SDK-side cap **64 KiB** (`ClusdrException` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`). Over size or `accepted=false` → `ClusdrException` ([Errors](../reference/errors.md#applications)).

Not on the Raft log. A Watch reconnect does not replay this signal.

## Locks

Exclusive name on the Raft log. Name it after the job (`scheduler.payments.nightly`). Store `lk.token()` with fenced writes.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Lock;
import java.time.Duration;

public final class NightlyScheduler {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      Lock lk = c.lock("scheduler.payments.nightly", Duration.ofSeconds(15));
      try {
        System.out.println(lk.name() + " " + lk.holder() + " " + lk.token() + " " + lk.deadline());
      } finally {
        c.unlock("scheduler.payments.nightly");
      }
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

`lock` blocks until acquired or `timeout` (default `requestTimeout`, 10s). Another holder that keeps the name longer than that throws; it does not hang. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Lock;
import java.time.Duration;
import java.util.Optional;

public final class TryNightly {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      Optional<Lock> lk = c.tryLock("scheduler.payments.nightly", Duration.ofSeconds(15));
      if (lk.isEmpty()) {
        System.out.println("held by another replica");
        return;
      }
      System.out.println("acquired " + lk.get().token());
      c.unlock("scheduler.payments.nightly");
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

That matches Python (`None`) and Rust (`Ok(None)`), not Go’s `(lk, false, nil)`.

`unlock` of a name this client does not hold → `ClusdrException` ([Errors](../reference/errors.md#locks-and-leases)).

Background renew: timer, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrException`). Take the lock on a voter, or `clusdr promote` that node.

No `listLocks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```java
import io.clusdr.Clusdr;
import io.clusdr.ClusdrException;
import io.clusdr.Cluster;
import io.clusdr.Lease;
import java.time.Duration;

public final class IngestLease {
  public static void main(String[] args) {
    try (Cluster c = Clusdr.local()) {
      Lease ls = c.lease("worker.payments.ingest-1", Duration.ofSeconds(15));
      System.out.println(ls.name() + " " + ls.owner() + " " + ls.token() + " " + ls.deadline());
      ls.stopRenew();
      c.renew("worker.payments.ingest-1");
      c.revoke("worker.payments.ingest-1");
    } catch (ClusdrException e) {
      System.err.println(e.getMessage());
      System.exit(1);
    }
  }
}
```

`stopRenew` stops background renew; the grant then expires at `ls.deadline()`. That is not a revoke. `close()` **revokes**.

Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline`.

`renew` / `revoke` of a name this client does not hold → `ClusdrException` ([Errors](../reference/errors.md#locks-and-leases)).

## TLS

On unless `insecure(true)` or `CLUSDR_TLS=disabled` (and no `dataDir`).

Lookup is the same as every official SDK: `dataDir`, else `CLUSDR_DATA_DIR`, else `~/.clusdr`. Callers do not pass PEM bytes. Missing files throw `ClusdrException` ([Errors](../reference/errors.md#tls)).

Server name: `serverName`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. Needed only when the CN is missing — gRPC requires a name; Go verifies the CA without SNI. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrException` is the SDK failure type (unchecked). Transient gRPC codes are retried; others throw immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` ([Errors](../reference/errors.md#applications)) |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` ([Errors](../reference/errors.md#tls)) |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` ([Errors](../reference/errors.md#tls)) |
| Bad publish type | `IllegalArgumentException` |
| Publish too large | `ClusdrException` (64 KiB) ([Errors](../reference/errors.md#applications)) |
| `tryLock` held by other | `Optional.empty()` |
| Unlock / revoke name you do not hold | `ClusdrException`, no success path ([Errors](../reference/errors.md#locks-and-leases)) |

## Not in this package

- Join, promote, config
- An async / reactive client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Python: [Python SDK](python.md). Rust: [Rust SDK](rust.md). TypeScript: [TypeScript SDK](typescript.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
