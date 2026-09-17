# `clusdr init`

Creates a new cluster on this machine: cluster id, node id, `clusdr.yaml`, BoltDB identity, CA, node cert, and a **join token (once)**.

Does not start the daemon.

## Synopsis

```bash
clusdr init [--force]
```

| Flag | Meaning |
|---|---|
| `--force` | Overwrite an existing config file |

## Output

Prints the join token once. Store it. The hash is kept in the store; plaintext is not.

If the config file already exists and `data.dir` already has identity, prints `already initialized` and exits 0. No new token.

## Errors

Config already exists without `--force` **and** there is no identity in `data.dir`.

## See also

- [Start the first member](../../guide/first-member.md)
- [Configuration](../configuration.md)
- [Security](../../concepts/security.md)
