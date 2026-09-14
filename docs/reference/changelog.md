# Changelog

No numbered release has been tagged.

## Unreleased

### Added

- Daemon CLI: `init`, `start`, `status`, `join`, `promote`, `members`, `leader`, `watch`, `publish`, `locks`, `leases`, `certs show`, `config validate`, `version`
- Runtime gRPC `clusdr.v1alpha1`: Health, Membership, Watch, Events, Locks, Leases, Join, Control, Heartbeat
- Raft membership, locks, leases; presence lease `presence.<id>`
- mTLS from cluster CA; join token; `CLUSDR_TLS=disabled`
- Go SDK `github.com/odurgut/clusdr/sdk` (`Local`, `Dial`)
- `clusdr-bench` (election / events / members / locks)
- Docker image and single-node compose file
- Observer nodes: `clusdr join --observer` (Raft non-voter). `Member.role` is `voter` or `observer`
- `clusdr promote [node-id]` turns an observer into a voter
- Observer daemons reject Lock / TryLock / Unlock / Renew; leases still work

### Changed

- Raft defaults: 150ms heartbeat and election, 75ms leader lease
- Documentation reorganized into tutorials, concepts, tasks, and reference
- `clusdr certs show` reads `ca.crt` / `node.crt` so it works while the daemon holds `state.db`
- Operator install is a release binary (`install.sh`, Homebrew tap, Docker Hub `odurgut/clusdr`). Source build stays in `CONTRIBUTING.md`
- User docs are a six-page guide (install → first member → grow → watch → app → other hosts). Task and tutorial pages are gone
- Default control socket is `$HOME/.clusdr/clusdr.sock` (was `/var/run/clusdr.sock`)
- Go SDK: `go get github.com/odurgut/clusdr/sdk`. Python: `pip install clusdr`
- SDK docs are a top-level category (`docs/sdk`): model, Go, Python — not a reference stub
