# Grow the cluster

You have one running seed from [step 2](first-member.md) and the join token from `init`. This page adds a second voter and an observer on the same machine.

A member is a **process**. Each daemon needs its own `data.dir`, ports, socket, and `node.id`. `node.addr` must be a host:port peers can dial — not `0.0.0.0`.

A cluster you keep wants **3 or 5 voters**. This laptop walkthrough uses two processes so you can see `join`. Quorum: [Observers](../concepts/observers.md).

## 1. Start the joiner

Write `b.yaml`. Do not reuse `~/.clusdr`.

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

```bash
clusdr start --config b.yaml
```

No `--bootstrap`.

## 2. Join

```bash
clusdr --config b.yaml join --token <token-from-init> 127.0.0.1:7947
```

`<addr>` is the seed's Runtime API, not Raft.

```bash
clusdr members
```

Two `alive` rows. Wrong token → `UNAUTHORIZED`.

## 3. Add an observer

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
clusdr --config obs.yaml join --observer --token <token-from-init> 127.0.0.1:7947
clusdr members
```

Role column shows `observer`. Promote later: `clusdr promote node-obs`. There is no demote in this version.

## Checkpoint

Three rows: two voters (one `leader`) and one `observer`. Keep the daemons running.

Next: [Watch and publish →](watch.md)

Crash vs `leave`: [Presence](../concepts/presence.md).
