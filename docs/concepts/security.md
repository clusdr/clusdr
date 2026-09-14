# Security

TLS is **on** by default (`CLUSDR_TLS=enabled`). Set `CLUSDR_TLS=disabled` only for local development, on **every** node and client.

## Join token

`clusdr init` creates a cluster CA, a seed node cert, and a **one-time join token**. The hash is stored; plaintext is shown once.

Join without a valid token is `UNAUTHORIZED` when the cluster has a token hash.

## mTLS

Node-to-node gRPC uses certificates from that CA. Join bootstrap speaks TLS without a client cert, then installs the issued cert.

Cert files in `data.dir`: `ca.crt`, `node.crt`, `node.key`.

Apps on the same host: the SDK reads those PEMs from `CLUSDR_DATA_DIR`, `WithDataDir`, or `~/.clusdr`.

Wrong CA → connection rejected. Server identity is the node id (SAN), not the dial hostname; clients skip hostname and verify the CA.

`clusdr certs show` prints CA fingerprint and node cert fields.

## Related

- [Start the first member](../guide/first-member.md)
- [`clusdr init`](../reference/cli/init.md)
- [`clusdr certs`](../reference/cli/certs.md)
