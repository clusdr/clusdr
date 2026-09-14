# `clusdr certs show`

Prints CA fingerprint and node cert fields from `ca.crt` and `node.crt` in `data.dir`. Works while the daemon is running.

The join token hash is in `state.db`. If the daemon holds that file, the line is `(store locked)`. Plaintext is shown only at `clusdr init`.

## Synopsis

```bash
clusdr certs show
```

Needs `data.dir` from `init` or a successful join.

## See also

- [Security](../../concepts/security.md)
- [Start the first member](../../guide/first-member.md) (TLS is on)
