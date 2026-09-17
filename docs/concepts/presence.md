# Presence

When `lease.presence` is true (default), each daemon holds `presence.<nodeID>` (default TTL **3s**).

If that lease expires, the leader marks the member **dead** (`member.dead`). The Raft server **stays** in the configuration and the id stays in `Members()`. Heartbeats are the slower backup (interval 2s, timeout 1s, 3 misses) and use the same liveness path — not `RemoveServer`.

Observers hold presence like voters so a dead observer does not stay listed as alive. They also keep their Raft id.

Turn off with `lease.presence: false` or `CLUSDR_LEASE_PRESENCE`. Heartbeats still mark not-alive; they only take longer.

There is no `disconnect` event. A crash, a hang, a partition, SIGKILL, and a clean process stop all look like `member.dead`. That is liveness, not leave. Leave is only [`clusdr leave`](../reference/cli/leave.md) (`member.left`, gone from the list).

## Crash is not leave

`clusdr join` is for a daemon that is **not** in the Raft configuration: the first time that process joins, or after someone ran `clusdr leave` (or you replaced the host and need a new id).

A restart with the same `data.dir` and `node.id` is `clusdr start` only. Raft still has that server. The process catches up and is marked alive again. Peers do not need you to type `join`. There is no network rediscovery. You do not raise `presence_ttl` to survive a reboot.

| Situation | What you run |
|---|---|
| Restart, same data dir, still a Raft member | `clusdr start` (same `--config`). No `join`. |
| Operator ran `clusdr leave` | Same identity on disk is not a member. `clusdr join --token … <seed-runtime>` |
| New host or new `data.dir` | `start`, then `join` once |

Default **3s** is how quickly Watch shows `member.dead` after a kill. It does not eject the voter. A systemd restart or a machine reboot is `clusdr start` with the same unit and the same `data.dir` — not a second `init`, not a new `node.id`, not another `join`.

A dead voter still counts in quorum until `clusdr leave`. Prefer 3 or 5 voters so one down host does not lose majority.

## Optional: slower `member.dead`

TTL only changes how long a crashed host still looks **alive**. It is not the reboot story.

```yaml
lease:
  presence: true
  presence_ttl: 15s
```

Same knob: `CLUSDR_LEASE_PRESENCE_TTL=15s`. Then restart the daemon with that `--config`.

Knobs: [Configuration](../reference/configuration.md). Addresses after a replace: [other hosts](../guide/other-hosts.md). Explicit remove: [`clusdr leave`](../reference/cli/leave.md).

## Related

- [Leases](leases.md)
- [Membership](membership.md)
- [Grow the cluster](../guide/grow.md)
- [Errors](../reference/errors.md)
