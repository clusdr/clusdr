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

## Errors

Config already exists without `--force`.

## See also

- [Start the first member](../../guide/first-member.md)
- [Security](../../concepts/security.md)
