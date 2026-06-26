BINARY := $(CURDIR)/bin/recuerd0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
GO := go

.PHONY: build test-unit clean tidy release-check release-snapshot

build:
	@mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/recuerd0

test-unit:
	$(GO) test -v ./internal/...

clean:
	rm -rf bin/ dist/
	$(GO) clean

tidy:
	$(GO) mod tidy

# Validate .goreleaser.yaml.
release-check:
	goreleaser check

# Dry-run the full release pipeline locally (no publish); artifacts land in dist/.
release-snapshot:
	goreleaser release --snapshot --clean
