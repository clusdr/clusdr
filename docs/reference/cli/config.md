# `clusdr config validate`

`clusdr config validate` loads YAML plus env, prints the resolved values, and exits 0. Use it after you edit a file and before you restart the daemon, to confirm merge order (defaults → YAML → `CLUSDR_*`). It does not start anything and does not write the file.

There is no `clusdr config set`. Edit the YAML, validate, restart the daemon with the same `--config`. The running process does not reload on SIGHUP. If you skip validate, a typo or a `~` path (not expanded) only shows up when `start` fails or binds the wrong port.

## Synopsis

```bash
clusdr config validate
clusdr config validate --config /etc/clusdr/clusdr.yaml
```

`--config` is the global flag (default `clusdr.yaml` in the current directory). The printout is the struct the next `clusdr start --config …` will use, including `grpc.addr` (Runtime) and `raft.addr`.

## See also

- [Configuration](../configuration.md) — process, what `init` writes, laptop vs server
