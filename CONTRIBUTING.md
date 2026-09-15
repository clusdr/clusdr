# Contributing

Build and test from this repository. Operators install a **Linux** release binary ([guide](docs/guide/install.md)); this file is the source-build path (any Go 1.25 host).

Product behavior is described in [docs/](docs/README.md), not here.

License: [Apache-2.0](LICENSE). Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Security: [SECURITY.md](SECURITY.md).

## Commits

Every commit and pull-request title uses [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>(optional-scope): <imperative summary>
```

Types we use: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

Examples:

```text
feat(sdk): add a Members name filter
fix(raft): retry snapshot restore
docs: document CLUSDR_RAFT_ADDR on other hosts
ci: run goreleaser check on pull requests
```

A breaking change uses `feat!:` (or another type with `!`) and a `BREAKING CHANGE:` footer. Subject is lowercase after the type, no trailing period, ~72 characters.

CI lints PR commits against that grammar. Prefer squash-merge; the squash title must stay conventional.

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
api/                generated Go stubs (module github.com/durguto/clusdr/api)
sdk/                application SDK (module github.com/durguto/clusdr/sdk)
```

Sibling checkouts:

- [`clusdr-python`](https://github.com/durguto/clusdr-python) — Python SDK
- [`clusdr-site`](https://github.com/durguto/clusdr-site) — clusdr.io

