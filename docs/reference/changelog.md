# Changelog

## Unreleased

### Added

- TypeScript SDK package `clusdr` on npm ([docs](../sdk/typescript.md), [github.com/clusdr/clusdr-js](https://github.com/clusdr/clusdr-js))
- [examples/](https://github.com/clusdr/clusdr/tree/main/examples): TypeScript copies of `who`, `scheduler`, `watch`, `worker`, and `agent`
- Java SDK artifact `io.clusdr:clusdr` ([docs](../sdk/java.md), [github.com/clusdr/clusdr-java](https://github.com/clusdr/clusdr-java))
- [examples/](https://github.com/clusdr/clusdr/tree/main/examples): Java copies of `who`, `scheduler`, `watch`, `worker`, and `agent`

### Changed

- Go module path is `github.com/clusdr/clusdr` (was `github.com/durguto/clusdr`). Install: `go get github.com/clusdr/clusdr/sdk`. Docker Hub remains `durguto/clusdr`.

## 0.1.3 — 2026-09-16

Daemon, Go modules `sdk`/`api`, Python `clusdr`, and crates.io `clusdr` share `0.1.3`.

### Added

- Rust SDK crate `clusdr` on [crates.io](https://crates.io/crates/clusdr) ([docs](../sdk/rust.md), [github.com/clusdr/clusdr-rust](https://github.com/clusdr/clusdr-rust))
- [examples/](https://github.com/clusdr/clusdr/tree/main/examples): `who`, `scheduler`, `watch`, `worker`, and `agent` each in Go, Python, and Rust

### Changed

- GitHub repositories moved to the `clusdr` organization. Browse and clone `github.com/clusdr/clusdr`. Go module path stays `github.com/durguto/clusdr`. New GHCR tags publish to `ghcr.io/clusdr/clusdr`; Docker Hub remains `durguto/clusdr`.
- Release workflow publishes GitHub archives and Hub/GHCR images only after lint and tests succeed

## 0.1.2 — 2026-09-15

Daemon, Go modules `sdk`/`api`, and Python `clusdr` share `0.1.2`.

### Added

- GHCR mirror `ghcr.io/durguto/clusdr` (same tags as Docker Hub `durguto/clusdr`)
- [Errors](errors.md) lookup: join token, TLS, dial, observer locks
- [examples/](https://github.com/clusdr/clusdr/tree/main/examples): Go `who` / `scheduler` / `worker`, Python `watch` / `agent`
- SDK Watch topic / event-type filters (`WithTopics` / `watch(topics=…)`), same semantics as CLI `--topic` / `--type`
- [Configuration](configuration.md): what `init` writes, how to edit, laptop vs server paths
- [Presence](../concepts/presence.md): reboot vs `join`, default 3s TTL, what to set on a server
- [Compatibility](compatibility.md): multi-host cluster is supported (not “best effort”)
- Go SDK package docs and examples for pkg.go.dev; goroutine / Local vs Dial notes on the SDK walkthrough

### Notes

- Proxy `v0.1.0` for `github.com/odurgut/clusdr` is frozen. Do not retag it. Use `github.com/durguto/clusdr`.

## 0.1.1 — 2026-09-15

First tags on `github.com/durguto/clusdr` (`sdk/`, `api/`). Daemon, Go modules, and Python `clusdr` share `0.1.1`.

### Changed

- Toolchain is Go 1.27 (CI and release use 1.27.1)
- Published image is `durguto/clusdr`
- Nested `sdk/` and `api/` zips include LICENSE and NOTICE

### Notes

- Proxy `v0.1.0` for `github.com/odurgut/clusdr` is frozen. Use `github.com/durguto/clusdr`.
- Hub `odurgut/clusdr` leftover from 0.1.0; pull `durguto/clusdr`

## 0.1.0 — 2026-09-14

First tagged release. Daemon, Go SDK (`github.com/odurgut/clusdr/sdk` + `…/api`), and Python `clusdr` share this version.

### Added

- Daemon CLI: `init`, `start`, `status`, `join`, `promote`, `members`, `leader`, `watch`, `publish`, `locks`, `leases`, `certs show`, `config validate`, `version`
- Runtime gRPC `clusdr.v1alpha1`: Health, Membership, Watch, Events, Locks, Leases, Join, Control, Heartbeat
- Raft membership, locks, leases; presence lease `presence.<id>`
- mTLS from cluster CA; join token; `CLUSDR_TLS=disabled`
- Go SDK `github.com/odurgut/clusdr/sdk` (`Local`, `Dial`)
- Python SDK `pip install clusdr`
- `clusdr-bench` (election / events / members / locks) — source-only, not a release artifact
- Docker image `odurgut/clusdr` and single-node compose file
- Observer nodes: `clusdr join --observer` (Raft non-voter). `Member.role` is `voter` or `observer`
- `clusdr promote [node-id]` turns an observer into a voter
- Observer daemons reject Lock / TryLock / Unlock / Renew; leases still work
- Operator install: Linux amd64/arm64 via `curl -fsSL https://clusdr.io/install.sh | sh`, Docker Hub `odurgut/clusdr`

### Changed

- Raft defaults: 150ms heartbeat and election, 75ms leader lease
- Documentation reorganized into a six-page guide, concepts, SDKs, and reference
- `clusdr certs show` reads `ca.crt` / `node.crt` so it works while the daemon holds `state.db`
- Default control socket is `$HOME/.clusdr/clusdr.sock` (or `/var/lib/clusdr/clusdr.sock` when `HOME` is unset)
