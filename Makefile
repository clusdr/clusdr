.PHONY: build bench test vet lint proto proto-lint proto-python clean

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

bench:
	go build -o bin/clusdr-bench ./cmd/clusdr-bench

test:
	go test -race ./...
	go test -C sdk -race ./...

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

clean:
	rm -rf bin/
