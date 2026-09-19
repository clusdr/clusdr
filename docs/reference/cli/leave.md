# `clusdr leave`

`clusdr leave` removes a member from the Raft configuration. Use it when you intend to take an id out of the cluster permanently. No argument leaves the local node. This is the only `RemoveServer`. Presence expiry and heartbeat misses do not call it — a crash is [`clusdr start`](start.md), not leave.

If you skip leave and just kill the process, the Raft id stays and Watch emits `member.dead`, not `member.left`. After leave, the same `data.dir` is not a member; add it again with [`clusdr join`](join.md).

## Synopsis

```bash
# IDs come from the ID column of clusdr members.
clusdr members
clusdr leave
clusdr leave <node-id>
clusdr leave --config joiner.yaml
```

## Behavior

The leader removes the Raft server and commits `drop_member`. Watch emits `member.left`; the id is gone from `Members()`. Followers forward. Crash uses `member.dead` and does not take this path.

| Condition | Result |
|---|---|
| Unknown id | Error (`NotFound`) |
| Already gone | Success |

After leave, the same `data.dir` is not a member. Add it again with [`clusdr join`](join.md). A crash or reboot that was only marked not-alive is [`clusdr start`](start.md), not leave and not join.

## Errors

| You see | What to do |
|---|---|
| `NotFound` / unknown id | Use an id from `clusdr members`. A missing process is not leave |
| `dial daemon at …` | Local Runtime is down. Start this node, then leave ([errors](../errors.md#daemon-and-dial)) |
| `Unavailable` / no leader | Majority must be up so the leader can `RemoveServer` ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Presence](../../concepts/presence.md)
- [Membership](../../concepts/membership.md)
- [Grow the cluster](../../guide/grow.md)
- [ControlService](../api/control.md)
