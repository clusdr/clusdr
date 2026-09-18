# Security

TLS is **on** by default (`CLUSDR_TLS=enabled`). Set `CLUSDR_TLS=disabled` only for local development, on **every** node and client.

## What to trust

| Piece | Trust it to |
|---|---|
| App + SDK on the same host | Talk to the local daemon only. They can read `data.dir` certs. |
| Daemon process | Be the Raft member. Hold cluster CA material. |
| Other members | Speak Raft and join/heartbeat with **mTLS** from that CA. |
| The operator of the machine | Everything on disk in `data.dir`. |

Do not trust the network without mTLS, a join that has no valid token, or a shared host if TLS is off.

## Join token

`clusdr init` creates a cluster CA, a seed node cert, and a **one-time join token**. The hash is stored; plaintext is shown once.

Join without a valid token is `UNAUTHORIZED` when the cluster has a token hash.

## mTLS

Node-to-node gRPC uses certificates from that CA. Join bootstrap speaks TLS without a client cert, then installs the issued cert.

Cert files in `data.dir`: `ca.crt`, `node.crt`, `node.key`.

Apps on the same host: the SDK reads those PEMs from `CLUSDR_DATA_DIR`, `WithDataDir`, or `~/.clusdr`.

Wrong CA → connection rejected. Server identity is the node id (SAN), not the dial hostname; clients skip hostname and verify the CA.

`clusdr certs show` prints CA fingerprint and node cert fields.

## What we worry about

- **Unauthorized join** — token hash + TLS. Steal the one-time token and you join once.
- **Impersonation** — node certs from the cluster CA. A client without that CA does not get in.
- **Lost disk** — `data.dir` is the member. Anyone with that directory is that member.
- **`CLUSDR_TLS=disabled`** — plaintext. Only a closed laptop loop.

How to report a hole: [SECURITY.md](../../SECURITY.md). How a release is built and checked: [Install](../guide/install.md).

## Related

- [Start the first member](../guide/first-member.md)
- [`clusdr init`](../reference/cli/init.md)
- [`clusdr certs`](../reference/cli/certs.md)
