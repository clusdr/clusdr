# `clusdr status`

If `grpc.control_socket` exists: `daemon running` (exit 0). If not: `daemon not running` (exit 1).

This is **not** the Health RPC. A running daemon that failed to bind the socket looks “down”. Use [`clusdr health`](health.md) for the Runtime probe.

## Synopsis

```bash
clusdr status
```

Default socket is `$HOME/.clusdr/clusdr.sock` (or `/var/lib/clusdr/clusdr.sock` if `HOME` is unset). Override with `CLUSDR_CONTROL_SOCKET`.

## See also

- [Run on other hosts](../../guide/other-hosts.md) (health checks)
- [HealthService](../api/health.md)
- [`clusdr health`](health.md)
