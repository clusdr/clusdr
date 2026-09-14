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
| **Status** | Liveness: `alive`, `leaving`, or `dead`. Not the same as role. |
| **Role** | Raft suffrage: `voter` or `observer`. CLI also prints `leader` for the current leader. |
| **Join token** | One-time secret from `clusdr init`. Hash stored; plaintext shown once. |
| **Presence** | Lease named `presence.<nodeID>`. Expiry removes the member. |
| **Fencing token** | Monotonic token on a lock or lease grant. Stale holders cannot unlock a newer grant. |
| **Watch** | Server stream of cluster and custom events. |
| **Custom event** | `custom.<topic>` from `Publish`. Gossip, not Raft. |
| **v1alpha1** | Current wire package name. Shape can still change. |

See also: [Architecture](architecture.md), [Membership](concepts/membership.md).
