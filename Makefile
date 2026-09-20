.PHONY: build build-operator bench soak test cover smoke vet lint proto proto-lint proto-python helm manifests hooks clean

VERSION  ?= dev
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT_AT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

PKG      := github.com/clusdr/clusdr/internal/version
LDFLAGS  := -ldflags "\
  -X $(PKG).Version=$(VERSION) \
  -X $(PKG).Commit=$(COMMIT) \
  -X $(PKG).BuildTime=$(BUILT_AT)"

build:
	go build $(LDFLAGS) -o bin/clusdr ./cmd/clusdr

smoke: build
	chmod +x scripts/release-smoke.sh
	./scripts/release-smoke.sh ./bin/clusdr

build-operator:
	go build $(LDFLAGS) -o bin/clusdr-operator ./cmd/clusdr-operator

bench:
	go build -o bin/clusdr-bench ./cmd/clusdr-bench

soak:
	go build -o bin/clusdr-soak ./cmd/clusdr-soak

test:
	go test -race ./...
	go test -C sdk -race ./...

# Statement coverage for daemon + operator + SDK. Examples and load
# generators (clusdr-bench, clusdr-soak) are excluded.
cover:
	@pkgs=$$(go list ./... | grep -Ev '/examples/|/cmd/clusdr-bench|/cmd/clusdr-soak'); \
	go test -count=1 -p 1 -covermode=atomic -coverprofile=coverage.out $$pkgs
	@go tool cover -func=coverage.out | tail -1
	go test -C sdk -count=1 -covermode=atomic -coverprofile=../sdk-coverage.out ./...
	@go tool cover -func=sdk-coverage.out | tail -1

vet:
	go vet ./...
	go vet -C sdk ./...

lint:
	golangci-lint run ./...
	cd sdk && golangci-lint run --config ../.golangci.yml ./...

proto:
	buf generate

proto-lint:
	buf lint
	buf format --exit-code --diff

proto-python:
	$(MAKE) -C ../clusdr-python proto

helm:
	helm lint charts/clusdr
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: DaemonSet'
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'app.kubernetes.io/component: prepare'
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'busybox:1.37.0'
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: Deployment'
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'image: durguto/clusdr:0.2.1'
	helm template clusdr charts/clusdr --namespace clusdr --set image.tag=dev | grep -q 'image: durguto/clusdr:dev'
	if helm template clusdr charts/clusdr --namespace clusdr --set voterCount=4 >/dev/null 2>&1; then echo "voterCount=4 must fail" >&2; exit 1; fi
	if helm template clusdr charts/clusdr --namespace clusdr --set probes.type=foo >/dev/null 2>&1; then echo "probes.type=foo must fail" >&2; exit 1; fi
	if helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: StatefulSet'; then echo "chart must not default sidecar STS" >&2; exit 1; fi
	if helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: CustomResourceDefinition'; then echo "chart must not carry CRDs" >&2; exit 1; fi
	tmp=$$(mktemp -d); \
	./scripts/package-chart.sh 0.0.0-ci "$$tmp"; \
	test -f "$$tmp/clusdr-0.0.0-ci.tgz"; \
	rm -rf "$$tmp"

manifests:
	tmp=$$(mktemp -d); \
	./scripts/package-operator-yaml.sh 0.2.1 "$$tmp"; \
	grep -q 'kind: CustomResourceDefinition' "$$tmp/clusdr-crds.yaml"; \
	grep -q 'name: WARNING' "$$tmp/clusdr-crds.yaml"; \
	grep -q 'image: durguto/clusdr-operator:0.2.1' "$$tmp/clusdr-operator.yaml"; \
	grep -q 'kind: CustomResourceDefinition' "$$tmp/clusdr-operator-bundle.yaml"; \
	grep -q 'image: durguto/clusdr-operator:0.2.1' "$$tmp/clusdr-operator-bundle.yaml"; \
	test -f "$$tmp/clusdr-crds-0.2.1.yaml"; \
	test -f "$$tmp/clusdr-operator-0.2.1.yaml"; \
	test -f "$$tmp/clusdr-operator-bundle.yaml"; \
	test -f "$$tmp/clusdr-operator-bundle-0.2.1.yaml"; \
	if grep -q '^kind: CustomResourceDefinition$$' "$$tmp/clusdr-operator.yaml"; then echo "operator.yaml must not include the CRD" >&2; exit 1; fi; \
	if grep -q '^kind: ClusdrCluster$$' "$$tmp/clusdr-operator.yaml" "$$tmp/clusdr-operator-bundle.yaml"; then echo "sample CR in operator YAML" >&2; exit 1; fi; \
	rm -rf "$$tmp"; \
	python3 -c 'from pathlib import Path; s=Path("examples/k8s/app.yaml").read_text(); \
		[exit(p+" must contain examples/k8s/app.yaml verbatim") for p in ("docs/guide/from-your-app.md","docs/guide/kubernetes-operator.md") if s not in Path(p).read_text()]'

# Copies scripts/githooks into .git/hooks (does not change git config).
hooks:
	@test -d .git || { echo "hooks: not a git checkout" >&2; exit 1; }
	cp scripts/githooks/pre-commit scripts/githooks/commit-msg .git/hooks/
	chmod +x .git/hooks/pre-commit .git/hooks/commit-msg scripts/lint-commit-msg.sh
	@echo "installed .git/hooks/{pre-commit,commit-msg}"

clean:
	rm -rf bin/
