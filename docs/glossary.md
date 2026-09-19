# Glossary

Terms used in these docs, the CLI, and the SDK.

| Term | Meaning |
|---|---|
| **Daemon** | The `clusdr start` process. The cluster member. |
| **Application** | A program that calls the local daemon via the SDK. Not a Raft member. |
| **Runtime API** | TCP gRPC listener (default `127.0.0.1:7947`). Apps, CLI, and peers. |
| **Control API** | Unix socket. Only `clusdr status` uses it (file-exists check). |
| **Voter** | Raft member that votes. Default join role. |
| **Observer** | Raft non-voter. Receives the log; does not change quorum. |
| **Leader** | The current Raft leader. The only writer of replicated state. |
| **Follower** | A voter that is not leader. Forwards mutations to the leader. |
| **Member** | A daemon in the membership list (`id`, address, status, role). |
| **Status** | Liveness: `alive` or `dead`. Not the same as role. A left id is gone from the list. |
| **Role** | Raft suffrage: `voter` or `observer`. CLI also prints `leader` for the current leader. |
| **Join token** | One-time secret from `clusdr init`. Hash stored; plaintext shown once. |
| **Presence** | Lease named `presence.<nodeID>`. Expiry marks `dead` (`member.dead`) and keeps the Raft server. Default TTL 3s. Leave is `clusdr leave` (`member.left`). |
| **Fencing token** | Monotonic token on a lock or lease grant. Stale holders cannot unlock a newer grant. |
| **Watch** | Server stream of cluster and custom events. |
| **Custom event** | `custom.<topic>` from `Publish`. Gossip, not Raft. |
| **v1alpha1** | Current wire package name. Shape can still change. |
| **Kubernetes** | A place to run the Linux host model ([Kubernetes](concepts/kubernetes.md)). Not a replacement for Lease, probes, EndpointSlice, or etcd. |
| **Helm** | DaemonSet chart `charts/clusdr`, published as `oci://ghcr.io/clusdr/charts/clusdr` ([Helm](guide/kubernetes-helm.md)). Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr). Not an Operator; does not join Raft. Not a `helm repo add`. |
| **ClusdrCluster** | CRD kind, group `clusdr.io`. Desired host topology. Status from `Members()`. Raft stays the member list. `spec.leave` is `clusdr leave`. Install: `https://clusdr.io/download/clusdr-crds.yaml`. |
| **Operator** (kube) | `clusdr-operator` ([Operator](guide/kubernetes-operator.md)): reconciles `ClusdrCluster` (DaemonSet or Sidecar STS) with `init` / `join`. `spec.leave` is `clusdr leave`. Crash / pod restart is not leave. Distinct from the human operator CLI. Image `durguto/clusdr-operator`. Install: `https://clusdr.io/download/clusdr-operator.yaml`. |

See also: [Architecture](architecture.md), [Membership](concepts/membership.md).
