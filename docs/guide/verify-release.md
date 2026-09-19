# Verify a release

Confirm a GitHub Release or an OCI image/chart was built by `clusdr/clusdr` on GitHub Actions.

You already have the files or can pull the tag. Cosign identity:

- certificate identity matches `https://github.com/clusdr/clusdr/`
- OIDC issuer is `https://token.actions.githubusercontent.com`

`v0.2.0` and earlier are checksum-only. Signed blobs, image signatures, and a signed chart start at the first release cut after signing landed.

## Checksums

Download `checksums.txt`, `checksums.txt.sig`, and `checksums.txt.pem` from the [GitHub Release](https://github.com/clusdr/clusdr/releases).

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

`install.sh` already checks SHA-256 against `checksums.txt`. This step checks the signature on that file.

## Images

```bash
cosign verify ghcr.io/clusdr/clusdr:vX.Y.Z \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Same identity for `ghcr.io/clusdr/clusdr-operator`.

## Helm chart

```bash
cosign verify ghcr.io/clusdr/charts/clusdr:X.Y.Z \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The chart tag is the version without `v`.
