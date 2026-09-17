# `clusdr join`

Tells the **local** daemon to join the cluster member at `<addr>` (that member's Runtime API). The local daemon must already be running.

Use this for a process that is **not** in the Raft configuration yet: first join, or after [`clusdr leave`](leave.md) already removed the node. A restart with the same `data.dir` is `clusdr start` only — not another `join`. Same token and `node.id` if you do have to join again.

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

- [Presence](../../concepts/presence.md) — crash vs `leave` vs `join`
- [`clusdr leave`](leave.md)
- [Errors](../../reference/errors.md)
- [Grow the cluster](../../guide/grow.md)
- [ControlService](../api/control.md)
