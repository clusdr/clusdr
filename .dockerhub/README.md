<p align="center">
  <a href="https://clusdr.io">
    <img src="https://raw.githubusercontent.com/clusdr/clusdr/main/docs/assets/logo-512.png" alt="clusdr" width="96" height="96">
  </a>
</p>

<p align="center"><strong>clusdr</strong></p>
<p align="center">A runtime for the cluster. An SDK for the app.</p>

<p align="center">
  <a href="https://clusdr.io"><img src="https://img.shields.io/badge/docs-clusdr.io-0C0C10" alt="docs"></a>
  <a href="https://github.com/clusdr/clusdr/releases"><img src="https://img.shields.io/github/v/release/clusdr/clusdr?label=release" alt="release"></a>
  <a href="https://hub.docker.com/r/durguto/clusdr"><img src="https://img.shields.io/docker/pulls/durguto/clusdr" alt="image"></a>
  <a href="https://github.com/clusdr/clusdr/blob/main/LICENSE"><img src="https://img.shields.io/github/license/clusdr/clusdr" alt="License"></a>
</p>

---

Linux daemon for cluster awareness. The container **is** the cluster member. Your application talks to it on the same host; it does not join Raft.

Not a database, queue, or Kubernetes. Wire API is **v1alpha1**. TLS is on by default.

```text
Application → local SDK → this container → the rest of the cluster
```

## Supported tags

| Tag | What it is |
|---|---|
| `v0.1.4` | Current release |
| `v0.1.3` | Previous |
| `v0.1.1` | Go 1.27, `durguto/clusdr` |
| `v0.1.0` | First release |
| `latest` | Same image as the newest `vX.Y.Z` |

Platforms: **linux/amd64**, **linux/arm64**. Distroless, non-root.

[Dockerfile](https://github.com/clusdr/clusdr/blob/main/Dockerfile)

## Quick reference

- **Docs:** [clusdr.io](https://clusdr.io) · [Install](https://clusdr.io/docs/guide/install) · [Other hosts](https://clusdr.io/docs/guide/other-hosts)
- **Source / issues:** [github.com/clusdr/clusdr](https://github.com/clusdr/clusdr)
- **Host install (no Docker):** `curl -fsSL https://clusdr.io/install.sh | sh` (Linux amd64/arm64)

## How to use this image

The image `CMD` is `start`. It does **not** run `init`. Identity lives on the volume. Init once, then start.

```bash
docker pull durguto/clusdr
docker volume create clusdr-data

docker run --rm \
  -v clusdr-data:/var/lib/clusdr \
  durguto/clusdr init

docker run --rm -d --name clusdr \
  -e CLUSDR_NODE_ADDR=127.0.0.1:7947 \
  -e CLUSDR_GRPC_ADDR=0.0.0.0:7947 \
  -p 7947:7947 \
  -v clusdr-data:/var/lib/clusdr \
  durguto/clusdr start --bootstrap
```

`CLUSDR_NODE_ADDR` is the address stored in membership. Do not set it to `0.0.0.0`. Across machines, use a host IP every peer can dial, and map **7946** for Raft.

```bash
docker run --rm -d --name clusdr \
  -e CLUSDR_NODE_ADDR=203.0.113.10:7947 \
  -e CLUSDR_GRPC_ADDR=0.0.0.0:7947 \
  -e CLUSDR_RAFT_ADDR=203.0.113.10:7946 \
  -p 7946:7946 -p 7947:7947 \
  -v clusdr-data:/var/lib/clusdr \
  durguto/clusdr start --bootstrap
```

## Image contract

| | |
|---|---|
| `ENTRYPOINT` | `/clusdr` |
| `CMD` | `start` |
| User | non-root (distroless) |
| Volume | `/var/lib/clusdr` |
| Ports | **7947** Runtime gRPC · **7946** Raft |
| `CLUSDR_DATA_DIR` | `/var/lib/clusdr` |
| `CLUSDR_CONTROL_SOCKET` | `/var/lib/clusdr/clusdr.sock` |

Do not share one volume between two processes.

## Compose

In-tree `docker-compose.yml` is **one node** and does not run `init`. A healthcheck that calls `clusdr version` only proves the binary runs.

```yaml
services:
  clusdr:
    image: durguto/clusdr:v0.1.4
    restart: unless-stopped
    environment:
      CLUSDR_NODE_ADDR: "127.0.0.1:7947"
      CLUSDR_GRPC_ADDR: "0.0.0.0:7947"
      CLUSDR_DATA_DIR: "/var/lib/clusdr"
      CLUSDR_LOG_FORMAT: "json"
    ports:
      - "7947:7947"
    volumes:
      - clusdr-data:/var/lib/clusdr

volumes:
  clusdr-data:
```

Init the volume before the first `start --bootstrap`.

## From an app

This image is the daemon, not the SDK.

```bash
go get github.com/clusdr/clusdr/sdk
pip install clusdr
# Cargo.toml: clusdr = "0.1.4"
npm install clusdr
# Maven: io.clusdr:clusdr:0.1.4
```

The app still talks to the local Runtime on this host.
