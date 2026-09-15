# `clusdr config validate`

Loads YAML + env, prints resolved values, exit 0. Does not start anything. Does not write the file.

There is no `clusdr config set`. Edit the YAML, validate, restart the daemon with the same `--config`. The running process does not reload on SIGHUP.

## Synopsis

```bash
clusdr config validate [--config clusdr.yaml]
```

## See also

- [Configuration](../configuration.md) — process, what `init` writes, laptop vs server
