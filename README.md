<h1 align="center">
  <a href="https://clusdr.io">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="docs/assets/clusdr-lettermark-side-dark.svg">
      <img src="docs/assets/clusdr-lettermark-side.svg" alt="clusdr" width="360" height="119">
    </picture>
  </a>
</h1>

<p align="center">A runtime for the cluster. An SDK for the app.</p>

<p align="center">
  <a href="https://github.com/clusdr/clusdr/actions/workflows/ci.yml"><img src="https://github.com/clusdr/clusdr/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/clusdr/clusdr/blob/main/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/clusdr/clusdr" alt="Go version"></a>
  <a href="https://github.com/clusdr/clusdr/releases"><img src="https://img.shields.io/github/v/release/clusdr/clusdr" alt="Release"></a>
  <a href="https://pkg.go.dev/github.com/clusdr/clusdr/sdk"><img src="https://pkg.go.dev/badge/github.com/clusdr/clusdr/sdk.svg" alt="Go Reference"></a>
  <a href="https://hub.docker.com/r/durguto/clusdr"><img src="https://img.shields.io/docker/pulls/durguto/clusdr" alt="Docker"></a>
  <a href="https://artifacthub.io/packages/search?repo=clusdr"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/clusdr" alt="Artifact Hub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/clusdr/clusdr" alt="License"></a>
</p>

Distributed runtime for cluster awareness. Applications talk to a **local daemon**; the daemon is the cluster member.

Not a database, queue, or Kubernetes. API is **v1alpha1**. TLS is on by default.

```text
Application → local SDK → clusdr daemon → cluster
```

## Install

```bash
curl -fsSL https://clusdr.io/install.sh | sh
```

Linux amd64/arm64. Image: `durguto/clusdr` on Docker Hub (`ghcr.io/clusdr/clusdr` is the same image). Other channels: **[Install](docs/guide/install.md)**.

## Run

```bash
clusdr init
clusdr start --bootstrap
```

Another terminal: `clusdr members`.

## Use

```go
c, err := clusdr.Local()
members, err := c.Members(ctx)
```

```python
from clusdr import local
c = local()
members = c.members()
```

```rust
let c = clusdr::local(clusdr::Options::new()).await?;
let members = c.members().await?;
```

```ts
import { local } from "clusdr";

const c = await local();
const members = await c.members();
```

Go: `go get github.com/clusdr/clusdr/sdk`. Python: `pip install clusdr`. Rust: `clusdr = "0.2.0"`. TypeScript: `npm install clusdr`. Java: `io.clusdr:clusdr`. Runnable copies: [examples/](examples/).

## Docs

Start at the **[guide](docs/guide/README.md)**. Hub: [docs/](docs/README.md).

## Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md). Contract changes (config, proto, SDK) update `docs/` in the same change. Apache-2.0. Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
