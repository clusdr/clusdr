# Use it from your app

Keep `clusdr start` running from [step 2](first-member.md). This page makes one local SDK call.

Pick a language. Full walkthroughs: [SDKs](../sdk/).

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

```ts
import { local } from "clusdr";

const c = await local();
const members = await c.members();
const lk = await c.lock("scheduler", 15);
```

```java
try (Cluster c = Clusdr.local()) {
    List<Member> members = c.members();
    Lock lk = c.lock("scheduler", Duration.ofSeconds(15));
}
```

| Language | Install |
|---|---|
| Go | `go get github.com/clusdr/clusdr/sdk` |
| Python | `pip install clusdr` |
| Rust | `clusdr = "0.2.0"` |
| TypeScript | `npm install clusdr` |
| Java | `io.clusdr:clusdr` |

`Local` / `local()` / `Clusdr.local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`. TLS is on; certs come from `CLUSDR_DATA_DIR` or `~/.clusdr`.

## Checkpoint

`Members` returns the same nodes you saw in `clusdr members`. You can lock a name.

The tutorial ends here. Real NICs and Docker: [Run on other hosts](other-hosts.md). Examples: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).
