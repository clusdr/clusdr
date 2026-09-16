# Examples

Small **applications** that talk to a local daemon. They do not join Raft, do not store data, and do not dial a remote Runtime API.

Each program is the same story in **Go**, **Python**, **Rust**, **TypeScript**, and **Java**. Languages live in their own package under the example folder (`go/`, `python/`, `rust/`, `typescript/`, `java/`).

```text
your process  ──►  clusdr daemon on this host  ──►  the rest of the cluster
```

## Prerequisite

```bash
clusdr init
clusdr start --bootstrap
```

Leave that process running. TLS is on; the SDK loads `ca.crt` / `node.crt` / `node.key` from `CLUSDR_DATA_DIR` or `~/.clusdr`. If connect fails, [Errors](../docs/reference/errors.md).

Python, Rust, TypeScript, and Java require those PEMs (or `CLUSDR_TLS=disabled`). Go falls back to bootstrap TLS if the data dir is empty.

Rust packages depend on a sibling [`clusdr-rust`](https://github.com/clusdr/clusdr-rust) checkout (`../clusdr-rust` next to this repo). TypeScript packages depend on sibling [`clusdr-js`](https://github.com/clusdr/clusdr-js) (`../clusdr-js`). Java examples depend on `mvn install` of sibling [`clusdr-java`](https://github.com/clusdr/clusdr-java) (`../clusdr-java`).

## Programs

| Path | Story |
|---|---|
| [who](who) | Inventory: who is alive, who leads, then live `member.*` / `leader.changed` |
| [scheduler](scheduler) | One exclusive scheduler. Run two copies; only one holds the lock |
| [watch](watch) | Snapshot, a worker lease, a `custom.hello`, then the full Watch bus |
| [worker](worker) | Hold a shard lease. A second copy fails immediately (no wait) |
| [agent](agent) | `custom.agent.task` pub/sub — signals, not a queue |

### who

Prints the membership table, then stays on Watch until Ctrl-C. Cluster events are labeled; `custom.*` is gossip and is not replayed after a reconnect.

```bash
go run ./examples/who/go
python3 examples/who/python/main.py
cargo run -p who --manifest-path examples/Cargo.toml
npx tsx examples/who/typescript/main.ts
mvn -q -f examples/who/java/pom.xml exec:java
```

Snapshot only:

```bash
go run ./examples/who/go -once
python3 examples/who/python/main.py --once
cargo run -p who --manifest-path examples/Cargo.toml -- --once
npx tsx examples/who/typescript/main.ts --once
mvn -q -f examples/who/java/pom.xml exec:java -Dexec.args=--once
```

### scheduler

Loop: try to acquire `scheduler`, do a short “dispatch” while holding the fencing token, release, repeat. Background renew keeps the grant alive during work.

Terminal A and B, same lock name:

```bash
go run ./examples/scheduler/go -holder replica-a
go run ./examples/scheduler/go -holder replica-b

python3 examples/scheduler/python/main.py --holder replica-a
python3 examples/scheduler/python/main.py --holder replica-b

cargo run -p scheduler --manifest-path examples/Cargo.toml -- --holder replica-a
cargo run -p scheduler --manifest-path examples/Cargo.toml -- --holder replica-b

npx tsx examples/scheduler/typescript/main.ts --holder replica-a
npx tsx examples/scheduler/typescript/main.ts --holder replica-b

mvn -q -f examples/scheduler/java/pom.xml exec:java -Dexec.args="--holder replica-a"
mvn -q -f examples/scheduler/java/pom.xml exec:java -Dexec.args="--holder replica-b"
```

One replica prints `held … token=…` and dispatches. The other prints `waiting`. Kill the holder; the waiter acquires. That is the product: exclusive work without the app joining the cluster.

Go `TryLock` may include the current holder when the name is taken. Python, Rust, TypeScript, and Java return `None` / `Ok(None)` / `null` / `Optional.empty()` with no holder object.

Observer daemons reject lock RPCs (`FailedPrecondition`). Run this against a voter.

### watch

```bash
go run ./examples/watch/go
python3 examples/watch/python/main.py
cargo run -p watch --manifest-path examples/Cargo.toml
npx tsx examples/watch/typescript/main.ts
mvn -q -f examples/watch/java/pom.xml exec:java
```

Another terminal, while it runs:

```bash
clusdr publish ping '{"from":"cli"}'
```

You should see `custom.ping` on the bus. Custom events are 1-hop gossip, not Raft. `member.left` is.

`--name` / `-name` sets the lease this process holds (`worker.<name>`). Close revokes it. Two processes with the same name — the second fails to grant.

```bash
go run ./examples/watch/go -name edge-1
python3 examples/watch/python/main.py --name edge-1
cargo run -p watch --manifest-path examples/Cargo.toml -- --name edge-1
npx tsx examples/watch/typescript/main.ts --name edge-1
mvn -q -f examples/watch/java/pom.xml exec:java -Dexec.args="--name edge-1"
```

### worker

Lease, not lock: Grant never blocks. Two processes, same name — the first holds `shard-7`, the second exits with `clusdr: lease "shard-7": … (owner worker-a)`.

```bash
go run ./examples/worker/go -name shard-7 -owner worker-a
go run ./examples/worker/go -name shard-7 -owner worker-b

python3 examples/worker/python/main.py --name shard-7 --owner worker-a
python3 examples/worker/python/main.py --name shard-7 --owner worker-b

cargo run -p worker --manifest-path examples/Cargo.toml -- --name shard-7 --owner worker-a
cargo run -p worker --manifest-path examples/Cargo.toml -- --name shard-7 --owner worker-b

npx tsx examples/worker/typescript/main.ts --name shard-7 --owner worker-a
npx tsx examples/worker/typescript/main.ts --name shard-7 --owner worker-b

mvn -q -f examples/worker/java/pom.xml exec:java -Dexec.args="--name shard-7 --owner worker-a"
mvn -q -f examples/worker/java/pom.xml exec:java -Dexec.args="--name shard-7 --owner worker-b"
```

Ctrl-C closes the client and **revokes** the lease. That is different from cancelling the Go lease context (or Python `stop_renew` / Rust `stop_renew` / TypeScript `stopRenew` / Java `stopRenew`), which only stops renew so the grant expires at its deadline.

### agent

Gossip between app processes through the local daemon. Not durable. The listener uses a topic filter so the membership snapshot is omitted. Start a listener, then an emitter:

```bash
go run ./examples/agent/go -mode listen
go run ./examples/agent/go -mode emit -from mapper

python3 examples/agent/python/main.py --mode listen
python3 examples/agent/python/main.py --mode emit --from mapper

cargo run -p agent --manifest-path examples/Cargo.toml -- --mode listen
cargo run -p agent --manifest-path examples/Cargo.toml -- --mode emit --from mapper

npx tsx examples/agent/typescript/main.ts --mode listen
npx tsx examples/agent/typescript/main.ts --mode emit --from mapper

mvn -q -f examples/agent/java/pom.xml exec:java -Dexec.args="--mode listen"
mvn -q -f examples/agent/java/pom.xml exec:java -Dexec.args="--mode emit --from mapper"
```

`--mode both` (default) emits and listens in one process so you can see round-trip on a single terminal.

## From this tree vs your app

This module `replace`s `github.com/clusdr/clusdr/sdk` → `./sdk`. `go run ./examples/…/go` always matches the checkout.

Python: `pip install clusdr` (or an editable `clusdr-python` checkout).

Rust: the `rust/` directory under each example is its own crate. `examples/Cargo.toml` is the workspace. It path-depends on sibling `clusdr-rust`.

TypeScript: `examples/package.json` path-depends on sibling `clusdr-js`. From `examples/`:

```bash
npm install
npx tsx who/typescript/main.ts
```

From the daemon repo root, `npx tsx examples/who/typescript/main.ts` works after that install.

Java: `mvn install` in sibling `clusdr-java`, then `mvn -q -f examples/who/java/pom.xml exec:java`.

In your own module:

```bash
go get github.com/clusdr/clusdr/sdk
pip install clusdr
npm install clusdr
```

```toml
clusdr = "0.1.4"
```

```xml
<dependency>
  <groupId>io.clusdr</groupId>
  <artifactId>clusdr</artifactId>
  <version>0.1.4</version>
</dependency>
```

Copy the files. Keep `Local` / `local()` / `Clusdr.local()`. `Dial` / `dial` is for tests and operators.

Guide: [Use it from your app](../docs/guide/from-your-app.md).
