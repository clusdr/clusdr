# Python SDK

The Python SDK talks to the **local** daemon from a CPython process. The application is not a cluster member: it does not vote or speak Raft.

A daemon must already be running ([guide: first member](../guide/first-member.md)). Shared model: [SDKs](./).

| | Value | Why it matters |
|---|---|---|
| Package | `clusdr` on PyPI | `pip install clusdr`. |
| Runtime | CPython 3.10+ | PyPy and older CPython are untested; `watch()` is a blocking iterator on this interpreter. |
| Version | same train as the daemon | A mismatched install talks `clusdr.v1alpha1` stubs the running process does not serve. |

```bash
pip install clusdr
```

Contributor checkout (editable + proto): `pip install -e ".[dev]"` in the `clusdr-python` tree. `make proto` exports [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (or sibling `../clusdr/proto/api`) and regenerates stubs. That is not the product install.

## Connect

```python
from clusdr import ClusdrError, local

try:
    c = local()
except ClusdrError as exc:
    raise SystemExit(f"connect failed: {exc}") from exc
print(c.members())
c.close()
```

Or a context manager (`close()` runs on the way out):

```python
from clusdr import ClusdrError, local

try:
    with local() as c:
        print(c.members())
except ClusdrError as exc:
    raise SystemExit(f"connect failed: {exc}") from exc
```

If this fails, start the **local** daemon and present PEMs from that host’s `data.dir` ([Errors](../reference/errors.md#applications), [TLS](../reference/errors.md#tls)). Python does not skip-verify the way Go bootstrap TLS does.

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`ready_timeout`, default 10s). On Kubernetes, `127.0.0.1` is the pod — set `CLUSDR_GRPC_ADDR` to the node Runtime, unless the app is a [sidecar](../guide/kubernetes-sidecar.md).

```python
from clusdr import ClusdrError, dial

try:
    c = dial("127.0.0.1:8947", data_dir="./data-b")
except ClusdrError as exc:
    raise SystemExit(f"connect failed: {exc}") from exc
c.close()
```

`dial` is `local()` with an explicit address (tests, a second daemon on this host). Do not `dial` a **remote** node's Runtime API as the normal app path — put a daemon on that host and call `local()` there. Empty `dial("")` raises `clusdr: empty dial address` ([Errors](../reference/errors.md#applications)).

Unary methods are fine from several threads. Same connection = same holder (`unlock` is process-wide for that name). Run one `watch()` loop per client — a second loop overwrites the cancel handle, so `close()` would cancel only the newer RPC.

```python
from clusdr import local

c = local(
    insecure=False,
    data_dir="",
    holder="",
    request_timeout=10.0,
    ready_timeout=10.0,
    server_name="",
)
c.close()
```

Same keyword arguments on `dial`.

| Argument | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `data_dir` is empty. Required when `start` disabled TLS, or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `data_dir` | Directory with `ca.crt` / `node.crt` / `node.key`. Missing files raise; there is no skip-verify fallback. |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>`. Two processes cannot unlock each other unless they share this id ([Errors](../reference/errors.md#locks-and-leases)). |
| `request_timeout` | Unary timeout in seconds (default 10). `lock` waits at most this long unless you pass `timeout`. |
| `ready_timeout` | Health wait on connect (default 10). `0` skips the wait — the first RPC then fails if the daemon is down. |
| `server_name` | TLS server name (peer **node id**). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt`. If none resolve, connect fails ([Errors](../reference/errors.md#tls)). |

Every unary method also takes `timeout: float | None` to override `request_timeout` for that call.

## `Cluster`

```text
members(timeout=None) -> list[Member]
leader(timeout=None) -> Member
watch(topics=None, event_types=None) -> Iterator[Event]
publish(topic, payload=None, timeout=None) -> None
lock(name, ttl=None, timeout=None) -> Lock
try_lock(name, ttl=None, timeout=None) -> Lock | None
unlock(name, timeout=None) -> None
lease(name, ttl=None, *, stop=None, timeout=None) -> Lease
renew(name, timeout=None) -> None
revoke(name, timeout=None) -> None
close() -> None
```

`ttl is None` or `<= 0` sends `ttl_ms = 0`; the daemon uses its default (15s). `ttl` is seconds (Go uses `time.Duration`).

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline. Other codes raise immediately.

`close()` (and leaving `with`) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures raise `ClusdrError` (or `TypeError` for a bad publish payload).

## Membership

```python
from clusdr import ClusdrError, local

try:
    with local() as c:
        for m in c.members():
            print(f"{m.id} {m.address} status={m.status} role={m.role} leader={m.leader}")
        leader = c.leader()
        print(f"leader {leader.id} at {leader.address}")
except ClusdrError as exc:
    raise SystemExit(exc) from exc
```

`Member` (frozen dataclass): `id`, `address`, `status` (`alive` or `dead`), `leader`, `role` (default `"voter"`). Empty wire role becomes `"voter"`. A left id is gone from `members()`.

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=True`). No leader → `ClusdrError` wrapping `Unavailable` ([Errors](../reference/errors.md#cluster)).

## Watch

You do not pass `last_seq`. The iterator stores `event.seq` and sends it as `last_seq` on reconnect so cluster events resume after a drop. `custom.*` is still not replayed.

```python
from clusdr import ClusdrError, local

try:
    c = local()
except ClusdrError as exc:
    raise SystemExit(f"connect failed: {exc}") from exc

try:
    for event in c.watch():
        if event.type == "member.dead":
            print(f"crash seq={event.seq} still listed src={event.source}")
        elif event.type == "member.left":
            print(f"leave seq={event.seq} gone from members src={event.source}")
        elif event.type == "custom.deploy.payments.canary":
            print(f"gossip seq={event.seq} payload={event.payload!r}")
        else:
            print(f"bus seq={event.seq} {event.type} src={event.source}")
except KeyboardInterrupt:
    pass
finally:
    c.close()
```

`watch()` is a blocking iterator. It reconnects with `last_seq` on drop (backoff 0.05s → 2s). `close()` cancels the in-flight RPC and ends the loop. Run **one** `watch()` per `Cluster`. Unary methods (`members`, `lock`, `publish`) are fine from other threads.

`topics` / `event_types` match the CLI. Empty (default) is the full bus.

```python
from clusdr import ClusdrError, local

try:
    c = local()
except ClusdrError as exc:
    raise SystemExit(f"connect failed: {exc}") from exc

try:
    for event in c.watch(topics=["deploy.payments.canary"]):
        print(event.type, event.seq, event.payload)
except ClusdrError as exc:
    raise SystemExit(exc) from exc
except KeyboardInterrupt:
    pass
finally:
    c.close()
```

Non-empty `topics`: only `custom.<topic>`; **membership snapshot is omitted**. Custom events are not replayed. Reconnects reuse the filter. Invalid topic → `ClusdrError` before the first event.

`Event` (frozen): `type`, `source`, `payload` (`bytes`), `timestamp` (UTC `datetime`), `seq`.

## Publish

```python
from clusdr import ClusdrError, local

try:
    with local() as c:
        c.publish("deploy.payments.canary", {"sha": "7f3a1c2", "env": "prod"})
        c.publish("deploy.payments.canary", '{"sha":"7f3a1c2","env":"prod"}')
        c.publish("deploy.payments.canary", b'{"sha":"7f3a1c2","env":"prod"}')
        c.publish("deploy.payments.canary")
except ClusdrError as exc:
    raise SystemExit(exc) from exc
```

| Python type | On the wire |
|---|---|
| `None` | empty bytes |
| `bytes` | as-is |
| `str` | UTF-8 |
| `dict` / `list` | compact JSON UTF-8 |
| anything else | `TypeError` |

SDK-side cap **64 KiB** (`ClusdrError` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`). Over size or `accepted=false` → `ClusdrError` ([Errors](../reference/errors.md#applications)).

Not on the Raft log. A Watch reconnect does not replay this signal.

## Locks

Exclusive name on the Raft log. Name it after the job (`scheduler.payments.nightly`). Store `lk.token` with fenced writes.

```python
from clusdr import ClusdrError, local

try:
    with local() as c:
        lk = c.lock("scheduler.payments.nightly", ttl=15)
        try:
            print(lk.name, lk.holder, lk.token, lk.deadline)
        finally:
            c.unlock("scheduler.payments.nightly")
except ClusdrError as exc:
    raise SystemExit(exc) from exc
```

`lock` blocks until acquired or `timeout` (default `request_timeout`, 10s). Another holder that keeps the name longer than that raises; it does not hang. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```python
from clusdr import ClusdrError, local

try:
    with local() as c:
        lk = c.try_lock("scheduler.payments.nightly", ttl=15)
        if lk is None:
            print("held by another replica")
        else:
            print("acquired", lk.token)
            c.unlock("scheduler.payments.nightly")
except ClusdrError as exc:
    raise SystemExit(exc) from exc
```

That differs from Go: Go returns `(lk, False, nil)` and `lk` may describe the current holder. Python returns `None`.

`unlock` of a name this client does not hold → `ClusdrError` ([Errors](../reference/errors.md#locks-and-leases)).

Background renew: daemon thread, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrError`). Take the lock on a voter, or `clusdr promote` that node.

No `list_locks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```python
import threading

from clusdr import ClusdrError, local

stop = threading.Event()
try:
    with local() as c:
        ls = c.lease("worker.payments.ingest-1", ttl=15, stop=stop)
        print(ls.name, ls.owner, ls.token, ls.deadline)
        stop.set()
        ls.stop_renew()
        c.renew("worker.payments.ingest-1")
        c.revoke("worker.payments.ingest-1")
except ClusdrError as exc:
    raise SystemExit(exc) from exc
```

`stop` is the lifetime of background renew (Go’s lease context). Cancelling it (or `ls.stop_renew()`) stops renew only; the grant expires at `ls.deadline`. `close()` **revokes**.

`renew` / `revoke` of a name this client does not hold → `ClusdrError` ([Errors](../reference/errors.md#locks-and-leases)).

Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline` (property).

## TLS

On unless `insecure=True` or `CLUSDR_TLS=disabled` (and no `data_dir`).

PEMs from `data_dir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, Python raises `ClusdrError` and tells you to disable TLS or pass `insecure=True`. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK ([Errors](../reference/errors.md#tls)).

Server name: `server_name`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. gRPC uses `grpc.ssl_target_name_override` with that name. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrError` is the SDK failure type. Transient gRPC codes are retried; others raise immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` ([Errors](../reference/errors.md#applications)) |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` ([Errors](../reference/errors.md#tls)) |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` ([Errors](../reference/errors.md#tls)) |
| Bad publish type | `TypeError` |
| Publish too large | `ClusdrError` (64 KiB) ([Errors](../reference/errors.md#applications)) |
| `try_lock` held by other | `None` |
| Unlock / revoke name you do not hold | `ClusdrError`, no success path ([Errors](../reference/errors.md#locks-and-leases)) |

## Not in this package

- Join, promote, config
- Async / `asyncio` client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md). Rust: [Rust SDK](rust.md). TypeScript: [TypeScript SDK](typescript.md). Java: [Java SDK](java.md). Runnable programs: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
