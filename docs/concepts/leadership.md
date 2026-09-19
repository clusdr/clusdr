# Leadership

Hashicorp Raft elects one **leader**. That process is the only writer of replicated state (membership, locks, leases). Followers and observers accept the same Runtime API; mutations they cannot commit locally are **forwarded** to the leader. If there is no leader, those RPCs return `Unavailable` — wait until majority is up, or fix advertised addresses so peers can elect ([other hosts](../guide/other-hosts.md)).

## How you see it

- CLI: `clusdr leader` and the `leader` / role column on `clusdr members`
- SDK: `Leader()`
- Watch: `leader.changed`

## Timeouts

Defaults: 150ms Raft heartbeat and election, 75ms leader lease. With those values, failover is a few hundred milliseconds on a LAN (sub-millisecond RTT). `leader_lease_timeout` must be ≤ `heartbeat_timeout` or the leader can time out while still sending heartbeats and you get extra elections.

If peer RTT is larger than those timers (cross-region WAN, overloaded hosts), raise `raft.heartbeat_timeout` and `raft.election_timeout` together or you get extra elections. Knobs: [Configuration](../reference/configuration.md). Caps: [Limits](../reference/limits.md).

## Bootstrap

Only the first voter uses `clusdr start --bootstrap`. Passing `--bootstrap` again on a node that already bootstrapped is safe (`ErrCantBootstrap` is ignored). Passing it on a **new** `data.dir` creates a second cluster — join instead ([grow](../guide/grow.md)).

Observers never become leader. After [promote](observers.md), they can.

## Related

- [Membership](membership.md)
- [Configuration](../reference/configuration.md)
- [`clusdr leader`](../reference/cli/leader.md)
