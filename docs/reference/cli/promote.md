# `clusdr promote`

Turns an observer into a Raft voter. No argument promotes the local node.

## Synopsis

```bash
clusdr promote [node-id]
```

## Behavior

The leader updates the Raft configuration (`AddVoter` on the existing id) and membership role.

| Condition | Result |
|---|---|
| Unknown id | Error (`NotFound`) |
| Already a voter | Success (no-op) |

## See also

- [Grow the cluster](../../guide/grow.md)
- [Observers](../../concepts/observers.md)
