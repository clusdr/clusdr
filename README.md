<h1 align="center">
  <a href="https://clusdr.io">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-lockup-dark.svg">
      <img src="docs/assets/logo-lockup.svg" alt="clusdr" width="180" height="184">
    </picture>
  </a>
</h1>

<p align="center">A runtime for the cluster. An SDK for the app.</p>

Distributed runtime for cluster awareness. Applications talk to a **local daemon**; the daemon is the cluster member.

Not a database, queue, or Kubernetes. API is **v1alpha1**. TLS is on by default.

```text
Application → local SDK → local daemon → cluster
```

## Install

```bash
curl -fsSL https://clusdr.io/install.sh | sh
```

Linux amd64/arm64. Image: `ghcr.io/odurgut/clusdr`. Other channels: **[Install](docs/guide/install.md)**.

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

Go: `go get github.com/odurgut/clusdr/sdk`. Python: `pip install clusdr`.

## Docs

Start at the **[guide](docs/guide/README.md)**. Hub: [docs/](docs/README.md).

## Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md). Contract changes (config, proto, SDK) update `docs/` in the same change.
