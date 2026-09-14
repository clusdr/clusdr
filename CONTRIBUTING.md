# Contributing

Build and test from this repository. Operators install a release binary ([guide](docs/guide/install.md)); this file is the source-build path.

Product behavior is described in [docs/](docs/README.md), not here.

## Requirements

- Go 1.25

```bash
make test    # go test -race ./... and sdk/
make vet
make build
```

Regenerate gRPC stubs after editing `proto/`:

```bash
make proto
make proto-python   # writes ../clusdr-python/src
```

`gofmt` on changed Go files.

## Docs

If you change configuration, CLI, proto, SDK, or defaults, update the matching page under `docs/` in the **same** change (concepts, tasks, or reference — not a dump on one page). Do not put roadmap or design notes in `docs/`.

## Layout

```text
cmd/clusdr          daemon CLI
cmd/clusdr-bench    load generator
internal/           daemon
proto/              .proto sources
api/                generated Go stubs (module github.com/odurgut/clusdr/api)
sdk/                application SDK (module github.com/odurgut/clusdr/sdk)
```

Sibling checkouts (same parent directory):

- [`clusdr-python`](https://github.com/odurgut/clusdr-python) — Python SDK
- [`clusdr-site`](https://github.com/odurgut/clusdr-site) — clusdr.io

