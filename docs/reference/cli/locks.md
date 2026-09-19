# `clusdr locks`

`clusdr locks` lists every lock currently held: name, holder, token, deadline. Use it to see who owns a name before you debug an unlock. Empty list (`no locks`) is a valid success.

The command lists only. Acquire and release are SDK RPCs. ListLocks is allowed on an observer; Lock / TryLock / Unlock / Renew on an observer return `FailedPrecondition` (`observer cannot mutate locks`) — take locks on a voter or [`promote`](promote.md) that node.

## Synopsis

```bash
clusdr locks
clusdr locks --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`). Pass the same `--config` as the daemon.

## Errors

| You see | What to do |
|---|---|
| `dial daemon at …` | Runtime is down or `grpc.addr` does not match ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS mismatch ([errors](../errors.md#tls)) |
| `observer cannot mutate locks` | Not this command (list is allowed). Take Lock RPCs on a voter ([errors](../errors.md#locks-and-leases)) |

## See also

- [Locks](../../concepts/locks.md)
- [LockService](../api/locks.md)
