# Verify a release

A GitHub Release artifact or an OCI image/chart is signed by the `clusdr/clusdr` GitHub Actions workflow, not by a laptop. Cosign checks that identity so you do not run a binary someone rebuilt locally.

You need [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/) on `PATH`.

`v0.2.0` and earlier are checksum-only. Signed blobs, image signatures, and a signed chart exist only on tags cut after signing landed. If `cosign verify` says there is no signature, you are on an older tag — use SHA-256 against `checksums.txt` or pick a newer release.

Identity that must match:

- certificate identity starts with `https://github.com/clusdr/clusdr/`
- OIDC issuer is `https://token.actions.githubusercontent.com`

A verify that succeeds against another repo’s identity is the wrong check.

## Checksums

From the [GitHub Release](https://github.com/clusdr/clusdr/releases) for that tag, download `checksums.txt` and `checksums.txt.sigstore.json` into the current directory.

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

`v0.2.1` used detached `checksums.txt.sig` + `checksums.txt.pem` instead of a bundle:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

`install.sh` already checks SHA-256 against `checksums.txt`. This step checks that the checksum file itself was signed by the release workflow. If the blob verify fails, do not trust `sha256sum` against that file.

## Images

Replace `vX.Y.Z` with the tag you pulled (example: `v0.2.1`).

```bash
cosign verify ghcr.io/clusdr/clusdr:v0.2.1 \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Same identity for `ghcr.io/clusdr/clusdr-operator`. Docker Hub tags are the same image; Cosign signatures live on GHCR. Tags after `v0.2.1` store the signature as an OCI referrer (no `sha256-*.sig` tag). `v0.2.1` used the legacy tag.

## Helm chart

The chart tag is the version **without** `v` (example: `0.2.1`).

```bash
cosign verify ghcr.io/clusdr/charts/clusdr:0.2.1 \
  --certificate-identity-regexp '^https://github.com/clusdr/clusdr/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

If verify says the tag has no signature, the chart was published before Cosign signing — install only if you accept checksum-only, or wait for a signed tag.
