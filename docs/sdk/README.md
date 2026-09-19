# SDKs

Every language SDK talks to the **local daemon**. The application is a client; the daemon is the cluster **member**. The SDK does not join Raft and does not dial other nodes. If you `Dial` a remote Runtime API — or a Kubernetes ClusterIP of clusdr — you are not using `Local()`. That app then talks to some other host’s daemon, so membership and locks on **this** host are invisible and `Local()` timeouts look like a down daemon ([Errors](../reference/errors.md#applications)). Put a daemon on that host and call `Local` there.

The first call against a running daemon is in the [guide](../guide/from-your-app.md). Go’s exported API lives on [pkg.go.dev](https://pkg.go.dev/github.com/clusdr/clusdr/sdk). Runnable copies: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).

```text
your process  ──►  clusdr daemon on this host  ──►  the rest of the cluster
```

Two apps on one machine share one daemon, the same way two processes share a local Docker engine.

| Language | Install | Start here |
|---|---|---|
| Go | `go get github.com/clusdr/clusdr/sdk` | [Go SDK](go.md) |
| Python | `pip install clusdr` | [Python SDK](python.md) |
| Rust | `clusdr = "0.2.0"` | [Rust SDK](rust.md) |
| TypeScript | `npm install clusdr` | [TypeScript SDK](typescript.md) |
| Java | `io.clusdr:clusdr` | [Java SDK](java.md) |

| Runtime | Minimum | Why it matters |
|---|---|---|
| CPython | 3.10+ | Older CPython and PyPy are untested; `watch()` is a blocking iterator. |
| Rust | 1.82+ (Tokio) | No blocking client — `local()` needs a Tokio runtime. |
| Node.js | 20+ | No blocking (non-async) client. |
| Java | 17+ | Blocking gRPC: `watch()` occupies a thread until you cancel it. |
| Wire | `clusdr.v1alpha1` | Pin daemon and SDK to the same version train or the stubs and the running process disagree. |

## What the SDKs do

- `Local` / `local()` — the application path. Address: `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`. On Kubernetes that is the **node** daemon ([Kubernetes](../concepts/kubernetes.md)), not a Service: `127.0.0.1` inside a pod is that pod, so an unset address fails Health ([Errors](../reference/errors.md#applications)). Sidecar exception: `127.0.0.1` in that pod ([sidecar](../guide/kubernetes-sidecar.md)).
- `Dial` / `dial(addr)` — tests and a second daemon on this host (another `grpc.addr`). Not the default app path; a remote Runtime is the same mistake as a ClusterIP Dial.
- Membership, leader, Watch, Publish — the same facts `clusdr members` and `clusdr publish` use.
- Optional Watch `topics` / `event_types` (same semantics as CLI `--topic` / `--type`). A non-empty topic list omits the membership snapshot, so a filtered listener will not see `member.join` until you drop the filter.
- Status is a string: `alive` or `dead`. Watch crash is `member.dead` (id still listed); leave is `member.left` (id gone). No `Leave()` on Cluster — that is CLI. Treating crash as leave and calling `join` again creates a second identity ([Errors](../reference/errors.md#cluster)).
- Locks and leases with a fencing token and background renew. Store the token with any fenced write; a stale unlock cannot steal a newer grant ([Errors](../reference/errors.md#locks-and-leases)).
- Retry `Unavailable` / `Aborted` / `ResourceExhausted` with bounded backoff (50ms → 2s) until the call deadline. Other codes fail immediately so a bad name or an observer lock is not hidden by retry.
- Watch reconnects with `last_seq`. Cluster events resume; `custom.*` is gossip and is not replayed — publish again if that signal still matters ([Errors](../reference/errors.md#applications)).
- On connect, wait for the Health RPC (default 10s) or fail. A silent skip would let the first RPC hang on a down daemon.

TLS is on unless `CLUSDR_TLS=disabled`. Certs: `ca.crt`, `node.crt`, `node.key` in `CLUSDR_DATA_DIR` or `~/.clusdr`. If `start` used plaintext and the SDK did not (or the reverse), the handshake fails ([Errors](../reference/errors.md#tls)).

## What they do not do

- Join, leave, promote, or configure the cluster (CLI). Doing that from the app would make a crash look like `leave` and force another `join`.
- Store application data. Publish is one-hop gossip, not the Raft log.
- Talk to a remote node's Runtime API as the normal path — put a daemon on that host.

## Environment

| Variable | Who reads it | Meaning |
|---|---|---|
| `CLUSDR_GRPC_ADDR` | Both | Runtime address. Default `127.0.0.1:7947`. Wrong port → `daemon not ready` even when `clusdr status` sees a socket ([Errors](../reference/errors.md#applications)). |
| `CLUSDR_TLS` | Both | `disabled` / `off` / `false` / `0` → plaintext. Must match every node and every client or the handshake fails ([Errors](../reference/errors.md#tls)). |
| `CLUSDR_DATA_DIR` | Both | Directory with PEMs. Else `~/.clusdr`. Point it at **this host’s** `data.dir` so the app presents the same CA as the daemon. |
| `CLUSDR_TLS_SERVER_NAME` | Python, Rust, TypeScript, Java | TLS server name override (peer **node id**). Go verifies the cluster CA and does not need this. The others fail connect if no `server_name` / `serverName`, no env, and no CN on `node.crt` ([Errors](../reference/errors.md#tls)). |

## Holder

Each connection has a lock/lease identity. Empty generates a unique `sdk-` id for that connection (Go: 16 random bytes as hex; Rust: UUID without hyphens; Python, TypeScript, Java: UUID). Two apps on the same host cannot unlock each other unless they share `WithHolder` / `holder=` / `holder`. Without a shared holder, a second replica’s `Unlock` returns “not held by this client” even though both sit on one daemon ([Errors](../reference/errors.md#locks-and-leases)).

One Go, Rust, TypeScript, or Java `Cluster` is safe from many tasks (same holder: `Unlock` / `unlock` is process-wide for that name). Several Watch streams on one client are fine. Python: unary calls are fine from several threads; run one `watch()` loop per `Cluster` — `close()` cancels the last in-flight Watch RPC, so a second loop overwrites that cancel handle and the first stream is left running until process exit.
