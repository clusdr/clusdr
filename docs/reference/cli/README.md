# CLI reference

`clusdr` is one binary: daemon and operator commands.

## Global flags

| Flag | Default | Meaning |
|---|---|---|
| `--config` | `clusdr.yaml` | Config file path |
| `--log-level` | (from config) | `debug` \| `info` \| `warn` \| `error` |
| `--log-format` | (from config) | `text` \| `json` |

`--log-level` and `--log-format` override env and YAML for that invocation.

Commands that talk to a running node dial the **Runtime API** (`grpc.addr`). Only [`status`](status.md) looks at the Unix control socket. [`health`](health.md) is the Runtime Health RPC.

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
