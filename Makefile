BINARY := $(CURDIR)/bin/recuerd0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
GO := go

.PHONY: build test-unit clean tidy

build:
	@mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/recuerd0

test-unit:
	$(GO) test -v ./internal/...

clean:
	rm -rf bin/
	$(GO) clean

tidy:
	$(GO) mod tidy
