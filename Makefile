.PHONY: build build-operator bench test cover vet lint proto proto-lint proto-python helm manifests clean

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

build-operator:
	go build $(LDFLAGS) -o bin/clusdr-operator ./cmd/clusdr-operator

bench:
	go build -o bin/clusdr-bench ./cmd/clusdr-bench

test:
	go test -race ./...
	go test -C sdk -race ./...

# Statement coverage for daemon + operator + SDK. Examples and clusdr-bench
# are excluded: they are demos / a load generator, not production packages.
cover:
	@pkgs=$$(go list ./... | grep -Ev '/examples/|/cmd/clusdr-bench'); \
	go test -count=1 -covermode=atomic -coverprofile=coverage.out $$pkgs
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
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: Deployment'
	helm template clusdr charts/clusdr --namespace clusdr | grep -q 'image: durguto/clusdr:0.2.0'
	helm template clusdr charts/clusdr --namespace clusdr --set image.tag=dev | grep -q 'image: durguto/clusdr:dev'
	if helm template clusdr charts/clusdr --namespace clusdr --set voterCount=4 >/dev/null 2>&1; then echo "voterCount=4 must fail" >&2; exit 1; fi
	if helm template clusdr charts/clusdr --namespace clusdr | grep -q 'kind: StatefulSet'; then echo "chart must not default sidecar STS" >&2; exit 1; fi
	tmp=$$(mktemp -d); \
	./scripts/package-chart.sh 0.0.0-ci "$$tmp"; \
	test -f "$$tmp/clusdr-0.0.0-ci.tgz"; \
	rm -rf "$$tmp"

manifests:
	tmp=$$(mktemp -d); \
	./scripts/package-operator-yaml.sh 0.2.0 "$$tmp"; \
	grep -q 'kind: CustomResourceDefinition' "$$tmp/clusdr-crds.yaml"; \
	grep -q 'image: durguto/clusdr-operator:0.2.0' "$$tmp/clusdr-operator.yaml"; \
	test -f "$$tmp/clusdr-crds-0.2.0.yaml"; \
	test -f "$$tmp/clusdr-operator-0.2.0.yaml"; \
	if grep -q '^kind: ClusdrCluster$$' "$$tmp/clusdr-operator.yaml"; then echo "sample CR in operator bundle" >&2; exit 1; fi; \
	rm -rf "$$tmp"

clean:
	rm -rf bin/
