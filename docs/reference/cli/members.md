# `clusdr members`

`clusdr members` lists every known Raft member: `ID`, `ADDRESS`, `STATUS`, `ROLE`. Use it as the cluster view — who is here, who is leader, who is an observer. This is the check for “is the Runtime API up,” not [`clusdr status`](status.md) (Unix socket file) and not [`clusdr health`](health.md) (`role` is always `standalone`).

`ADDRESS` is the advertised Runtime address (`node.addr`), not `raft.addr`. `STATUS` is liveness (`alive` or `dead`). A crash marks `dead` and keeps the Raft id; `leave` removes the row. Copy `ID` from this table for [`leave`](leave.md) and [`promote`](promote.md).

## Synopsis

```bash
clusdr members
clusdr members --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`). Pass the same `--config` as the daemon.

Role is `leader`, `voter`, or `observer`.

Empty cluster (should not happen on a running seed): prints `no members`.

## Errors

| You see | What to do |
|---|---|
| `dial daemon at …` / `connection refused` | Runtime is down or `grpc.addr` does not match. Start the daemon; point `--config` at the same YAML ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS mismatch ([errors](../errors.md#tls)) |

## See also

- [Start the first member](../../guide/first-member.md)
- [Membership](../../concepts/membership.md)
- [MembershipService](../api/membership.md)
