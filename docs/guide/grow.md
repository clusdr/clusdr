# 3. Grow the cluster

You have one running seed from [step 2](first-member.md) and the join token from `init`. Now add another daemon.

A member is a **process**, not a laptop. Two daemons on one machine is a real cluster as long as each has its own `data.dir`, ports, socket, and `node.id`.

`node.addr` must be a host:port peers can dial. `0.0.0.0` is a bind address, not an advertisement.

## Why a second voter

The leader is the only writer of membership, locks, and leases. If the only voter dies, there is no quorum and no new leader.

| Voters | Majority | What you can lose |
|---|---|---|
| 1 | 1 | Nothing. The node dies, the cluster is gone |
| 2 | 2 | Nothing. One death loses majority |
| 3 | 2 | One voter |
| 5 | 3 | Two voters |

Laptop walkthrough uses two processes so you can see `join`. A cluster you care about wants **3 or 5 voters**.

## Start the joiner

Write `b.yaml` in a working directory. Do not reuse `~/.clusdr`.

```yaml
node:
  id: node-b
  addr: 127.0.0.1:8947
cluster:
  id: ""          # empty is fine; mismatch is rejected only when both sides set a value
data:
  dir: ./data-b
grpc:
  addr: 127.0.0.1:8947
  control_socket: ./clusdr-b.sock
raft:
  addr: 127.0.0.1:8946
```

Start it **without** `--bootstrap`:

```bash
clusdr start --config b.yaml
```

This process has identity on disk after you join. It is not in the Raft configuration yet.

## Join

Another terminal, same token you saved:

```bash
clusdr --config b.yaml join --token <token-from-init> 127.0.0.1:7947
```

`<addr>` is the seed's Runtime API, not Raft.

Then, against the seed (default ports):

```bash
clusdr members
```

Two `alive` rows. Join goes through the **leader**. If you had pointed `join` at a follower, that daemon forwards.

Wrong token → `UNAUTHORIZED`. Cluster ids set on both sides and different → rejected.

## Add a replica that does not vote

A 4th or 10th voter in another rack makes failover slower and that node's death count against majority. An **observer** receives the Raft log and serves a local app, but does not vote.

Same token. Quorum does not change.

```yaml
# obs.yaml — own dir and ports, like b.yaml
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
clusdr --config obs.yaml join --observer --token <token-from-init> 127.0.0.1:7947
clusdr members
```

Role column shows `observer`. That daemon **rejects locks** (`FailedPrecondition`). Watch, publish, and leases still work — presence must, or a dead observer would stay in the list.

Promote later if you want a vote:

```bash
clusdr promote node-obs
```

Empty `clusdr promote` promotes the local node. Already a voter → success. Unknown id → error. After promote, lock RPCs work on that daemon.

There is no demote in this version.

## What “alive” means

Each daemon holds a presence lease `presence.<nodeID>`. If it expires, the leader **removes that Raft server** and you see `member.left`. Heartbeats are the slower backup.

Default TTL is **3s** (fine for this laptop kill-and-watch). On real hosts that reboot, raise `lease.presence_ttl` and treat `join` as first add / after removal — not every boot. Full operator path: [presence](../concepts/presence.md).

Kill the observer: the voter list and quorum stay put. Kill a voter in a 3-node cluster: the other two elect.

## Next

Keep the daemons running. [Watch and publish →](watch.md)
