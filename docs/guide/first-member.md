# Start the first member

You need `clusdr` on `PATH` from [step 1](install.md). This page leaves one running voter.

## 1. Create identity

```bash
clusdr init
```

This writes `clusdr.yaml` in the current directory (or `--config`), a cluster id, a node id, a cluster CA, a seed certificate, and a **join token**. It does not start anything.

The token is printed **once**. Copy it. You need it in [step 3](grow.md).

A second `init` without `--force` is an error unless this `data.dir` is already initialized (then it succeeds and does not print a new token). `--force` overwrites the file.

## 2. Bootstrap Raft

```bash
clusdr start --bootstrap
```

Leave it running. `--bootstrap` is only for the first Raft member. Passing it again on a node that already bootstrapped is safe.

Data, certs, and the control socket live under `$HOME/.clusdr`. Two processes must not share one `data.dir`.

## 3. Ask who is here

Another terminal. No extra env:

```bash
clusdr members
clusdr leader
```

## Checkpoint

`members` shows one row, status `alive`, role `leader`. `leader` prints that same node.

`clusdr members` dials `127.0.0.1:7947`. Prefer it over `clusdr status` (that command only checks that the Unix socket file exists).

Keep this `start` running. Next: [Grow the cluster →](grow.md)

How the YAML is edited later: [Configuration](../reference/configuration.md). Why the app is not this process: [Daemon](../concepts/daemon.md).
