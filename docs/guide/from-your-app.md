# 5. Use it from your app

The daemon you started in [step 2](first-member.md) is the cluster member. The application is a client of the daemon **on the same host**.

```text
app A ─┐
app B ─┼─► clusdr daemon on this host ─► other clusdr daemons
cli   ─┘
```

Two processes on one machine share one daemon. The app does not vote. It does not speak Raft.

Keep `clusdr start` running. Then pick a language:

| Language | Install | Full docs |
|---|---|---|
| Go | `go get github.com/durguto/clusdr/sdk` | [Go SDK](../sdk/go.md) |
| Python | `pip install clusdr` | [Python SDK](../sdk/python.md) |
| Rust | `clusdr = "0.1.2"` | [Rust SDK](../sdk/rust.md) |

```go
c, err := clusdr.Local()
members, err := c.Members(ctx)
lk, err := c.Lock(ctx, "scheduler", 15*time.Second)
```

```python
from clusdr import local

c = local()
members = c.members()
lk = c.lock("scheduler", ttl=15)
```

```rust
let c = clusdr::local(clusdr::Options::new()).await?;
let members = c.members().await?;
let lk = c.lock("scheduler", Some(std::time::Duration::from_secs(15))).await?;
```

`Local` / `local()` dial `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`. TLS is on; certs come from `CLUSDR_DATA_DIR` or `~/.clusdr`.

What the app must not do: join the cluster, dial a remote Runtime API as the normal path, or treat `publish` as durable storage.

Model, env, holder, every SDK: **[SDKs](../sdk/)**. Copy-paste programs (Go, Python, and Rust): [examples/](https://github.com/clusdr/clusdr/tree/main/examples).

## Next

Laptop defaults bind localhost. [Run on other hosts →](other-hosts.md)
