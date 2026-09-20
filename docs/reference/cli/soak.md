# `clusdr-soak`

`clusdr-soak` is a separate binary: an in-process stability run. Use it for a 24-hour (or shorter) cluster that joins and leaves a transient voter, churns locks and leases, then checks heap, goroutines, and log noise. It is not a cluster member and not a release artifact. Build from source with `make soak`.

The process spins an in-process cluster, churns until `--duration` or SIGINT/SIGTERM, prints a report, and tears the nodes down. Exit 1 if a check misses. Latency numbers are [`clusdr-bench`](bench.md).

CI runs the same workload compressed (`go test ./internal/soak`, a few seconds). A full day is:

```bash
make soak
./bin/clusdr-soak
./bin/clusdr-soak --duration 24h --tick 2s --json
CLUSDR_SOAK_DURATION=24h go test -timeout 26h -race -count=1 ./internal/soak -run TestRun_ShortStability
```

| Flag | Default | Meaning |
|---|---|---|
| `--nodes` | `3` | Core Raft voters. Extra members join and leave each tick. |
| `--duration` | `24h` | How long to churn. Cancel still audits completed ticks. |
| `--tick` | `2s` | One join/leave + lock + lease cycle |
| `--json` | false | Write the report as JSON |

Checks: heap growth after warmup < 128 MiB, goroutines +48, no error-level slog, warn only for known expire-apply races, no unexpected Info spam.
