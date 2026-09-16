# Limits

Product boundaries of this version. Not a bug list. Same story as [Overview](../overview.md).

## Scope

- Not a database, queue, workflow engine, mesh, or Kubernetes
- Custom events are ephemeral (1-hop gossip). They are not Raft-replicated and are not replayed on Watch reconnect. Bus is bounded; slow subscribers drop events
- No federation, no OpenTelemetry export
- Observer (non-voter) nodes exist; they do not change quorum

## API and process

- Wire API is **v1alpha1**. No compatibility promise of a v1 tag
- `Health.role` is always `standalone`. `Health.healthy` is always `true`
- `clusdr status` = control socket exists. Default path `$HOME/.clusdr/clusdr.sock`
- `CLUSDR_SHUTDOWN_TIMEOUT` is not implemented. Only YAML `shutdown_timeout` (if > 0) changes the 15s close grace

## Numeric caps

| Cap | Value |
|---|---|
| Custom event payload | 64 KiB |
| Topic / lock / lease name | 1–128 characters (`A–Z a–z 0–9 . _ -`) |
| Locks in the table | 4096 |
| Leases in the table | 4096 |
| Lock/lease TTL | 1ms–24h (default 15s) |
| Watch client buffer (SDK) | 64 events |

## Cluster

- Raft and advertised `node.addr` default to localhost. Multi-host needs explicit addresses
- Default Raft heartbeat and election are 150ms. If peer RTT is larger, raise them or you get extra elections
- `docker compose` is a single node without `init`
- Two processes must not share one `data.dir`
- Presence expiry (and heartbeat misses) **remove** the Raft server. There is no `disconnect` event and no auto-rejoin. Default TTL is 3s; raise it for reboots ([presence](../concepts/presence.md))

## Distribution

- Release binaries embed the tag via ldflags. Source builds print `dev`
- Install channels: Linux install script, Docker Hub `durguto/clusdr` (GHCR mirror `ghcr.io/clusdr/clusdr`)
- Python SDK: `pip install clusdr`
- Apache-2.0 ([LICENSE](https://github.com/clusdr/clusdr/blob/main/LICENSE))
