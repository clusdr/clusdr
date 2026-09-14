# `clusdr start`

Starts the daemon. Blocks until SIGINT/SIGTERM.

## Synopsis

```bash
clusdr start [--bootstrap]
```

| Flag | Meaning |
|---|---|
| `--bootstrap` | First Raft member only. Sets `CLUSDR_RAFT_BOOTSTRAP` for this process |

Safe to pass `--bootstrap` again on a node that already bootstrapped (`ErrCantBootstrap` is ignored).

If `clusdr init` was never run, the process still starts and logs that identity is missing.

## See also

- [Start the first member](../../guide/first-member.md)
- [Leadership](../../concepts/leadership.md)
