# `clusdr health`

`clusdr health` dials the local Runtime API and calls Health. Use it as a process probe: gRPC is serving and `healthy` is true (exit 0). It is **not** [`clusdr status`](status.md) (Unix socket file) and it is **not** Raft role. In this version `healthy` is always true and `role` is always `standalone` — use [`clusdr members`](members.md) for who is in the cluster.

## Synopsis

```bash
clusdr health
clusdr health --config /etc/clusdr/clusdr.yaml
```

The CLI dials `grpc.addr` (default `127.0.0.1:7947`), the same Runtime address apps use. Pass the same `--config` as the daemon.

## Output

`healthy`, `node id`, `cluster id`, `role`. In this version `healthy` is always true and `role` is always `standalone`.

## Errors

| You see | What to do |
|---|---|
| `dial daemon at …` / `connection refused` | Runtime is down or `grpc.addr` does not match. Start the daemon; point `--config` at the same YAML ([errors](../errors.md#daemon-and-dial)) |
| Handshake / certificate errors | TLS mismatch. Same `tls.mode` on CLI and daemon ([errors](../errors.md#tls)) |

## See also

- [HealthService](../api/health.md)
- [Run on other hosts](../../guide/other-hosts.md)
- [Run on Kubernetes](../../guide/kubernetes.md)
