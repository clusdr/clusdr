# Use it from your app

The application is a client of the daemon **on this host**. It does not vote and does not speak Raft. Two processes on one machine share one daemon. The snippets below make one local SDK call against the daemon you started in [step 2](first-member.md).

Keep that `clusdr start` running. If `clusdr members` fails in another terminal, `Local()` will fail the same way — start the daemon first.

Install the SDK for one language, then run the snippet. Full walkthroughs: [SDKs](../sdk/). Runnable copies: [examples/](https://github.com/clusdr/clusdr/tree/main/examples).

| Language | Install |
|---|---|
| Go | `go get github.com/clusdr/clusdr/sdk` |
| Python | `pip install clusdr` |
| Rust | `clusdr = "0.2.0"` in `Cargo.toml` |
| TypeScript | `npm install clusdr` |
| Java | `io.clusdr:clusdr` on the same version train |

TLS is on by default. Every official SDK locates PEMs the same way (`CLUSDR_DATA_DIR` or `~/.clusdr`) — see [Security](../concepts/security.md). If `start` used `CLUSDR_TLS=disabled`, the SDK must too, or the handshake fails ([Errors](../reference/errors.md#tls)).

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
	ctx := context.Background()
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err) // daemon down, TLS, or Health not ready within 10s
	}
	defer c.Close()

	members, err := c.Members(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(members)

	lk, err := c.Lock(ctx, "scheduler.payments.nightly", 15*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	_ = lk.Token // store with any write that must be fenced
	if err := c.Unlock(ctx, "scheduler.payments.nightly"); err != nil {
		log.Fatal(err)
	}
}
```

```python
from clusdr import local

c = local()
print(c.members())
lk = c.lock("scheduler.payments.nightly", ttl=15)
c.unlock(lk.name)
c.close()
```

```rust
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let c = clusdr::local(clusdr::Options::new()).await?;
    println!("{:?}", c.members().await?);
    let lk = c.lock("scheduler.payments.nightly", Some(std::time::Duration::from_secs(15))).await?;
    c.unlock(&lk.name).await?;
    Ok(())
}
```

```ts
import { local } from "clusdr";

const c = await local();
console.log(await c.members());
const lk = await c.lock("scheduler.payments.nightly", 15);
await c.unlock(lk.name);
await c.close();
```

```java
import io.clusdr.Clusdr;
import io.clusdr.Cluster;
import io.clusdr.Lock;
import java.time.Duration;

try (Cluster c = Clusdr.local()) {
    System.out.println(c.members());
    Lock lk = c.lock("scheduler.payments.nightly", Duration.ofSeconds(15));
    c.unlock(lk.name());
}
```

`Local` / `local()` / `Clusdr.local()` dials `CLUSDR_GRPC_ADDR` or `127.0.0.1:7947`, then waits on the Health RPC (10s). Name the lock after the work (`scheduler.payments.nightly`), not `foo` — that is the exclusive job name your workers contend on. Release with `Unlock` / `unlock` on the **client**, not on the `Lock` value.

Do not join the cluster from the app, do not `Dial` a remote node’s Runtime as the normal path, and do not treat `publish` as durable storage.

## On Kubernetes (DaemonSet)

A pod’s `127.0.0.1` is that pod, not the node daemon. Copy this Deployment. Sidecar topology still uses `Local()` on `127.0.0.1` — do not set `hostIP` there. Not a webhook, not a kube SDK.

```yaml
# Not a clusdr member. Not a Service of clusdr.
# Sidecar topology: skip this — Local() on 127.0.0.1.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: default
  labels:
    app.kubernetes.io/name: app
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: app
    spec:
      containers:
        - name: app
          image: your.registry/app:tag
          env:
            - name: NODE_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
            - name: CLUSDR_GRPC_ADDR
              value: "$(NODE_IP):7947"
            - name: CLUSDR_DATA_DIR
              value: /var/lib/clusdr
            # Dev without PEMs: CLUSDR_TLS=disabled on daemon and app.
            # - name: CLUSDR_TLS
            #   value: disabled
          volumeMounts:
            - name: clusdr-data
              mountPath: /var/lib/clusdr
              readOnly: true
      volumes:
        - name: clusdr-data
          hostPath:
            path: /var/lib/clusdr
            type: Directory
```

Same file: [`examples/k8s/app.yaml`](https://github.com/clusdr/clusdr/blob/main/examples/k8s/app.yaml).

## Checkpoint

`Members` returns the same nodes you saw in `clusdr members`. You hold `scheduler.payments.nightly` until `Unlock` / `unlock`. If `Local()` times out, the daemon is down or TLS does not match ([Errors](../reference/errors.md#applications)). On Kubernetes the same `Local()` call needs the Deployment snippet above.

The tutorial ends here. Real NICs and Docker: [Run on other hosts](other-hosts.md).
