# `clusdr health`

Dials the local Runtime API and calls Health. Exit 0 if the RPC succeeds and `healthy` is true.

This is the process probe. It is **not** [`clusdr status`](status.md) (Unix socket file) and it is **not** Raft role.

## Synopsis

```bash
clusdr health
```

## Output

`healthy`, `node id`, `cluster id`, `role`. In this version `healthy` is always true and `role` is always `standalone`.

Error if the Runtime API is down.

## See also

- [HealthService](../api/health.md)
- [Run on other hosts](../../guide/other-hosts.md)
- [Run on Kubernetes](../../guide/kubernetes.md)
