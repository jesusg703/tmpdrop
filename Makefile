GO     ?= go
BIN     := bin/tmpdrop
VERSION ?= 2.1.0

GOFLAGS := -trimpath -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all build test lint fmt clean run build-docker test-docker lint-docker

all: build

build:
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -o $(BIN) ./cmd/tmpdrop

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...
	@out="$$(gofmt -l cmd internal)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt needed on:"; echo "$$out"; exit 1; \
	fi
	@echo "vet + gofmt: OK"

fmt:
	gofmt -w cmd internal

clean:
	rm -rf bin

run:
	$(GO) run ./cmd/tmpdrop

# Variants for hosts without a local Go toolchain; they build and test inside
# a Go container.
DOCKER_GO := docker run --rm -v "$$(pwd)":/src -w /src golang:1.24-alpine

build-docker:
	@mkdir -p bin
	$(DOCKER_GO) sh -c 'CGO_ENABLED=0 go build $(GOFLAGS) -o /tmp/tmpdrop ./cmd/tmpdrop && cp /tmp/tmpdrop bin/tmpdrop'

test-docker:
	$(DOCKER_GO) go test ./...

lint-docker:
	$(DOCKER_GO) sh -c 'go vet ./... && out="$$(gofmt -l cmd internal)"; if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi'
	@echo "vet + gofmt: OK"
