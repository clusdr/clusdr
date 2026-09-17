# `clusdr leave`

Removes a member from the Raft configuration. No argument leaves the local node.

This is the only `RemoveServer`. Presence expiry and heartbeat misses do not call it.

## Synopsis

```bash
clusdr leave [node-id]
```

## Behavior

The leader removes the Raft server and commits `drop_member`. Watch emits `member.left`; the id is gone from `Members()`. Followers forward. Crash uses `member.dead` and does not take this path.

| Condition | Result |
|---|---|
| Unknown id | Error (`NotFound`) |
| Already gone | Success |

After leave, the same `data.dir` is not a member. Add it again with [`clusdr join`](join.md). A crash or reboot that was only marked not-alive is [`clusdr start`](start.md), not leave and not join.

## See also

- [Presence](../../concepts/presence.md)
- [Membership](../../concepts/membership.md)
- [Grow the cluster](../../guide/grow.md)
- [ControlService](../api/control.md)
