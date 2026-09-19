# `clusdr promote`

`clusdr promote` turns an observer into a Raft voter. Use it when a host that joined with `--observer` should now count in quorum. No argument promotes the local node. There is no demote in this version.

After promote, lock mutations work on that daemon (forward, or local if it later leads). Until then, Lock / TryLock / Unlock / Renew return `FailedPrecondition` (`observer cannot mutate locks`).

## Synopsis

```bash
# IDs and ROLE come from clusdr members.
clusdr members
clusdr promote
clusdr promote <observer-id>
clusdr promote --config obs.yaml
```

## Behavior

The leader updates the Raft configuration (`AddVoter` on the existing id) and membership role.

| Condition | Result |
|---|---|
| Unknown id | Error (`NotFound`) |
| Already a voter | Success (no-op) |

## Errors

| You see | What to do |
|---|---|
| `NotFound` / unknown id | Use an id from `clusdr members` whose ROLE is `observer` |
| `dial daemon at …` | Local Runtime is down. Start this node, then promote ([errors](../errors.md#daemon-and-dial)) |
| `Unavailable` / no leader | Majority must be up so the leader can `AddVoter` ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Grow the cluster](../../guide/grow.md)
- [Observers](../../concepts/observers.md)
