# HealthService

HealthService answers whether this daemon’s Runtime API is serving. Use it for process probes and SDK ready checks — not for Raft role and not as a Kubernetes substitute for `clusdr members`. Application module [`buf.build/clusdr/api`](https://buf.build/clusdr/api).

The CLI entry is [`clusdr health`](../../reference/cli/health.md), which dials `grpc.addr` (default `127.0.0.1:7947`).

## `Health`

**Signature:** `Health(HealthRequest) returns (HealthResponse)`

**Response:** `node_id`, `cluster_id`, `role`, `healthy`.

In this version `healthy` is always `true` and `role` is always `standalone` (set once at process start). This is **not** Raft role. If you treat `role` as leader/follower, you will misread every node. Use [`ListMembers`](membership.md) / [`clusdr members`](../../reference/cli/members.md).

Dial failure or a TLS mismatch is a transport error, not a `healthy = false` response ([errors](../errors.md#daemon-and-dial), [errors](../errors.md#tls)).

## See also

- [`clusdr health`](../../reference/cli/health.md)
- [Run on other hosts](../../guide/other-hosts.md) (health checks)
