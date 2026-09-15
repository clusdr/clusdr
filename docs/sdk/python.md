# Python SDK

Package `clusdr` on PyPI. CPython 3.10+. Applications call the daemon on this host. Shared model: [SDKs](./).

```bash
pip install clusdr
```

Same version train as the daemon (first release: `0.1.0`).

A running daemon is required ([guide: first member](../guide/first-member.md)).

Contributor checkout (editable + proto): `pip install -e ".[dev]"` in the `clusdr-python` tree. That is not the product install.

## Connect

```python
from clusdr import local

c = local()
# ...
c.close()
```

Or a context manager:

```python
from clusdr import local

with local() as c:
    print(c.members())
```

`local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (`ready_timeout`, default 10s).

```python
from clusdr import dial

c = dial("127.0.0.1:8947", data_dir="./data-b")
```

`dial` is for tests and operators. Apps use `local()`.

```python
local(
    insecure=False,
    data_dir="",
    holder="",
    request_timeout=10.0,
    ready_timeout=10.0,
    server_name="",
)
```

Same keyword arguments on `dial(addr, ...)`.

| Argument | Meaning |
|---|---|
| `insecure` | Plaintext. Also set if `CLUSDR_TLS=disabled` and `data_dir` is empty |
| `data_dir` | Directory with `ca.crt` / `node.crt` / `node.key` |
| `holder` | Lock/lease identity. Empty → `sdk-<uuid>` |
| `request_timeout` | Unary timeout in seconds (default 10) |
| `ready_timeout` | Health wait on connect (default 10). `0` skips the wait |
| `server_name` | TLS server name (peer node id). Else `CLUSDR_TLS_SERVER_NAME`, else CN of `node.crt` |

Every unary method also takes `timeout: float | None` to override `request_timeout` for that call.

## `Cluster`

```text
members(timeout=None) -> list[Member]
leader(timeout=None) -> Member
watch() -> Iterator[Event]
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

Unary calls retry `UNAVAILABLE`, `ABORTED`, and `RESOURCE_EXHAUSTED` until the monotonic deadline.

`close()` (and leaving `with`) stops Watch, unlocks locks, revokes leases, closes the channel.

Failures raise `ClusdrError` (or `TypeError` for a bad publish payload).

## Membership

```python
members = c.members()
leader = c.leader()
```

`Member` (frozen dataclass): `id`, `address`, `status`, `leader`, `role` (default `"voter"`). Empty wire role becomes `"voter"`.

`leader()` builds a member from `GetLeader` (`status="alive"`, `role="voter"`, `leader=True`). No leader → `ClusdrError` wrapping `Unavailable`.

## Watch

```python
for event in c.watch():
    if event.type == "member.left":
        ...
    if event.type == "custom.deployment":
        event.payload  # bytes
```

`watch()` is a blocking iterator. It reconnects with `last_seq` on drop (backoff 0.05s → 2s). `close()` cancels the in-flight RPC and ends the loop.

There is **no** topic or type filter. You see the full bus. CLI `--topic` is a different client.

Custom events are not replayed after `watch.gap`.

`Event` (frozen): `type`, `source`, `payload` (`bytes`), `timestamp` (UTC `datetime`), `seq`.

## Publish

```python
c.publish("deployment", {"sha": "abc"})
c.publish("deployment", '{"sha":"abc"}')
c.publish("deployment", b'{"sha":"abc"}')
c.publish("ping")  # empty payload
```

| Python type | On the wire |
|---|---|
| `None` | empty bytes |
| `bytes` | as-is |
| `str` | UTF-8 |
| `dict` / `list` | compact JSON UTF-8 |
| anything else | `TypeError` |

SDK-side cap **64 KiB** (`ClusdrError` before the RPC). Topic rules are the daemon’s (1–128, `A–Z a–z 0–9 . _ -`).

Not on the Raft log. `accepted=false` → `ClusdrError`.

## Locks

Exclusive name on the Raft log. Store `lk.token` with fenced writes.

```python
lk = c.lock("scheduler", ttl=15)
try:
    _ = lk.name, lk.holder, lk.token, lk.deadline
finally:
    c.unlock("scheduler")
```

`lock` blocks until acquired or `timeout`. Calling it again for a name this connection already holds returns the existing `Lock` and does not re-RPC.

```python
lk = c.try_lock("scheduler", ttl=15)
if lk is None:
    # someone else holds it — not an error
```

That differs from Go: Go returns `(lk, False, nil)` and `lk` may describe the current holder. Python returns `None`.

`unlock` of a name this client does not hold → `ClusdrError`.

Background renew: daemon thread, interval about TTL/3 (minimum 50ms). Observer daemon rejects lock mutations (`FAILED_PRECONDITION` → `ClusdrError`).

No `list_locks` in this package.

## Leases

Named TTL grant. `lease` never waits on another owner; it fails if the name is taken.

```python
import threading

stop = threading.Event()
ls = c.lease("worker-1", ttl=15, stop=stop)

# stop renew; grant expires at ls.deadline. Not a revoke.
stop.set()
# or
ls.stop_renew()

c.renew("worker-1")
c.revoke("worker-1")
```

`stop` is the lifetime of background renew (Go’s lease context). `close()` **revokes**.

`renew` / `revoke` of a name this client does not hold → `ClusdrError`.

Observers can grant leases. `presence.<nodeID>` is the daemon’s lease, not yours.

`Lease`: `name`, `owner`, `token`, `deadline` (property).

## TLS

On unless `insecure=True` or `CLUSDR_TLS=disabled` (and no `data_dir`).

PEMs from `data_dir` or `CLUSDR_DATA_DIR` or `~/.clusdr`. If the three files are **missing**, Python raises `ClusdrError` and tells you to disable TLS or pass `insecure=True`. It does **not** fall back to skip-verify bootstrap TLS. That is stricter than the Go SDK.

Server name: `server_name`, else `CLUSDR_TLS_SERVER_NAME`, else the CN of `node.crt`. If none of those resolve, connect fails. gRPC uses `grpc.ssl_target_name_override` with that name. Peer identity is still the cluster CA, not the dial hostname.

## Errors

`ClusdrError` is the SDK failure type. Transient gRPC codes are retried; others raise immediately.

| Situation | What you see |
|---|---|
| Daemon down / Health timeout | `clusdr: daemon not ready at …` |
| Empty `dial("")` | `clusdr: empty dial address` |
| TLS files missing | `clusdr: TLS enabled but ca.crt/… missing in …` |
| TLS name unknown | `clusdr: TLS hostname unknown; set CLUSDR_TLS_SERVER_NAME …` |
| Bad publish type | `TypeError` |
| Publish too large | `ClusdrError` (64 KiB) |
| `try_lock` held by other | `None` |
| Unlock / revoke name you do not hold | `ClusdrError`, no success path |

## Not in this package

- Watch topic / type filters
- `list_locks` / `list_leases`
- Join, promote, config
- Async / `asyncio` client

Wire shapes: [gRPC API](../reference/api/). Go surface: [Go SDK](go.md).
