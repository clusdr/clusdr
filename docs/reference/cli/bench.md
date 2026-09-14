# `clusdr-bench`

Separate binary. In-process load generator. Not a cluster member and not a release artifact. Build from source with `make bench`.

## Synopsis

```bash
make bench
./bin/clusdr-bench
./bin/clusdr-bench --scenario events --event-nodes 25 --json
```

Scenarios: election, event fanout, member list, lock acquire.

Targets it checks (p95): election < 500ms, event fanout < 100ms, lock acquire < 50ms. Member-list latency is reported without a fail line. Exit 1 if a targeted scenario misses. Not a 24-hour soak.
