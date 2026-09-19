# Run on other hosts

Point every peer at addresses they can dial. `127.0.0.1` from the tutorial cannot form a cluster across machines.

`clusdr` is already on each host ([Install](install.md)), or use the published Linux image below.

## 1. Set dialable addresses

On **every** node:

| Knob | Must be |
|---|---|
| `raft.addr` / `CLUSDR_RAFT_ADDR` | Host:port **every peer** can dial |
| `node.addr` / `CLUSDR_NODE_ADDR` | Dialable Runtime address stored in membership (not `0.0.0.0`) |
| `grpc.addr` / `CLUSDR_GRPC_ADDR` | Bind address for the Runtime API |
| `data.dir` | Unique per process |

Laptop vs `/etc` + `/var/lib`: [Configuration](../reference/configuration.md).

## 2. Form the cluster

Seed: `clusdr init`, then `clusdr start --bootstrap`.  
Others: `clusdr start` without bootstrap, then `clusdr join --token … <seed-runtime>`.

A reboot of the **same** `data.dir` is `clusdr start` again. `join` only after [`clusdr leave`](../reference/cli/leave.md) or a new `data.dir`. Do not `init` a second time.

Prefer 3 or 5 **voters**. Extra machines that only need a local API: `join --observer`.

TLS stays on unless every node and every client sets `CLUSDR_TLS=disabled`. Confirm certs with `clusdr certs show`.

## 3. Optional: Docker image

```bash
docker pull durguto/clusdr
```

Same tags on GHCR: `ghcr.io/clusdr/clusdr`. Distroless, non-root, `ENTRYPOINT /clusdr`, `CMD start`, volume `/var/lib/clusdr`, port **7947**. Map **7946** if Raft peers sit outside the container network.

In-tree `docker compose` is one node and does **not** run `init`.

## Checkpoint

| Check | What it proves |
|---|---|
| `clusdr version` | Binary runs |
| `clusdr members` | Membership + Runtime API |
| `clusdr health` | Process is serving gRPC |

Prefer `members` or `health`. Do not use `clusdr status` as a Kubernetes probe.

Same host model on a node: [Run on Kubernetes](kubernetes.md).
