# Security

clusdr encrypts peer and app traffic with mTLS so a host that can reach port 7947 cannot join or impersonate a member. TLS is **on** by default (`CLUSDR_TLS=enabled`). Set `CLUSDR_TLS=disabled` only for local development, and set it on **every** node and client — a mixed cluster fails handshake instead of falling back to plaintext.

## What to trust

| Piece | Trust it to |
|---|---|
| App + SDK on the same host | Talk to the local daemon only. They can read `data.dir` certs. |
| Daemon process | Be the Raft member. Hold cluster CA material. |
| Other members | Speak Raft and join/heartbeat with **mTLS** from that CA. |
| The operator of the machine | Everything on disk in `data.dir`. Anyone with that directory is that member. |

Do not trust the network without mTLS, a join that has no valid token, or a shared host if TLS is off.

## Join token

`clusdr init` creates a cluster CA, a seed node cert, and a **one-time join token**. The hash is stored; plaintext is shown once. A second `init` without `--force` does not print a new token — copy the first printout or you will get `UNAUTHORIZED` ([errors](../reference/errors.md#join)).

Join without a valid token is `UNAUTHORIZED` when the cluster has a token hash. Steal the one-time token and you join once; after that the hash still matches only that secret.

## mTLS

Node-to-node gRPC uses certificates from that CA. Join bootstrap speaks TLS without a client cert, then installs the issued cert so later RPCs are mutual.

Cert files in `data.dir`: `ca.crt`, `node.crt`, `node.key`.

Apps on the same host: every official SDK locates those PEMs the same way — constructor override (`WithDataDir` / `data_dir` / `dataDir`), else `CLUSDR_DATA_DIR`, else `~/.clusdr`. Callers do not pass PEM bytes. All SDKs locate TLS material the same way by default — see [SDKs](../sdk/).

If the three files are present, every language presents the node cert and trusts the cluster CA. Wrong CA → connection rejected. Server identity is the node id (SAN), not the dial hostname.

If the files are **missing**, every official SDK fails. Point the data dir at the daemon’s `data.dir`, or set `CLUSDR_TLS=disabled` / `insecure` (dev only) ([errors](../reference/errors.md#tls)). Set `CLUSDR_TLS_SERVER_NAME` only when the CN on `node.crt` is missing; the value is the **peer node id**. Go does not read that variable — it verifies the cluster CA without SNI.

`clusdr certs show` prints CA fingerprint and node cert fields. Compare fingerprints across nodes after join.

## What we worry about

- **Unauthorized join** — token hash + TLS. Steal the one-time token and you join once.
- **Impersonation** — node certs from the cluster CA. A client without that CA does not get in.
- **Lost disk** — `data.dir` is the member. Anyone with that directory is that member.
- **`CLUSDR_TLS=disabled`** — plaintext. Only a closed laptop loop.

How to report a hole: [SECURITY.md](https://github.com/clusdr/clusdr/blob/main/SECURITY.md). How a release is built and checked: [Install](../guide/install.md).

## Related

- [Start the first member](../guide/first-member.md)
- [`clusdr init`](../reference/cli/init.md)
- [`clusdr certs`](../reference/cli/certs.md)
