# HealthService

## `Health`

**Signature:** `Health(HealthRequest) returns (HealthResponse)`

**Response:** `node_id`, `cluster_id`, `role`, `healthy`.

In this version `healthy` is always `true` and `role` is always `standalone` (set once at process start). This is **not** Raft role.

## See also

- [Run on other hosts](../../guide/other-hosts.md) (health checks)
