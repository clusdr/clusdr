# `clusdr status`

`clusdr status` checks whether the Unix control socket **file** exists. Use it in a laptop script that only needs “did this process bind the socket.” It is **not** the Health RPC, not Raft role, and not a Kubernetes probe.

If `grpc.control_socket` exists: `daemon running` (exit 0). If not: `daemon not running` (exit 1). A running daemon that failed to bind the socket looks “down”; a process that bound the socket but never served gRPC looks “up.” Prefer [`clusdr members`](members.md) to see if the Runtime API is up, or [`clusdr health`](health.md) for the Runtime probe.

## Synopsis

```bash
clusdr status
clusdr status --config /etc/clusdr/clusdr.yaml
```

Default socket is `$HOME/.clusdr/clusdr.sock` (or `/var/lib/clusdr/clusdr.sock` if `HOME` is unset). Override with `CLUSDR_CONTROL_SOCKET` or `grpc.control_socket` in the same YAML the daemon uses. Pass `--config` so the path matches; a second process has its own socket.

## Errors

| You see | What to do |
|---|---|
| `daemon not running (socket not found: …)` | Start the daemon, or pass `--config` so the socket path matches. Prefer `clusdr members` ([errors](../errors.md#daemon-and-dial)) |

## See also

- [Run on other hosts](../../guide/other-hosts.md) (health checks)
- [HealthService](../api/health.md)
- [`clusdr health`](health.md)
