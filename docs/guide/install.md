# Install the binary

You need one program: `clusdr`. It is the daemon and the CLI. Do not clone the repository to run a cluster.

Linux, amd64 or arm64. Windows and macOS are not documented hosts.

```bash
curl -fsSL https://clusdr.io/install.sh | sh
```

That puts the latest release into `/usr/local/bin`. Another prefix:

```bash
curl -fsSL https://clusdr.io/install.sh | BINDIR=~/bin sh
```

Pin a tag with `CLUSDR_VERSION=0.2.0`. The script checks SHA-256 against `checksums.txt`.

## Checkpoint

```bash
clusdr version
```

A release binary prints the tag, commit, and build time. You have not started a cluster yet. No `CLUSDR_*` is required on this laptop.

Next: [Start the first member →](first-member.md)

Verify signatures, pull the Docker image, or build from source: [Verify a release](verify-release.md), [Run on other hosts](other-hosts.md), [CONTRIBUTING.md](https://github.com/clusdr/clusdr/blob/main/CONTRIBUTING.md). The SDK is [step 5](from-your-app.md).
