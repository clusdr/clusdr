# Changelog

## Unreleased

### Added

- GHCR mirror `ghcr.io/durguto/clusdr` (same tags as Docker Hub `durguto/clusdr`)

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
