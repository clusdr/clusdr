# 1. Install the binary

You need one program: `clusdr`. It is the daemon and the CLI. Do not clone the repository to run a cluster.

## Put it on PATH

Linux, amd64 or arm64. Windows and macOS are not documented hosts.

```bash
curl -fsSL https://clusdr.io/install.sh | sh
```

That puts the latest release into `/usr/local/bin`. Another prefix:

```bash
curl -fsSL https://clusdr.io/install.sh | BINDIR=~/bin sh
```

The script checks SHA-256 against `checksums.txt`. Pin a tag with `CLUSDR_VERSION=0.1.0`. Override the archive origin with `CLUSDR_DOWNLOAD_ORIGIN`.

Archives are published as GitHub Releases and served from `clusdr.io`. Direct GitHub URL if you need it:

```bash
curl -fsSL https://github.com/durguto/clusdr/releases/latest/download/install.sh | sh
```

Or unpack `clusdr_<version>_linux_<arch>.tar.gz` from [clusdr.io/download](https://clusdr.io/download/) ([GitHub Releases](https://github.com/durguto/clusdr/releases) is the source). Checksums sit next to the archives.

## Check it

```bash
clusdr version
```

A release binary prints the tag, commit, and build time. A binary built from source prints `dev`.

You have not started a cluster yet. Defaults when you do:

| Knob | Default |
|---|---|
| `data.dir` | `$HOME/.clusdr` |
| Runtime API | `127.0.0.1:7947` |
| Raft | `127.0.0.1:7946` |
| Control socket | `$HOME/.clusdr/clusdr.sock` |

No `CLUSDR_*` is required on a laptop.

## Other ways to get the binary

**Docker Hub** (the published Linux image; used later on [other hosts](other-hosts.md)):

```bash
docker pull odurgut/clusdr
```

The image is distroless, non-root, `ENTRYPOINT /clusdr`, `CMD start`, volume `/var/lib/clusdr`, port **7947**. Map Raft **7946** if peers sit outside the container network.

**From source** is for people changing the daemon: [CONTRIBUTING.md](../../CONTRIBUTING.md). `clusdr-bench` is not a release artifact.

**The SDK is not this install.** Apps: [SDKs](../sdk/README.md). First call in [step 5](from-your-app.md). They still need this daemon on the same machine.

## Next

[Start the first member →](first-member.md)
