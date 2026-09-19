# `clusdr init`

`clusdr init` creates a new cluster identity on this machine: cluster id, node id, `clusdr.yaml`, BoltDB identity, CA, node cert, and a **join token printed once**. Use it on the first voter before [`clusdr start --bootstrap`](start.md). It does not start the daemon. Without this identity, `start` can still run but logs that the store is empty — that is not a cluster, and peers cannot join.

The token is printed on a `join token :` line. Copy it. The hash is stored; plaintext is not. A second `init` without `--force` succeeds when this `data.dir` already has identity and **does not print a new token**. Lose the first printout and later joins get `UNAUTHORIZED` ([errors](../errors.md#join)).

TLS is **on** by default. `CLUSDR_TLS=disabled` is only for a closed laptop loop, and then every node and client must set it.

## Synopsis

```bash
clusdr init
clusdr init --config /etc/clusdr/clusdr.yaml
# Copy the "join token :" line, then:
clusdr start --bootstrap
```

| Flag | Meaning |
|---|---|
| `--force` | Overwrite an existing config file **and** print a new join token |

`--force` is not how you change ports or `data.dir`. Do it only if no peer has joined yet, or existing peers keep the old hash and reject the new token.

## Output

Prints cluster id, node id, config path, data dir, CA path, CA fingerprint, and the join token once. Store the token. The hash is kept in the store; plaintext is not.

If the config file already exists and `data.dir` already has identity, prints `already initialized` (cluster id, node id, paths) and exits 0. No new token.

## Errors

| You see | What to do |
|---|---|
| `config file "clusdr.yaml" already exists (use --force to overwrite)` | Config exists and `data.dir` has no identity. Keep the file, or `--force` only if you intend to replace config **and** the token ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Start the first member](../../guide/first-member.md)
- [Configuration](../configuration.md)
- [Security](../../concepts/security.md)
