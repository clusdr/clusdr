# `clusdr start`

`clusdr start` starts the daemon and blocks until SIGINT/SIGTERM. Use it on every boot for a node that already has identity in `data.dir`. The first voter also passes `--bootstrap`. A restart with the same `data.dir` is this command again — do not `join` unless [`clusdr leave`](leave.md) already removed the id.

If you `join` after every crash, you fight a member that is still in Raft. If you skip `init` on the seed, the process still starts and logs that identity is missing — that is not a cluster.

TLS is **on** by default. `CLUSDR_TLS=disabled` must be set on every node and client if you turn it off.

## Synopsis

```bash
clusdr init                      # once, on the seed; copy the join token
clusdr start --bootstrap         # first voter only
# later members, and every reboot of an existing member:
clusdr start
clusdr start --config /etc/clusdr/clusdr.yaml
```

| Flag | Meaning |
|---|---|
| `--bootstrap` | First Raft member only. Sets `CLUSDR_RAFT_BOOTSTRAP` for this process |

Safe to pass `--bootstrap` again on a node that already bootstrapped (`ErrCantBootstrap` is ignored). Passing it on a **new** `data.dir` creates a second cluster — join instead.

If `clusdr init` was never run, the process still starts and logs that identity is missing.

## Errors

| You see | What to do |
|---|---|
| log `node identity not found in store` | Run `clusdr init`, then `start --bootstrap` on the seed ([errors](../errors.md#daemon-and-dial)) |
| `listen grpc runtime …: address already in use` | Another process owns that port. Give this daemon its own `grpc.addr`, `raft.addr`, socket, and `data.dir` ([errors](../errors.md#daemon-and-dial)) |
| Handshake failures after start | Mixed TLS. Same `tls.mode` everywhere ([errors](../errors.md#tls)) |

## See also

- [Start the first member](../../guide/first-member.md)
- [Leadership](../../concepts/leadership.md)
- [Presence](../../concepts/presence.md)
