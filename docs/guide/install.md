# Install the binary

`clusdr` is one program: the daemon and the CLI. Putting it on `PATH` is how you run a cluster without cloning the repository. Cloning is the contributor path ([CONTRIBUTING.md](https://github.com/clusdr/clusdr/blob/main/CONTRIBUTING.md)) and leaves you with an unstripped `dev` binary.

| | Value | Why it matters |
|---|---|---|
| OS | Linux | Windows and macOS are not documented hosts; the script will not hand you a working cluster there. |
| Arch | amd64 or arm64 | Other architectures have no published binary. |

```bash
curl -fsSL https://clusdr.io/install.sh | sh
```

That writes the latest release into `/usr/local/bin`. If that directory is not writable (permission denied), pick a prefix you own:

```bash
curl -fsSL https://clusdr.io/install.sh | BINDIR=~/bin sh
```

Then ensure `~/bin` is on `PATH`, or the next page’s `clusdr init` will say `command not found`.

Pin a tag when you need a known build: `CLUSDR_VERSION=0.2.0`. The script checks SHA-256 against `checksums.txt` from the same release; a truncated download fails that check instead of installing a half file.

Sigstore signatures on that checksum file, the image, and the Helm chart live on [Verify a release](verify-release.md). `v0.2.0` and earlier are checksum-only.

## Checkpoint

```bash
clusdr version
```

A release binary prints the tag, commit, and build time. A binary you compiled yourself prints `dev` — that is fine for hacking, not for the tutorial’s later checksum story.

You have not started a cluster yet. No `CLUSDR_*` is required on this laptop.

Next: [Start the first member →](first-member.md)

Signatures on the checksum file, the Docker image, or a source build: [Verify a release](verify-release.md), [Run on other hosts](other-hosts.md). The SDK is [step 5](from-your-app.md); it still needs this daemon on the same machine.
