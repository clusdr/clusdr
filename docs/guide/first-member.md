# Start the first member

clusdr is not a library you embed. Your app talks only to this process, and this process is the Raft member. The steps below turn the binary from [step 1](install.md) into one running voter.

If `clusdr` is not on `PATH`, go back to [Install](install.md) — every command below assumes `clusdr version` already works.

## 1. Create identity

```bash
clusdr init
```

This writes `clusdr.yaml` in the current directory (or `--config`), a cluster id, a node id, a cluster CA, a seed certificate, and a **join token**. It does not start anything. Without this file and disk identity, `start --bootstrap` can still run but logs that identity is missing — that is not a cluster, and [step 3](grow.md) cannot join.

The token is printed **once**, on a line like `join token : <value>`. Copy the value. You need it in [step 3](grow.md). The hash is stored; the plaintext is not. If you lose it, a second `init` without `--force` succeeds only when this `data.dir` is already initialized and **does not print a new token**. `--force` overwrites the file and prints a new token — do that only if no peer has joined yet, or existing peers will reject the new hash ([Errors](../reference/errors.md)).

TLS is on so peer traffic is encrypted without extra setup. `CLUSDR_TLS=disabled` is only for a closed laptop loop, and then **every** node and every client must set it or handshakes fail.

## 2. Bootstrap Raft

```bash
clusdr start --bootstrap
```

Leave it running in this terminal. `--bootstrap` is only for the first Raft member. Passing it again on a node that already bootstrapped is safe (idempotent). Passing it on a second process that shares `data.dir` is not — BoltDB flocks the store and the second process fails to open it.

Data, certs, and the control socket live under `$HOME/.clusdr` unless you changed `data.dir`. Two processes must not share one `data.dir`.

## 3. Ask who is here

Another terminal. No extra env — the CLI dials `127.0.0.1:7947` by default:

```bash
clusdr members
clusdr leader
```

If `members` cannot dial, the `start` process is not up or not listening on 7947. Check that terminal’s logs. Do not use `clusdr status` as the check: it only asks whether `$HOME/.clusdr/clusdr.sock` exists.

## Checkpoint

`members` shows one row, status `alive`, role `leader`. `leader` prints that same node.

Keep this `start` running. Next: [Grow the cluster →](grow.md)

How the YAML is edited later: [Configuration](../reference/configuration.md). Why the app is not this process: [Daemon](../concepts/daemon.md).
