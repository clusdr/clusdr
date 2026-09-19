# `clusdr leader`

`clusdr leader` prints the current Raft leader id and address. Use it when you need the leader’s `node.id` and Runtime address, not the full member table. For the cluster view, use [`clusdr members`](members.md) (role `leader`). [`clusdr health`](health.md) `role` is always `standalone` and is not this.

No leader → `GetLeader` returns `Unavailable`. One voter: that process must be up. Three voters: majority must be up.

## Synopsis

```bash
clusdr leader
clusdr leader --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`). `address` is the leader’s Runtime API, not `raft.addr`.

## Errors

| You see | What to do |
|---|---|
| `Unavailable` / `no leader elected` | Wait for majority. Check `clusdr members` ([errors](../errors.md#daemon-and-dial)) |
| `dial daemon at …` | Local Runtime is down ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Leadership](../../concepts/leadership.md)
- [MembershipService](../api/membership.md)
