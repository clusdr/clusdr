# SDKs

The application talks to the **local daemon**. The daemon is the cluster member. The SDK does not join Raft and does not dial other nodes.

```text
your process  ──►  clusdr daemon on this host  ──►  the rest of the cluster
```

Same split as a local Docker engine. Two apps on one machine share one daemon.

The [guide](../guide/from-your-app.md) is the first call. These pages are the walkthrough. Go API on [pkg.go.dev](https://pkg.go.dev/github.com/durguto/clusdr/sdk). Runnable copies: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).

| Language | Install | Start here |
|---|---|---|
| Go | `go get github.com/durguto/clusdr/sdk` | [Go SDK](go.md) |
| Python | `pip install clusdr` | [Python SDK](python.md) |
| Rust | `clusdr = "0.1.2"` | [Rust SDK](rust.md) |

CPython 3.10+. Rust 1.82+ (Tokio). Wire package `clusdr.v1alpha1`.

## What the SDKs do

- `Local` / `local()` — apps. Address: `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`
- `Dial` / `dial(addr)` — tests and operators, not the default app path
- Membership, leader, Watch, Publish
- Optional Watch `topics` / `event_types` (same semantics as CLI `--topic` / `--type`)
- Locks and leases with a fencing token and background renew
- Retry `Unavailable` / `Aborted` / `ResourceExhausted` with bounded backoff
- Watch reconnects with `last_seq`
- On connect, wait for the Health RPC (default 10s) or fail

TLS is on unless `CLUSDR_TLS=disabled`. Certs: `ca.crt`, `node.crt`, `node.key` in `CLUSDR_DATA_DIR` or `~/.clusdr`.

## What they do not do

- Join, promote, or configure the cluster (CLI)
- Store application data
- Talk to a remote node's Runtime API as the normal path — put a daemon on that host

## Environment

| Variable | Who reads it | Meaning |
|---|---|---|
| `CLUSDR_GRPC_ADDR` | Both | Runtime address. Default `127.0.0.1:7947` |
| `CLUSDR_TLS` | Both | `disabled` / `off` / `false` / `0` → plaintext |
| `CLUSDR_DATA_DIR` | Both | Directory with PEMs. Else `~/.clusdr` |
| `CLUSDR_TLS_SERVER_NAME` | Python, Rust | TLS server name override (peer node id) |

## Holder

Each connection has a lock/lease identity. Empty → generated `sdk-<hex>`. Two apps on the same host cannot unlock each other unless they share `WithHolder` / `holder=`.

One Go or Rust `Cluster` is safe from many tasks (same holder: `Unlock` / `unlock` is process-wide for that name). Several Watch streams on one client are fine. Python: unary calls are fine from several threads; run one `watch()` loop per `Cluster` — `close()` cancels the last in-flight Watch RPC.
