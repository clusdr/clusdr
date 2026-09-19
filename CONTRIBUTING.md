# Contributing

Build and test from this repository. Operators install a **Linux** release binary ([guide](docs/guide/install.md)); this file is the source-build path (any Go 1.27 host).

Product behavior is described in [docs/](docs/README.md), not here.

License: [Apache-2.0](LICENSE). Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Security: [SECURITY.md](SECURITY.md). Governance: [GOVERNANCE.md](GOVERNANCE.md).

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

A breaking change uses `feat!:` (or another type with `!`) and a `BREAKING CHANGE:` footer. Subject is lowercase after the type, no trailing period. Header and body lines are at most 100 characters (wrap the body; ~72 on the subject is easier to read). A body needs a blank line after the subject.

CI lints PR commits against that grammar. Prefer squash-merge; the squash title must stay conventional.

```bash
make hooks   # .git/hooks: gofmt on staged .go, commit-msg same as CI
```

## Branching

`main` is the trunk. It is protected: no direct push, no force-push, no delete. Every change lands through a pull request. CI must be green (Lint, Test, Helm, Buf, Conventional Commits). Those names always report; Lint/Test/Helm/Buf skip the work when their paths did not change. Conventional Commits always runs. Squash-merge; the squash title stays conventional.

Branch names: `feat/…`, `fix/…`, `docs/…`, `ci/…`, `chore/…`. Do not cut a release tag from a red `main`.

This is GitHub Flow plus SemVer tags (`vX.Y.Z-rc.N`, then `vX.Y.Z`). There is no `develop` branch.

## Requirements

- Go 1.27
- [Buf](https://buf.build/docs/cli/installation) (1.73+) for proto — `brew install bufbuild/buf/buf`

```bash
make test    # go test -race ./... and sdk/
make cover   # statement coverage (examples and clusdr-bench excluded); prints totals
make vet
make lint    # golangci-lint on the Go modules (see .golangci-lint-version)
make build
make build-operator
make smoke    # init + start + health + members, then stop
make hooks    # install local pre-commit and commit-msg hooks
```

`proto/api` is the application wire (`buf.build/clusdr/api`). `proto/internal` is join/heartbeat (`buf.build/clusdr/internal`). After editing either:

```bash
make proto-lint   # buf lint + format
make proto        # buf generate → api/
make proto-python # sibling clusdr-python stubs
```

Pull requests lint, format, and run `buf breaking` against the PR base (FILE). Additive changes in `v1alpha1` are fine; field delete/renumber/type change is not. Intentional breaks use the `buf skip breaking` label.

Language SDKs export `buf.build/clusdr/api` only (or the sibling `../clusdr/proto/api` checkout). They never take `internal`.

Push to `main` or a GA `vX.Y.Z` tag (not `-rc`) publishes both modules to the [Buf Schema Registry](https://buf.build/clusdr). That needs a `clusdr` org on buf.build and repo secret `BUF_TOKEN` (account token with push). Repositories are created public so SDK CI can export without a token.

Message names follow Buf STANDARD: `{Method}Request` when the method is unique in the package (`GrantRequest`, `LockRequest`). Two services that share a method name use `{Service}{Method}Request` (`LockServiceRenewRequest`, `LeaseServiceRenewRequest`). `TryLock` has its own request/response types (same fields as `Lock`). gRPC method paths (`/clusdr.v1alpha1.LockService/TryLock`) are the wire identity; protobuf field numbers are the payload identity.

`gofmt` on changed Go files.

## Dependencies

Go modules come from the public module proxy, recorded in `go.mod` / `go.sum`. Prefer well-known libraries (stdlib first). Do not vendor. Dependabot opens weekly PRs for Go, Actions, and Docker base images. A GA tag does not ship a known critical/high hole without a note in the release — see [SECURITY.md](SECURITY.md).

## Review

On a pull request, check that tests cover the new path (or the PR says why not), docs in `docs/` moved with the behavior, and the squash title stays conventional. Required CI must be green.

## Tests

Major new functionality must include automated tests in the same change (or a short note in the PR explaining why tests are not applicable). Docs-only and comment-only changes are exempt. Run the suite with `make test` (see the pull-request Testing checklist). `make cover` prints statement coverage locally (examples and `clusdr-bench` excluded). CI’s Test job writes the same profiles (`coverage.out`, `sdk-coverage.out`) and uploads them as an artifact; it does not fail the build on a percentage.

`make build` does not strip symbols. GitHub Release binaries use `-s -w`.

## Docs

If you change configuration, CLI, proto, SDK, or defaults, update the matching page under `docs/` in the **same** change. Each page is one type: tutorial (`docs/guide/` steps 1–5), how-to (other hosts, Kubernetes apply pages), reference, or explanation (overview, architecture, `docs/concepts/`). Do not mix types on one page. Do not put roadmap or design notes in `docs/`.

## Layout

```text
cmd/clusdr          daemon CLI
cmd/clusdr-bench    load generator
cmd/clusdr-operator Kubernetes operator (ClusdrCluster → DaemonSet)
config/crd          ClusdrCluster CRD
config/operator     operator RBAC + Deployment
charts/clusdr       Helm chart (DaemonSet example; does not join)
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

## Releases

Do not tag until `main` is green. Cut an RC first; GA is a second tag after that Release workflow is green.

```bash
git checkout main
git pull
git tag vX.Y.Z-rc.1
git push origin vX.Y.Z-rc.1
# wait for Release (lint, test, smoke, GitHub prerelease, versioned images/chart)
git tag vX.Y.Z
git push origin vX.Y.Z
```

A hyphen in the tag (`-rc.1`) is a prerelease: GitHub Release is marked prerelease, images and the chart use that version, `:latest` / `latest.json` / Artifact Hub metadata / BSR tag push / language SDK registries do **not** move. A tag with no hyphen (`vX.Y.Z`) is GA and publishes all of those.

`clusdr.io/download/<file>` 302s to the GitHub Release. Images: `durguto/clusdr` and `durguto/clusdr-operator` (GHCR mirrors `ghcr.io/clusdr/clusdr` and `ghcr.io/clusdr/clusdr-operator`). Chart: `helm push` to `oci://ghcr.io/clusdr/charts` (`clusdr-<version>.tgz` also lands on the GitHub Release). Do not push the chart to `ghcr.io/clusdr/clusdr` (daemon image). Make the GHCR package `charts/clusdr` **public**. The GA job `oras push`es `charts/clusdr/artifacthub-repo.yml` as tag `artifacthub.io`.

One-time on [Artifact Hub](https://artifacthub.io): in the **clusdr** org, add a Helm repository, kind **OCI**, URL `oci://ghcr.io/clusdr/charts/clusdr`. After the first chart tag exists, Artifact Hub indexes it. Paste the repository ID into `artifacthub-repo.yml` (`repositoryID`) so the next GA push can show Verified publisher. `owners.email` must match the Artifact Hub login. Catalog URL: `https://artifacthub.io/packages/helm/clusdr/clusdr` (repo name = what you set in the org). `scripts/package-operator-yaml.sh` writes `clusdr-crds.yaml` and `clusdr-operator.yaml` (plus versioned copies) onto the GitHub Release. `https://clusdr.io/download/…` redirects there.

Repo secrets (not in git):

- `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN`

Create the Hub repo `durguto/clusdr-operator` (the daemon repo `durguto/clusdr` already exists). To republish images, chart, and YAML for an existing tag: Release `workflow_dispatch`.

`docker build -f Dockerfile.operator` and `kubectl apply -k config/operator` stay the contributor path.

