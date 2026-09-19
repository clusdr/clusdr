# `clusdr join`

`clusdr join` tells the **local** daemon to join the cluster member at `<addr>` (that member's Runtime API). Use it for a process that is **not** in the Raft configuration yet: first add, or after [`clusdr leave`](leave.md) already removed the node. The local daemon must already be running (`clusdr start` without `--bootstrap`).

A restart with the same `data.dir` is `clusdr start` only — not another `join`. Same token and `node.id` if you do have to join again. `<addr>` is the seed **Runtime** (`grpc.addr`, default `127.0.0.1:7947`), not Raft (`raft.addr`).

## Synopsis

```bash
# Token is printed once by clusdr init on the seed (the "join token :" line).
# Seed Runtime is grpc.addr, not raft.addr.
clusdr start --config joiner.yaml
clusdr --config joiner.yaml join --token "$TOKEN" 127.0.0.1:7947
clusdr --config joiner.yaml join --observer --token "$TOKEN" 127.0.0.1:7947
clusdr members
```

| Flag | Meaning |
|---|---|
| `--token` | Cluster join token from `clusdr init` |
| `--observer` | Join as a Raft non-voter |

An observer receives the Raft log and serves a local app but does not vote. That daemon **rejects lock mutations** (`FailedPrecondition`) until you [`promote`](promote.md) it. Watch, publish, and leases still work.

## Output

`joined cluster`, optional `node certificate issued`, then a member table (`ID`, `ADDRESS`, `STATUS`, `ROLE`).

## Errors

| Condition | Result | What to do |
|---|---|---|
| Invalid token | `UNAUTHORIZED` | Use the plaintext `clusdr init` printed once ([errors](../errors.md#join)) |
| Cluster id mismatch (both set, different) | Rejected | Leave `cluster.id` empty on the joiner, or copy the seed’s id ([errors](../errors.md#join)) |
| Unreachable addr | Unavailable | `<addr>` is Runtime, not Raft. Wait until `clusdr members` on the seed shows a leader ([errors](../errors.md#join)) |
| Local daemon down | `dial daemon at …` | `clusdr start` this node first ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS reject | Same `tls.mode` everywhere ([errors](../errors.md#tls)) |

## See also

- [Presence](../../concepts/presence.md) — crash vs `leave` vs `join`
- [`clusdr leave`](leave.md)
- [Errors](../../reference/errors.md)
- [Grow the cluster](../../guide/grow.md)
- [ControlService](../api/control.md)
