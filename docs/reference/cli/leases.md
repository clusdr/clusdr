# `clusdr leases`

`clusdr leases` lists every lease currently held: name, owner, token, deadline. Use it to see application grants and, when presence is on, `presence.<nodeID>` rows. Empty list (`no leases`) is a valid success.

The command lists only. Grant, renew, and revoke are SDK RPCs. Presence expiry marks a member `dead`; it does not `leave`. An observer **can** hold application leases.

## Synopsis

```bash
clusdr leases
clusdr leases --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`). Pass the same `--config` as the daemon.

Presence grants appear as `presence.<nodeID>` when enabled.

## Errors

| You see | What to do |
|---|---|
| `dial daemon at …` | Runtime is down or `grpc.addr` does not match ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS mismatch ([errors](../errors.md#tls)) |

## See also

- [Leases](../../concepts/leases.md)
- [LeaseService](../api/leases.md)
