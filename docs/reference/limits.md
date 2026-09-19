# Limits

Product boundaries of this version are not defects. Raising a payload or a table size here is a version change, not a config knob. Same story as [Overview](../overview.md).

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

Over a name or table cap, the RPC fails — [errors](errors.md#locks-and-leases), [errors](errors.md#applications).

| Cap | Value | Why it matters |
|---|---|---|
| Custom event payload | 64 KiB | Larger publishes are rejected; they are not stored on Raft. |
| Topic / lock / lease name | 1–128 characters (`A–Z a–z 0–9 . _ -`) | Illegal names fail before a grant or publish. |
| Locks in the table | 4096 | The 4097th lock fails; release unused names. |
| Leases in the table | 4096 | Same cap as locks; presence leases count. |
| Lock/lease TTL | 1ms–24h (default 15s) | After TTL the name expires (`lock.expired` / `lease.expired`) so a dead holder does not keep it. |
| Watch client buffer (SDK) | 64 events | A slow receiver blocks the client read loop; the daemon bus still drops. |

## Cluster

- Raft and advertised `node.addr` default to localhost. Multi-host needs explicit addresses
- Default Raft heartbeat and election are 150ms. If peer RTT is larger, raise them or you get extra elections
- `docker compose` is a single node without `init`
- Two processes must not share one `data.dir`
- Presence expiry (and heartbeat misses) mark a member **not-alive**. They do not remove the Raft server. `clusdr leave` is the only `RemoveServer`. Restart with the same `data.dir` is `clusdr start` ([presence](../concepts/presence.md))

## Distribution

| Channel | Value | Why it matters |
|---|---|---|
| Release binary | tag via ldflags | Source builds print `dev` and are off the [compatibility](compatibility.md) train. |
| Linux install | `https://clusdr.io/install.sh` | amd64/arm64 only. |
| Daemon image | Docker Hub `durguto/clusdr` (GHCR `ghcr.io/clusdr/clusdr`) | Same tag as the release. |
| Operator image | `durguto/clusdr-operator` (GHCR `ghcr.io/clusdr/clusdr-operator`) | Not baked into the daemon image. |
| Helm | `oci://ghcr.io/clusdr/charts/clusdr` | No `helm repo add`. Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr). |
| Operator YAML | `https://clusdr.io/download/clusdr-crds.yaml` + `clusdr-operator.yaml` | CRD is not in the Helm chart. |
| Python / Rust / TS / Java | `pip` / crate / `npm` / `io.clusdr:clusdr` | Same version train as the daemon. |
| License | Apache-2.0 | [LICENSE](https://github.com/clusdr/clusdr/blob/main/LICENSE) |
