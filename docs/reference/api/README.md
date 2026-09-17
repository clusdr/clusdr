# gRPC API

Package `clusdr.v1alpha1`. Applications should use the [SDKs](../../sdk/). This section is the wire contract.

| Module | Tree | Audience |
|---|---|---|
| [`buf.build/clusdr/api`](https://buf.build/clusdr/api) | `proto/api/` | Apps and language SDKs |
| [`buf.build/clusdr/internal`](https://buf.build/clusdr/internal) | `proto/internal/` | Join, heartbeat, local ControlService — not exported to SDKs |

Pull requests lint and reject FILE-incompatible changes (field delete/renumber/type). Generated Go stubs: module `github.com/clusdr/clusdr/api`.

Message names follow Buf STANDARD: `{Method}Request` when the method is unique in the package (`GrantRequest`, `LockRequest`). Two services that share a method use `{Service}{Method}Request` (`LockServiceRenewRequest`, `LeaseServiceRenewRequest`). `TryLock` has its own types (same fields as `Lock`). gRPC method paths and protobuf field numbers are the wire identity.

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
