# Compatibility

What this version is built and tested against. Anything else is unclaimed.

## Supported

| Layer | Value |
|---|---|
| Daemon language / build | Go 1.27 |
| Application protocol | gRPC, package `clusdr.v1alpha1` |
| Go SDK module | `github.com/durguto/clusdr/sdk` — [docs](../sdk/go.md) |
| Python SDK | `pip install clusdr`, CPython 3.10+ — [docs](../sdk/python.md) |
| Rust SDK | crate `clusdr`, Rust 1.82+ — [docs](../sdk/rust.md) |
| Published daemon | `https://clusdr.io/download/` (GitHub Releases behind it) |
| Install script | `https://clusdr.io/install.sh` (Linux amd64/arm64) |
| Container | Docker Hub `durguto/clusdr` (linux/amd64, linux/arm64); GHCR mirror `ghcr.io/clusdr/clusdr` |
| Release train | `0.1.2` — daemon tag, Go modules `sdk`/`api`, PyPI `clusdr` |
| Local storage | BoltDB under `data.dir` |
| Consensus | Hashicorp Raft |
| Default OS assumption | Linux (Unix control socket) |
| Multi-host cluster | Linux daemons on different machines. Set dialable `raft.addr` and advertised `node.addr` ([other hosts](../guide/other-hosts.md)). Localhost defaults are laptop-only |

CI in this train is same-host (multiple processes / in-memory partition). Two-VM jobs are not in the matrix; that is a test gap, not “Raft is best effort.”

## Best effort

| Layer | Note |
|---|---|
| `docker compose up` | One container, `clusdr start`, no `init`. Identity warning in logs. Healthcheck is `clusdr version`, not cluster Health |
| `CLUSDR_TLS=disabled` | Plaintext. Development only |
| `go install` of `cmd/clusdr` | Requires a Go toolchain. Not the operator path |

## Not supported

| Layer | Note |
|---|---|
| Windows or macOS as a documented host | Control API is a Unix socket. Operator binaries are Linux only |
| apt / rpm / Snap / Homebrew / Helm | Not published. Use the install script or the image |
| HTTP/JSON Runtime API | gRPC only |
| Kubernetes operator / Helm | Not in this tree |

API package name **v1alpha1** means the wire shape can still change. [Limits](limits.md).
