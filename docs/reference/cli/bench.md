# `clusdr-bench`

`clusdr-bench` is a separate binary: an in-process load generator. Use it when you want local p95 numbers for election, event fanout, member list, or lock acquire. It is not a cluster member and not a release artifact. Build from source with `make bench`.

The process spins an in-process cluster, measures the selected scenarios, and tears the nodes down on SIGINT/SIGTERM. It does not join an existing daemon. Exit 1 if a targeted scenario misses. Long-running stability is [`clusdr-soak`](soak.md).

## Synopsis

```bash
make bench
./bin/clusdr-bench
./bin/clusdr-bench --scenario events --event-nodes 25 --json
./bin/clusdr-bench --scenario election,locks --nodes 3 --ops 10
```

| Flag | Default | Meaning |
|---|---|---|
| `--nodes` | `3` | Raft voters for election and lock scenarios |
| `--event-nodes` | `25` | Event-mesh size for fanout and member-list size |
| `--ops` | `10` | Samples per scenario |
| `--scenario` | `all` | Comma-separated: `all`, `election`, `events`, `members`, `locks` |
| `--json` | false | Write the report as JSON |

Scenarios: election, event fanout, member list, lock acquire.

Targets it checks (p95): election < 500ms, event fanout < 100ms, lock acquire < 50ms. Member-list latency is reported without a fail line. Exit 1 if a targeted scenario misses.
