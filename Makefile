.PHONY: build bench test vet lint proto proto-python clean

VERSION  ?= dev
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILT_AT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

PKG      := github.com/durguto/clusdr/internal/version
LDFLAGS  := -ldflags "\
  -X $(PKG).Version=$(VERSION) \
  -X $(PKG).Commit=$(COMMIT) \
  -X $(PKG).BuildTime=$(BUILT_AT)"

build:
	go build $(LDFLAGS) -o bin/clusdr ./cmd/clusdr

bench:
	go build -o bin/clusdr-bench ./cmd/clusdr-bench

test:
	go test -race ./...
	go test -C sdk -race ./...

vet:
	go vet ./...
	go vet -C sdk ./...

lint:
	golangci-lint run

proto:
	protoc \
		--proto_path=proto \
		--go_out=api \
		--go_opt=paths=source_relative \
		--go-grpc_out=api \
		--go-grpc_opt=paths=source_relative \
		$$(find proto -name '*.proto')

proto-python:
	python3 -m grpc_tools.protoc \
		--proto_path=proto \
		--python_out=../clusdr-python/src \
		--grpc_python_out=../clusdr-python/src \
		--pyi_out=../clusdr-python/src \
		proto/clusdr/v1alpha1/health.proto \
		proto/clusdr/v1alpha1/membership.proto \
		proto/clusdr/v1alpha1/watch.proto \
		proto/clusdr/v1alpha1/events.proto \
		proto/clusdr/v1alpha1/locks.proto \
		proto/clusdr/v1alpha1/leases.proto

clean:
	rm -rf bin/
