# `clusdr certs show`

`clusdr certs show` prints CA fingerprint and node cert fields from `ca.crt` and `node.crt` in `data.dir`. Use it after `init` or join to confirm every node trusts the same CA. It works while the daemon is running.

If fingerprints differ, those nodes will fail the handshake instead of forming one cluster ([errors](../errors.md#tls)). Server identity is the node id (SAN), not the dial hostname.

The join token hash is in `state.db`. If the daemon holds that file, the line is `(store locked)`. Plaintext is shown only at `clusdr init`.

## Synopsis

```bash
clusdr init                 # first voter; copy the join token
clusdr certs show
clusdr certs show --config joiner.yaml   # after a successful join
```

Needs `data.dir` from `init` or a successful join. Pass the same `--config` as that process so `data.dir` matches.

## Errors

| You see | What to do |
|---|---|
| `read ca.crt: …` / no PEMs | Run `clusdr init` on the seed, or `clusdr join` on this node, then show again |
| `join token : (store locked)` | Expected while the daemon has `state.db` open. Plaintext is not recoverable ([errors](../errors.md#join)) |

## See also

- [Security](../../concepts/security.md)
- [Start the first member](../../guide/first-member.md) (TLS is on)
