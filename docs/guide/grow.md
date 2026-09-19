# Grow the cluster

A second voter and an observer are extra **processes**, not extra laptops. Each daemon needs its own `data.dir`, ports, socket, and `node.id`. Reusing `~/.clusdr` for the second process fails the BoltDB flock. `node.addr` must be a host:port the other process can dial — `0.0.0.0` is a bind address, not an advertisement, and join will not be able to call back.

Keep the seed `clusdr start --bootstrap` from [step 2](first-member.md) running, and keep the join token `clusdr init` printed once (the `join token :` line). If that terminal is gone and you did not copy the token, you cannot recover it from disk — only the hash is stored. Start over with a fresh `data.dir` or `clusdr init --force` on a cluster that has no peers yet ([Errors](../reference/errors.md)).

A cluster you keep wants **3 or 5 voters** so one death does not lose majority. This laptop walkthrough uses two voter processes so you can see `join`. Quorum rules: [Observers](../concepts/observers.md).

## 1. Start the joiner

Write `b.yaml` in a working directory. Do not reuse `~/.clusdr`.

```yaml
node:
  id: node-b
  addr: 127.0.0.1:8947
cluster:
  id: ""
data:
  dir: ./data-b
grpc:
  addr: 127.0.0.1:8947
  control_socket: ./clusdr-b.sock
raft:
  addr: 127.0.0.1:8946
```

Empty `cluster.id` is fine. A mismatch is rejected only when **both** sides set a value.

```bash
clusdr start --config b.yaml
```

No `--bootstrap`. This process has identity on disk after you join. It is not in the Raft configuration yet.

## 2. Join

Use the token from step 2’s `init`. The address is the seed’s **Runtime API** (`127.0.0.1:7947`), not Raft (`7946`).

```bash
TOKEN='<paste the join token from init>'
clusdr --config b.yaml join --token "$TOKEN" 127.0.0.1:7947
```

Then, against the seed (default ports, no `--config`):

```bash
clusdr members
```

Two `alive` rows. Join goes through the **leader**. If you point `join` at a follower, that daemon forwards.

Wrong token → `UNAUTHORIZED` — you pasted a different string than `init` printed, or you ran `--force` after a peer joined ([Errors](../reference/errors.md#join)). Cluster ids set on both sides and different → rejected; leave `cluster.id` empty on the joiner unless you copied the seed’s id on purpose.

## 3. Add an observer

A fourth voter in another rack makes failover slower and that node’s death count against majority. An **observer** receives the Raft log and serves a local app, but does not vote.

```yaml
# obs.yaml
node:
  id: node-obs
  addr: 127.0.0.1:9947
data:
  dir: ./data-obs
grpc:
  addr: 127.0.0.1:9947
  control_socket: ./clusdr-obs.sock
raft:
  addr: 127.0.0.1:9946
```

```bash
clusdr start --config obs.yaml
clusdr --config obs.yaml join --observer --token "$TOKEN" 127.0.0.1:7947
clusdr members
```

Role column shows `observer`. That daemon **rejects locks** (`FailedPrecondition`) until you promote it — Watch, publish, and leases still work so a dead observer can be marked dead.

```bash
clusdr promote node-obs
```

Empty `clusdr promote` promotes the local node. Already a voter → success. Unknown id → error. There is no demote in this version.

## Checkpoint

Three rows: two voters (one `leader`) and one `observer`. Keep the daemons running.

Next: [Watch and publish →](watch.md)

Crash vs `leave`: [Presence](../concepts/presence.md).
