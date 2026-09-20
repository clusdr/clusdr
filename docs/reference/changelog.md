---
aside: false
outline: false
title: Changelog
description: Notable changes in each clusdr release.
---

# Changelog

Notable changes in each release. The daemon and the language SDKs share one version number, so a tag you already installed is the same train on every language. Not a substitute for [Compatibility](compatibility.md).

## Unreleased

## 0.2.1-rc.3 — 2026-09-20

Prerelease. Images, chart, operator YAML, and cosign signatures use this tag. `:latest`, `latest.json`, Artifact Hub metadata, BSR, and language SDK registries stay on 0.2.0. `v0.2.1-rc.1` and `v0.2.1-rc.2` are tags only — Cosign v3 aborted goreleaser before a GitHub Release.

### Added

- `clusdr-soak`: 24h in-process join/leave + lock/lease churn, then heap, goroutine, and slog noise checks. CI runs the same loop compressed. Not a release artifact.
- Kubernetes: prepare DaemonSet (`mkdir` + `chown 65532` on hostPath). Helm and the Operator render it. No `docker exec` onto the node.
- Operator: omit `spec.seedNodeName`. It writes `status.seedNodeName` on the CR first (optimistic concurrency), then pins init + `--bootstrap` to that node. Helm still requires `--set seed.nodeName`.
- Operator: one apply — `https://clusdr.io/download/clusdr-operator-bundle.yaml` (CRDs, then RBAC + Deployment). The two-file apply stays as pin/fallback. Not a Helm chart of the Operator.
- `kubectl get clusdrcluster` WARNING column: two or more `ClusdrCluster` objects set `status.warning` to `two clusters`. Not a validating webhook.
- App snippet: Downward API `status.hostIP` → `CLUSDR_GRPC_ADDR`, PEM mount or `CLUSDR_TLS=disabled` (dev). Same block in from-your-app, Operator guide, and `examples/k8s/app.yaml`. Not a webhook. Sidecar stays `Local()` / `127.0.0.1`.
- Docs: the DaemonSet Helm chart **never** carries CRDs (permanent; Helm CRD lifecycle). Drain / PreStop / eviction are reboot, not `leave`. Future leave trigger is a deleted Node object only.

### Fixed

- Go SDK: missing client PEMs fail the TLS setup instead of dialing plaintext. Other official SDKs already failed closed.
- Release: Cosign v3 `sign` / `sign-blob` keep `--new-bundle-format=false --use-signing-config=false` so `checksums.txt.sig` / `.pem`, the chart blob, and the legacy `sha256-*.sig` image tags are written.

## 0.2.0 — 2026-09-17

### Added

- [Run on Kubernetes](../guide/kubernetes.md): same Linux host model, one daemon per node. Examples in [`examples/k8s/`](https://github.com/clusdr/clusdr/tree/main/examples/k8s)
- Helm chart: `helm install clusdr oci://ghcr.io/clusdr/charts/clusdr --version <x.y.z>`. Join is still `clusdr join`. Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr)
- `ClusdrCluster` CRD and `clusdr-operator`. Install with `https://clusdr.io/download/clusdr-crds.yaml` and `clusdr-operator.yaml`. Image: `durguto/clusdr-operator`
- `clusdr health` (Runtime RPC) and `clusdr leave` (the only way to remove a Raft member)
- Linux archives and `install.sh` from GitHub Releases (`https://clusdr.io/download/` 302s there)
- Buf for proto; BSR modules [`buf.build/clusdr/api`](https://buf.build/clusdr/api) and [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal)

### Changed

- A crash marks a member `dead` without removing it from Raft. `clusdr leave` is how a member leaves. Restart with the same `data.dir` is `clusdr start`, not another `join`
- Member status is `alive` or `dead` only
- `clusdr init` succeeds (no new token) when identity already exists
- gRPC message names follow Buf STANDARD. RPC paths and field numbers are unchanged

## 0.1.4 — 2026-09-17

### Added

- TypeScript SDK: `npm install clusdr`
- Java SDK: `io.clusdr:clusdr`
- TypeScript and Java copies of the `who`, `scheduler`, `watch`, `worker`, and `agent` examples

### Changed

- Go module path is `github.com/clusdr/clusdr`. Install with `go get github.com/clusdr/clusdr/sdk`

## 0.1.3 — 2026-09-16

### Added

- Rust SDK: `clusdr = "0.1.3"`
- Examples in Go, Python, and Rust: `who`, `scheduler`, `watch`, `worker`, `agent`

### Changed

- Source moved to [github.com/clusdr](https://github.com/clusdr/clusdr). Docker Hub is still `durguto/clusdr`

## 0.1.2 — 2026-09-15

### Added

- Watch filters by topic and event type in the SDKs (`WithTopics`, `watch(topics=…)`)
- Examples: Go `who` / `scheduler` / `worker`, Python `watch` / `agent`
- GHCR image `ghcr.io/clusdr/clusdr` (same tags as Docker Hub)

## 0.1.1 — 2026-09-15

### Changed

- Published image is `durguto/clusdr`

## 0.1.0 — 2026-09-14

First release.

### Added

- Local daemon: membership, leader election, watch, publish, locks, and leases
- Observer members (`clusdr join --observer`) and `clusdr promote`
- mTLS between members, join tokens
- Go and Python SDKs
- Linux install script and a Docker image
