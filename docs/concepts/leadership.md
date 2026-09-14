# Leadership

Hashicorp Raft elects one **leader**. That process is the only writer of replicated state (membership, locks, leases).

Followers and observers accept the same Runtime API. Mutations they cannot commit locally are **forwarded** to the leader.

## How you see it

- CLI: `clusdr leader` and the `leader` / role column on `clusdr members`
- SDK: `Leader()`
- Watch: `leader.changed`

## Timeouts

Defaults: 150ms Raft heartbeat and election, 75ms leader lease. Failover with those values is typically a few hundred milliseconds.

If peer RTT is larger, raise the timeouts or you get extra elections. `leader_lease_timeout` must be ≤ `heartbeat_timeout`.

## Bootstrap

Only the first voter uses `clusdr start --bootstrap`. Passing `--bootstrap` again on a node that already bootstrapped is safe (`ErrCantBootstrap` is ignored).

Observers never become leader. After [promote](observers.md), they can.

## Related

- [Membership](membership.md)
- [Configuration](../reference/configuration.md)
- [`clusdr leader`](../reference/cli/leader.md)
