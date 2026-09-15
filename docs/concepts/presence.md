# Presence

When `lease.presence` is true (default), each daemon holds `presence.<nodeID>` (default TTL **3s**).

If that lease expires, the leader **removes the node from the Raft configuration** (`RemoveServer` + `remove_member`). Watch emits `member.left`. Heartbeats are the slower backup (interval 2s, timeout 1s, 3 misses) and use the **same** remove path.

Observers hold presence like voters so a dead observer does not stay in the list.

Turn off with `lease.presence: false` or `CLUSDR_LEASE_PRESENCE`. Heartbeats still remove; they only take longer.

There is no `disconnect` event. A crash, a hang, a partition, SIGKILL, and a clean process stop all look like `member.left`. This version has no `clusdr leave`; expiry *is* leave.

## Reboot is not a new join

`clusdr join` is for a daemon that is **not** in the Raft configuration: the first time that process joins, or after presence already removed it.

If the process comes back **before** the TTL fires, with the same `data.dir` and `node.id`, run `clusdr start` only. Raft still has that server. Peers do not need you to type `join` again. There is no network rediscovery.

| Situation | What you run |
|---|---|
| Restart within `presence_ttl`, same data dir | `clusdr start` (same `--config`). No `join`. |
| Down longer than `presence_ttl` | Removed. Same identity on disk, then `join --token … <seed-runtime>` |
| New host or new `data.dir` | `start`, then `join` once |

Default **3s** is a laptop demo: kill a process, see `member.left` immediately. A systemd restart or a machine reboot is almost always longer than 3s, so the leader ejects the node and the next `start` is not enough. That is the “I have to keep adding the host” loop.

## What to set on a server

Set TTL **above** how long this daemon may be gone and still be the same member (process bounce, package upgrade, reboot):

```yaml
lease:
  presence: true
  presence_ttl: 2m
```

Same knob: `CLUSDR_LEASE_PRESENCE_TTL=2m`. Then restart the daemon with that `--config`.

Keep the process managed (`Restart=always` or equivalent) on a **pinned** config file. After a reboot: same unit, same `data.dir` — not a second `init`, not a new `node.id`.

If you still get removed (host down longer than TTL, long partition), `join` again with the **same** token and **same** `node.id`. Put the token in a file the unit can read and point at a stable seed Runtime address. That is a script, not a human at a terminal after every boot.

Raising TTL means a truly dead host stays in quorum until the lease expires. Prefer 3 or 5 voters so one slow reboot does not lose majority. Do not set TTL to hours unless you accept a dead voter that long.

Knobs: [Configuration](../reference/configuration.md). Addresses after a replace: [other hosts](../guide/other-hosts.md).

## Related

- [Leases](leases.md)
- [Membership](membership.md)
- [Grow the cluster](../guide/grow.md)
- [Errors](../reference/errors.md)
