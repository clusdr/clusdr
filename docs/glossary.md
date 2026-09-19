# Glossary

The words in the table below are used the same way in the docs, the CLI, and the SDK. A mismatch (status vs role, crash vs leave) is how operators invent a second `join` after a reboot.

This is a lookup table, not an introduction — start at [Get started](./) if you have not run a cluster yet.

| Term | Meaning |
|---|---|
| **Daemon** | The `clusdr start` process. The cluster member. The app is not this process. |
| **Application** | A program that calls the local daemon via the SDK. Not a Raft member, so it does not vote or store the log. |
| **Runtime API** | TCP gRPC listener (default `127.0.0.1:7947`). Apps, CLI, and peers. If this port is wrong, `members` and `Local()` fail even when `status` looks fine. |
| **Control API** | Unix socket. Only `clusdr status` uses it (file-exists check). Do not use it as a Kubernetes probe. |
| **Voter** | Raft member that votes. Default `join` role. Voters are the quorum. |
| **Observer** | Raft non-voter. Receives the log; does not change quorum. Locks are rejected until `promote`. |
| **Leader** | The current Raft leader. The only writer of replicated state. Followers forward mutations here. |
| **Follower** | A voter that is not leader. Forwards mutations to the leader. |
| **Member** | A daemon in the membership list (`id`, address, status, role). |
| **Status** | Liveness: `alive` or `dead`. Not the same as role. A left id is gone from the list. |
| **Role** | Raft suffrage: `voter` or `observer`. CLI also prints `leader` for the current leader. |
| **Join token** | One-time secret from `clusdr init`. Hash stored; plaintext shown once. A second `init` without `--force` does not print a new token. |
| **Presence** | Lease named `presence.<nodeID>`. Expiry marks `dead` (`member.dead`) and keeps the Raft server. Default TTL 3s. Leave is `clusdr leave` (`member.left`). |
| **Fencing token** | Monotonic token on a lock or lease grant. Stale holders cannot unlock a newer grant, so a delayed unlock cannot steal work. |
| **Watch** | Server stream of cluster and custom events. Not a poll. |
| **Custom event** | `custom.<topic>` from `Publish`. Gossip, not Raft — reconnect does not replay missed publishes. |
| **v1alpha1** | Current wire package name. Shape can still change; pin daemon and SDK to the same version train. |
| **Kubernetes** | A place to run the Linux host model ([Kubernetes](concepts/kubernetes.md)). Not a replacement for Lease, probes, EndpointSlice, or etcd. |
| **Helm** | DaemonSet chart `charts/clusdr`, published as `oci://ghcr.io/clusdr/charts/clusdr` ([Helm](guide/kubernetes-helm.md)). Catalog: [Artifact Hub](https://artifacthub.io/packages/helm/clusdr/clusdr). Not an Operator; does not join Raft. Not a `helm repo add`. |
| **ClusdrCluster** | CRD kind, group `clusdr.io`. Desired host topology. Status from `Members()`. Raft stays the member list. `spec.leave` is `clusdr leave`. Install: `https://clusdr.io/download/clusdr-crds.yaml`. |
| **Operator** (kube) | `clusdr-operator` ([Operator](guide/kubernetes-operator.md)): reconciles `ClusdrCluster` (DaemonSet or Sidecar STS) with `init` / `join`. `spec.leave` is `clusdr leave`. Crash / pod restart is not leave. Distinct from the human operator CLI. Image `durguto/clusdr-operator`. Install: `https://clusdr.io/download/clusdr-operator.yaml`. |

See also: [Architecture](architecture.md), [Membership](concepts/membership.md).
