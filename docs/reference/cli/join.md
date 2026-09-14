# `clusdr join`

Tells the **local** daemon to join the cluster member at `<addr>` (that member's Runtime API). The local daemon must already be running.

## Synopsis

```bash
clusdr join --token <token> [--observer] <addr>
```

| Flag | Meaning |
|---|---|
| `--token` | Cluster join token from `clusdr init` |
| `--observer` | Join as a Raft non-voter |

## Output

`joined cluster`, optional `node certificate issued`, then a member table (`ID`, `ADDRESS`, `STATUS`, `ROLE`).

## Errors

| Condition | Result |
|---|---|
| Invalid token | `UNAUTHORIZED` |
| Cluster id mismatch (both set, different) | Rejected |
| Unreachable addr | Unavailable |

## See also

- [Grow the cluster](../../guide/grow.md)
- [ControlService](../api/control.md)
