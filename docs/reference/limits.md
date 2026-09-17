# Limits

Product boundaries of this version. Not a bug list. Same story as [Overview](../overview.md).

## Scope

- Not a database, queue, workflow engine, mesh, or Kubernetes replacement. It can run on Kubernetes; it does not replace kube coordination. Helm chart, `ClusdrCluster` CRD, and `clusdr-operator` are in-tree ([Helm](../guide/kubernetes-helm.md), [Operator](../guide/kubernetes-operator.md))
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
- Presence expiry (and heartbeat misses) mark a member **not-alive**. They do not remove the Raft server. `clusdr leave` is the only `RemoveServer`. Restart with the same `data.dir` is `clusdr start` ([presence](../concepts/presence.md))

## Distribution

- Release binaries embed the tag via ldflags. Source builds print `dev`
- Install channels: Linux install script, Docker Hub `durguto/clusdr` (GHCR mirror `ghcr.io/clusdr/clusdr`), operator image `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`), Helm `oci://ghcr.io/clusdr/charts/clusdr` ([Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr)), operator YAML `https://clusdr.io/download/clusdr-crds.yaml` + `clusdr-operator.yaml`
- Python SDK: `pip install clusdr`
- Rust SDK: crate `clusdr` ([github.com/clusdr/clusdr-rust](https://github.com/clusdr/clusdr-rust))
- TypeScript SDK: `npm install clusdr` ([github.com/clusdr/clusdr-js](https://github.com/clusdr/clusdr-js))
- Java SDK: `io.clusdr:clusdr` ([github.com/clusdr/clusdr-java](https://github.com/clusdr/clusdr-java))
- Apache-2.0 ([LICENSE](https://github.com/clusdr/clusdr/blob/main/LICENSE))
