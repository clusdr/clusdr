# gRPC API

Package `clusdr.v1alpha1`. Application sources: `proto/api/` ([`buf.build/clusdr/api`](https://buf.build/clusdr/api)). Join/heartbeat: `proto/internal/` ([`buf.build/clusdr/internal`](https://buf.build/clusdr/internal)). Pull requests lint and reject wire-incompatible changes. Generated Go stubs: module `github.com/clusdr/clusdr/api`.

Applications should use the [SDKs](../../sdk/). This section is the wire contract.

All application RPCs go to the **Runtime** TCP server. Node-to-node uses the same server. There is no HTTP/JSON API.

## Services

| Service | Audience | Page |
|---|---|---|
| HealthService | Probes, SDK ready check | [health](health.md) |
| MembershipService | Apps, CLI | [membership](membership.md) |
| WatchService | Apps, CLI | [watch](watch.md) |
| EventService | Apps, CLI, peer relay | [events](events.md) |
| LockService | Apps | [locks](locks.md) |
| LeaseService | Apps, presence | [leases](leases.md) |
| ControlService | CLI → local daemon | [control](control.md) |
| JoinService | Daemon → daemon | [join](join.md) |
| HeartbeatService | Daemon → daemon | [heartbeat](heartbeat.md) |

## grpcurl (TLS off)

```bash
CLUSDR_TLS=disabled clusdr start --bootstrap   # dev only
grpcurl -plaintext 127.0.0.1:7947 list
grpcurl -plaintext 127.0.0.1:7947 clusdr.v1alpha1.MembershipService/ListMembers
```

With TLS, use the PEMs in `data.dir`. Server identity is the node id (SAN), not the dial hostname.
