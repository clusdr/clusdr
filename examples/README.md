# Examples

Three small **applications** that talk to a local daemon. They do not join Raft, do not store data, and do not dial a remote Runtime API.

```text
your process  ──►  clusdr on this host  ──►  the rest of the cluster
```

## Prerequisite

```bash
clusdr init
clusdr start --bootstrap
```

Leave that process running. TLS is on; the SDK loads `ca.crt` / `node.crt` / `node.key` from `CLUSDR_DATA_DIR` or `~/.clusdr`. If connect fails, [Errors](../docs/reference/errors.md).

## Programs

| Path | Language | Story |
|---|---|---|
| [who](who) | Go | Inventory: who is alive, who leads, then live `member.*` / `leader.changed` |
| [scheduler](scheduler) | Go | One exclusive scheduler. Run two copies; only one holds the lock |
| [watch](watch) | Python | Snapshot, a `custom.hello`, then the full Watch bus (cluster + gossip) |

### who

Prints the membership table, then stays on Watch until Ctrl-C. Cluster events are labeled; `custom.*` is gossip and is not replayed after a reconnect.

```bash
go run ./examples/who
```

Snapshot only:

```bash
go run ./examples/who -once
```

### scheduler

Loop: try to acquire `scheduler`, do a short “dispatch” while holding the fencing token, release, repeat. Background renew keeps the grant alive during work.

Terminal A and B, same lock name:

```bash
go run ./examples/scheduler -holder replica-a
go run ./examples/scheduler -holder replica-b
```

One replica prints `held … token=…` and dispatches. The other prints `waiting (held by replica-a token=…)`. Kill the holder; the waiter acquires. That is the product: exclusive work without the app joining the cluster.

Observer daemons reject lock RPCs (`FailedPrecondition`). Run this against a voter.

### watch

Python is stricter about TLS than Go: missing PEMs raise `ClusdrError` instead of bootstrap TLS.

```bash
pip install clusdr
python3 examples/watch/main.py
```

Another terminal, while it runs:

```bash
clusdr publish ping '{"from":"cli"}'
```

You should see `custom.ping` on the bus. Custom events are 1-hop gossip, not Raft. `member.left` is.

`--name` sets the lease this process holds (`worker.<name>`). `close()` revokes it. Two processes with the same `--name` — the second fails to grant.

```bash
python3 examples/watch/main.py --name edge-1
```

## From this tree vs your app

This module `replace`s `github.com/durguto/clusdr/sdk` → `./sdk`. `go run ./examples/…` always matches the checkout.

In your own module:

```bash
go get github.com/durguto/clusdr/sdk
pip install clusdr
```

Copy the files. Keep `clusdr.Local()` / `local()`. `Dial` / `dial` is for tests and operators.

Guide: [Use it from your app](../docs/guide/from-your-app.md).
