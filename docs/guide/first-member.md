# Start the first member

Clusdr is not a library you embed. It is a **daemon on this host**. Your app will talk only to that process. The daemon is the Raft member.

```text
your app  ──►  clusdr daemon on this machine  ──►  other clusdr daemons
```

Same idea as a local Docker engine: containers do not join a swarm by themselves; the engine does.

You need `clusdr` on `PATH` from [step 1](install.md).

## Create identity

```bash
clusdr init
```

This writes `clusdr.yaml` in the current directory (or `--config`), a cluster id, a node id, a cluster CA, a seed certificate, and a **join token**. It does not start anything.

The YAML is short: ids, advertised `node.addr`, log. Other knobs stay at built-in defaults until you add them. How to edit that file, and a server layout: [Configuration](../reference/configuration.md).

The token is printed **once**. Copy it. You need it in [step 3](grow.md). The hash is stored; the plaintext is not.

`--force` overwrites an existing config file. A second `init` without `--force` is an error unless this `data.dir` is already initialized (then it is success and does not print a new token).

TLS is on. `CLUSDR_TLS=disabled` is only for local experiments, and then every node and every client must set it.

## Bootstrap Raft

```bash
clusdr start --bootstrap
```

This process is now the first voter and the leader. Leave it running.

`--bootstrap` is only for the first Raft member. Passing it again on a node that already bootstrapped is safe.

If you never ran `init`, `start` still runs and logs that identity is missing. That is not a cluster.

Data, certs, and the control socket live under `$HOME/.clusdr`. Two processes must not share one `data.dir`.

## Ask who is here

Open another terminal. No extra env:

```bash
clusdr members
clusdr leader
```

Success: one row, status `alive`, role `leader`. `leader` prints that same node.

`clusdr members` is the check that the Runtime API is up. It dials `127.0.0.1:7947`.

`clusdr status` only asks whether the Unix socket file exists (`$HOME/.clusdr/clusdr.sock`). A daemon that failed to bind the socket looks “down”. Prefer `members`.

## See the first events

```bash
clusdr watch
```

You get a snapshot (`member.join`, `leader.changed`, `seq = 0`), then `watch.sync`, then a live stream. Ctrl-C stops it.

That stream is how applications learn “who joined / who left / who leads” without polling. You will publish on it in [step 4](watch.md) and consume it from code in [step 5](from-your-app.md).

## What you just built

- One voter. Writes (membership, locks, leases) go through this process.
- One leader. There is no failover yet — there is nobody else to elect.
- A CA and a join token. The next machine cannot wander in.

Keep this `start` running. Next page adds a second daemon on the same laptop.

## Next

[Grow the cluster →](grow.md)
