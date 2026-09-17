# Run on other hosts

[Step 2](first-member.md) and [step 3](grow.md) used `127.0.0.1`. That cannot form a cluster across machines. You now point every peer at addresses they can actually dial.

`clusdr` is already on each host ([step 1](install.md)), or you use the published Linux image below.

## Addresses

On **every** node:

| Knob | Must be |
|---|---|
| `raft.addr` / `CLUSDR_RAFT_ADDR` | Host:port **every peer** can dial |
| `node.addr` / `CLUSDR_NODE_ADDR` | Dialable Runtime address stored in membership (not `0.0.0.0`) |
| `grpc.addr` / `CLUSDR_GRPC_ADDR` | Bind address for the Runtime API |
| `data.dir` | Unique per process |

Config file vs data dir vs socket, laptop vs `/etc` + `/var/lib`: [Configuration](../reference/configuration.md).

Seed: `clusdr init`, then `clusdr start --bootstrap`.  
Others: `clusdr start` without bootstrap, then `clusdr join --token … <seed-runtime>`.

After that, a reboot of the **same** `data.dir` is `clusdr start` again. Presence expiry does not eject the Raft server. `join` only after [`clusdr leave`](../reference/cli/leave.md) or a new `data.dir`. Do not `init` a second time.

Prefer 3 or 5 **voters**. Extra machines that only need a local API: `join --observer` ([step 3](grow.md)).

Do not share `data.dir` between two processes. Back it up if you need identity and the log after a replace.

TLS stays on. Do not set `CLUSDR_TLS=disabled` unless every node and every client does. Confirm certs with `clusdr certs show`. Apps on a host load PEMs from that host's `data.dir`.

## Docker Hub

The pipeline publishes `durguto/clusdr` (linux/amd64 and linux/arm64). GHCR carries the same tags as `ghcr.io/clusdr/clusdr`.

```bash
docker pull durguto/clusdr
```

Image: distroless non-root, `ENTRYPOINT /clusdr`, `CMD start`, volume `/var/lib/clusdr`, exposes **7947**. Map **7946** if Raft peers sit outside the container network.

`docker compose` in the daemon tree is one node and does **not** run `init`. A compose healthcheck that calls `clusdr version` only proves the binary runs.

## Is it up?

These checks are not interchangeable.

| Check | What it proves |
|---|---|
| `clusdr version` | Binary runs |
| `clusdr status` | Control socket **file** exists (default `$HOME/.clusdr/clusdr.sock`) |
| `clusdr health` / Health RPC | Process is serving gRPC. In this version `healthy` is always `true` and `role` is always `standalone` |
| `clusdr members` | Membership + Runtime API |

Prefer `members` or `health` over Runtime TCP. Do not use `status` as a Kubernetes readiness probe ([Kubernetes](kubernetes.md)).

## You are done with the guide

You installed a binary, bootstrapped a member, grew the cluster, watched the stream, called it from an app, and pointed it at real addresses.

The same host model on a Kubernetes node: [Run on Kubernetes](kubernetes.md). Helm / Operator / Sidecar are their own pages. Example YAML: [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s).

- Still deciding if this is the right tool: [Overview](../overview.md)
- Guarantees (what is on Raft, how presence works): [concepts](../concepts/)
- A flag, RPC, or SDK type: [reference](../reference/)
- Token, TLS, dial, locks: [Errors](../reference/errors.md)
