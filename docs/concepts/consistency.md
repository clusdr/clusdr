# Consistency

clusdr uses Raft for **cluster metadata**: who is a member, who holds a lock or lease, who is leader. That is a quorum write. Application data, files, and custom-event payloads are not on that log. If you need a strongly consistent user database, put it in your own store; this page is about what the daemon itself guarantees.

## Replicated (quorum write)

- Membership and leadership
- Locks and leases (including presence)

The leader is the only writer. Followers and observers forward mutations they cannot commit (except observer **locks**, which are rejected — [observers](observers.md)). A minority partition cannot grant a lock the majority never saw.

## Not replicated

- Application data, files, large datasets
- Custom event payloads

Custom events are 1-hop gossip. A partition loses those signals. Membership and locks on the log are not lost that way.

Putting user payloads on the Raft log would make every publish a quorum write and bloat snapshots. That is a non-goal.

## Related

- [Architecture](../architecture.md)
- [Custom events](events.md)
- [Limits](../reference/limits.md)
