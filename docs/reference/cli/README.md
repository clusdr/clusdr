# CLI reference

`clusdr` is one binary: it starts the daemon and it is the operator CLI. Flags, output, and failure text for a command you already decided to run live below. Form the first voter in [Start the first member](../../guide/first-member.md); add peers in [Grow the cluster](../../guide/grow.md).

| | Default | Why it matters |
|---|---|---|
| TLS | on | Mixed `CLUSDR_TLS` with a client fails handshake, not fallback to plaintext. |
| Runtime API | `grpc.addr` `127.0.0.1:7947` | Most CLI commands dial this, not Raft (`raft.addr`). Wrong port looks like a down daemon. |
| Control socket | Unix file | Only [`status`](status.md) looks at it. Prefer [`members`](members.md) for “is the cluster up.” |
| Health RPC | `healthy=true`, `role=standalone` | [`health`](health.md) is not Raft role. |

## Global flags

| Flag | Default | Meaning |
|---|---|---|
| `--config` | `clusdr.yaml` | Config file path |
| `--log-level` | (from config) | `debug` \| `info` \| `warn` \| `error` |
| `--log-format` | (from config) | `text` \| `json` |

`--log-level` and `--log-format` override env and YAML for that invocation.

CLI commands that talk to the daemon (`members`, `join`, …) must use the same `--config` as that process so they see the same `grpc.addr` and `data.dir`. A second process on the same machine needs its own file.

## Commands

| Command | Page |
|---|---|
| `clusdr version` | [version](version.md) |
| `clusdr init` | [init](init.md) |
| `clusdr start` | [start](start.md) |
| `clusdr status` | [status](status.md) |
| `clusdr health` | [health](health.md) |
| `clusdr config validate` | [config](config.md) |
| `clusdr join` | [join](join.md) |
| `clusdr promote` | [promote](promote.md) |
| `clusdr leave` | [leave](leave.md) |
| `clusdr members` | [members](members.md) |
| `clusdr leader` | [leader](leader.md) |
| `clusdr watch` | [watch](watch.md) |
| `clusdr publish` | [publish](publish.md) |
| `clusdr locks` | [locks](locks.md) |
| `clusdr leases` | [leases](leases.md) |
| `clusdr certs show` | [certs](certs.md) |
| `clusdr-bench` | [bench](bench.md) (separate binary) |
| `clusdr-soak` | [soak](soak.md) (separate binary) |
