---
aside: false
outline: false
title: Changelog
description: Notable changes in each clusdr release.
---

# Changelog

Notable changes in each release. The daemon and the language SDKs share one version number.

## Unreleased

### Added

- Buf for proto: `make proto` runs `buf generate`; pull requests lint and reject wire-incompatible changes
- BSR modules [`buf.build/clusdr/api`](https://buf.build/clusdr/api) (application) and [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal) (join/heartbeat). `main` and `v*` tags push both.

### Changed

- gRPC request/response message names follow Buf STANDARD (`TryLockRequest`, `GrantRequest`, `LockServiceRenewRequest`, `LeaseServiceRenewRequest`). RPC paths and field numbers are unchanged.

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
