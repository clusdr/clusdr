# Presence

Presence is how the cluster decides a member is **alive** without treating a crash as leave. Each daemon holds a lease named `presence.<nodeID>` (on by default, TTL **3s**). When that lease expires, the leader marks the member **dead** (`member.dead`) but **keeps** the Raft server and the id in `Members()`. If you raise the TTL hoping a reboot will skip `join`, you are solving the wrong problem — a reboot with the same `data.dir` is `clusdr start`, not another join.

Heartbeats are the slower backup and use the same liveness path — not `RemoveServer`.

| Heartbeat | Value | Why it matters |
|---|---|---|
| Interval | 2s | How often peers ping. |
| Timeout | 1s | A ping that takes longer counts as a miss. |
| Misses | 3 | After three misses the member is `dead` but still in Raft — same as presence expiry. |

Turn presence off with `lease.presence: false` or `CLUSDR_LEASE_PRESENCE=false` only if you want liveness to wait on heartbeats alone. Heartbeats still mark not-alive; they only take longer.

Observers hold presence like voters so a dead observer does not stay listed as alive. They also keep their Raft id.

There is no `disconnect` event. A crash, a hang, a partition, SIGKILL, and a clean process stop all look like `member.dead`. That is liveness, not leave. Leave is only [`clusdr leave`](../reference/cli/leave.md) (`member.left`, gone from the list).

## Crash is not leave

`clusdr join` is for a daemon that is **not** in the Raft configuration: the first time that process joins, or after someone ran `clusdr leave` (or you replaced the host and need a new id).

A restart with the same `data.dir` and `node.id` is `clusdr start` only. Raft still has that server. The process catches up and is marked alive again. Peers do not need you to type `join`. There is no network rediscovery. You do not raise `presence_ttl` to survive a reboot.

| Situation | What you run |
|---|---|
| Restart, same data dir, still a Raft member | `clusdr start` (same `--config`). No `join`. |
| Operator ran `clusdr leave` | Same identity on disk is not a member. `clusdr join --token … <seed-runtime>` — token from `clusdr init` on the seed ([errors](../reference/errors.md#join)) |
| New host or new `data.dir` | `start`, then `join` once |

Default **3s** is how quickly Watch shows `member.dead` after a kill. It does not eject the voter. A systemd restart, a machine reboot, or a Kubernetes cordon/drain/evict/PreStop is `clusdr start` with the same unit and the same `data.dir` — not a second `init`, not a new `node.id`, not another `join`. On Kubernetes a later `leave` automation (not shipped) may fire only when the **Node object is deleted**.

A dead voter still counts in quorum until `clusdr leave`. Prefer 3 or 5 voters so one down host does not lose majority.

## Optional: slower `member.dead`

TTL only changes how long a crashed host still looks **alive**. It is not the reboot story.

```yaml
lease:
  presence: true
  presence_ttl: 15s
```

Same knob: `CLUSDR_LEASE_PRESENCE_TTL=15s`. Then stop the daemon and `clusdr start` with that `--config` — the process does not reload YAML on SIGHUP.

Knobs: [Configuration](../reference/configuration.md). Addresses after a replace: [other hosts](../guide/other-hosts.md). Explicit remove: [`clusdr leave`](../reference/cli/leave.md).

## Related

- [Leases](leases.md)
- [Membership](membership.md)
- [Grow the cluster](../guide/grow.md)
- [Errors](../reference/errors.md)
