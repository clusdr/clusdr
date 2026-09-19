# Compatibility

This version is built and tested against the layers in the tables below. Pick a toolchain, SDK, or host OS from that list. Anything else is unclaimed — do not treat an unlisted platform as supported.

## Supported

| Layer | Value |
|---|---|
| Daemon language / build | Go 1.27 |
| Application protocol | gRPC, package `clusdr.v1alpha1` |
| Schema registry | [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (apps / SDKs), [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal) (join / heartbeat / control) |
| Go SDK module | `github.com/clusdr/clusdr/sdk` — [docs](../sdk/go.md) |
| Python SDK | `pip install clusdr`, CPython 3.10+ — [docs](../sdk/python.md) |
| Rust SDK | crate `clusdr`, Rust 1.82+ — [docs](../sdk/rust.md) |
| TypeScript SDK | `npm install clusdr`, Node.js 20+ — [docs](../sdk/typescript.md) |
| Java SDK | `io.clusdr:clusdr`, Java 17+ — [docs](../sdk/java.md) |
| Published daemon | GitHub Releases. `https://clusdr.io/download/` 302s there |
| Install script | `https://clusdr.io/install.sh` (Linux amd64/arm64) |
| Container | Docker Hub `durguto/clusdr` (linux/amd64, linux/arm64); GHCR mirror `ghcr.io/clusdr/clusdr` |
| Operator image | Docker Hub `durguto/clusdr-operator` (linux/amd64, linux/arm64); GHCR `ghcr.io/clusdr/clusdr-operator`. Same tag as the daemon. Not baked into `durguto/clusdr` |
| Release train | `0.2.0` — daemon tag, Go modules `sdk`/`api`, PyPI `clusdr`, crates.io `clusdr`, npm `clusdr`, Maven `io.clusdr:clusdr` |
| Local storage | BoltDB under `data.dir` |
| Consensus | Hashicorp Raft |
| Default OS assumption | Linux (Unix control socket) |
| Multi-host cluster | Linux daemons on different machines. Set dialable `raft.addr` and advertised `node.addr` ([other hosts](../guide/other-hosts.md)). Localhost defaults are laptop-only |
| Kubernetes | A place to run the Linux host model ([Kubernetes](../concepts/kubernetes.md)). How-to: [YAML](../guide/kubernetes.md), [Helm](../guide/kubernetes-helm.md), [Operator](../guide/kubernetes-operator.md), [Sidecar](../guide/kubernetes-sidecar.md). Not a replacement for Lease / probes / EndpointSlice / etcd-for-kube |

CI in this train is same-host (multiple processes / in-memory partition). Two-VM jobs are not in the matrix; that is a test gap, not “Raft is best effort.”

## Best effort

| Layer | Note |
|---|---|
| `docker compose up` | One container, `clusdr start`, no `init`. Identity warning in logs. Healthcheck is `clusdr version`, not cluster Health |
| `CLUSDR_TLS=disabled` | Plaintext. Development only. A mixed cluster fails the handshake ([errors](errors.md#tls)) |
| `go install` of `cmd/clusdr` | Requires a Go toolchain. Not the operator path |

## Not supported

| Layer | Note |
|---|---|
| Windows or macOS as a documented host | Control API is a Unix socket. Operator binaries are Linux only |
| apt / rpm / Snap / Homebrew | Not published. Use the install script or the image |
| HTTP/JSON Runtime API | gRPC only |
| Kubernetes sidecar injection | Not in tree. Sidecar topology is an explicit `ClusdrCluster` / YAML StatefulSet, not a webhook |

API package name **v1alpha1** means the wire shape can still change. [Limits](limits.md).
