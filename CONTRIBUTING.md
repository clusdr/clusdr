# Contributing

Build and test from this repository. Operators install a **Linux** release binary ([guide](docs/guide/install.md)); this file is the source-build path (any Go 1.27 host).

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

- Go 1.27
- [Buf](https://buf.build/docs/cli/installation) (1.73+) for proto — `brew install bufbuild/buf/buf`

```bash
make test    # go test -race ./... and sdk/
make vet
make lint    # golangci-lint on the Go modules (see .golangci-lint-version)
make build
```

`proto/api` is the application wire (`buf.build/clusdr/api`). `proto/internal` is join/heartbeat (`buf.build/clusdr/internal`). After editing either:

```bash
make proto-lint   # buf lint + format
make proto        # buf generate → api/
make proto-python # sibling clusdr-python stubs
```

Pull requests lint, format, and run `buf breaking` against the PR base (FILE). Additive changes in `v1alpha1` are fine; field delete/renumber/type change is not. Intentional breaks use the `buf skip breaking` label.

Language SDKs export `buf.build/clusdr/api` only (or the sibling `../clusdr/proto/api` checkout). They never take `internal`.

Push to `main` or a `v*` tag publishes both modules to the [Buf Schema Registry](https://buf.build/clusdr). That needs a `clusdr` org on buf.build and repo secret `BUF_TOKEN` (account token with push). Repositories are created public so SDK CI can export without a token.

Message names follow Buf STANDARD: `{Method}Request` when the method is unique in the package (`GrantRequest`, `LockRequest`). Two services that share a method name use `{Service}{Method}Request` (`LockServiceRenewRequest`, `LeaseServiceRenewRequest`). `TryLock` has its own request/response types (same fields as `Lock`). gRPC method paths (`/clusdr.v1alpha1.LockService/TryLock`) are the wire identity; protobuf field numbers are the payload identity.

`gofmt` on changed Go files.

## Docs

If you change configuration, CLI, proto, SDK, or defaults, update the matching page under `docs/` in the **same** change (concepts, tasks, or reference — not a dump on one page). Do not put roadmap or design notes in `docs/`.

## Layout

```text
cmd/clusdr          daemon CLI
cmd/clusdr-bench    load generator
examples/           small programs against a local daemon; each example has go/, python/, rust/, typescript/, java/ packages
internal/           daemon
proto/api           application .proto (BSR buf.build/clusdr/api)
proto/internal      join/heartbeat .proto (BSR buf.build/clusdr/internal)
buf.yaml            Buf workspace (two named modules)
buf.gen.yaml        Go stub generation into api/
api/                generated Go stubs (module github.com/clusdr/clusdr/api)
sdk/                application SDK (module github.com/clusdr/clusdr/sdk)
```

Sibling checkouts:

- [`clusdr-python`](https://github.com/clusdr/clusdr-python) — Python SDK
- [`clusdr-rust`](https://github.com/clusdr/clusdr-rust) — Rust SDK
- [`clusdr-js`](https://github.com/clusdr/clusdr-js) — TypeScript SDK
- [`clusdr-java`](https://github.com/clusdr/clusdr-java) — Java SDK
- [`clusdr-site`](https://github.com/clusdr/clusdr-site) — clusdr.io

