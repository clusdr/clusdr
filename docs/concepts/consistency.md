# Consistency

Strong consistency via Raft for cluster metadata.

## Replicated (quorum write)

- Membership and leadership
- Locks and leases (including presence)

The leader is the only writer. Followers and observers forward mutations they cannot commit (except observer **locks**, which are rejected).

## Not replicated

- Application data, files, large datasets
- Custom event payloads

Custom events are 1-hop gossip. A partition loses those signals. Membership and locks on the log are not lost that way.

Putting user payloads on the Raft log would make every publish a quorum write and bloat snapshots. That is a non-goal.

## Related

- [Architecture](../architecture.md)
- [Custom events](events.md)
- [Limits](../reference/limits.md)
