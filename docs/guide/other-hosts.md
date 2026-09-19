# Run on other hosts

`127.0.0.1` cannot form a cluster across machines: `join` will succeed the token check and then fail to keep the peer reachable, or membership will store a loopback address nobody else can call. Point every peer at addresses they can actually dial.

The laptop cluster from the [tutorial](./) is the starting point. `clusdr` must already be on each host ([Install](install.md)), or use the published Linux image in step 3.

## 1. Set dialable addresses

On **every** node, before `join`:

| Knob | Must be |
|---|---|
| `raft.addr` / `CLUSDR_RAFT_ADDR` | Host:port **every peer** can dial. If this is loopback, elections look fine on one box and freeze across boxes |
| `node.addr` / `CLUSDR_NODE_ADDR` | Dialable Runtime address stored in membership. `0.0.0.0` is a bind address — peers cannot dial it |
| `grpc.addr` / `CLUSDR_GRPC_ADDR` | Bind address for the Runtime API |
| `data.dir` | Unique per process. Sharing a directory fails the BoltDB flock |

Laptop vs `/etc` + `/var/lib`: [Configuration](../reference/configuration.md).

## 2. Form the cluster

On the seed:

```bash
clusdr init
clusdr start --bootstrap
```

Copy the `join token :` line. On every other host:

```bash
clusdr start
clusdr join --token "$TOKEN" '<seed-host>:7947'
```

`<seed-host>:7947` is the seed’s Runtime API, not Raft (`7946`).

A reboot of the **same** `data.dir` is `clusdr start` again. Presence expiry does not eject the Raft server. `join` only after [`clusdr leave`](../reference/cli/leave.md) or a new `data.dir` — a second `join` for a live id is the wrong fix for a crash ([Presence](../concepts/presence.md)). Do not `init` a second time unless you mean a new cluster (new CA, new token).

Prefer 3 or 5 **voters** so one death does not lose majority. Extra machines that only need a local API: `join --observer`.

TLS stays on so peer traffic is encrypted. Disable it only when **every** node and every client sets `CLUSDR_TLS=disabled`; a mixed cluster fails the handshake ([Errors](../reference/errors.md#tls)). Confirm certs with `clusdr certs show`.

## 3. Optional: Docker image

```bash
docker pull durguto/clusdr
```

Same tags on GHCR: `ghcr.io/clusdr/clusdr`. Distroless, non-root, `ENTRYPOINT /clusdr`, `CMD start`, volume `/var/lib/clusdr`, port **7947**. Map **7946** if Raft peers sit outside the container network — otherwise join works and Raft does not.

In-tree `docker compose` is one node and does **not** run `init`. A compose healthcheck that calls `clusdr version` only proves the binary runs.

## Checkpoint

| Check | What it proves |
|---|---|
| `clusdr version` | Binary runs |
| `clusdr members` | Membership + Runtime API — prefer this |
| `clusdr health` | Process is serving gRPC |

Do not use `clusdr status` as a Kubernetes probe: it only checks that a Unix socket file exists, and that file is not in the container’s probe path the way you think ([Errors](../reference/errors.md#daemon-and-dial)).

Same host model on a node: [Run on Kubernetes](kubernetes.md).
