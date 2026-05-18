# KubeKit build targets.
# `make` (no args) builds the Go binary into ./bin/kubekit.

GO        ?= go
BINARY    := kubekit
BIN_DIR   := bin
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -s -w -X main.Version=$(VERSION)

.PHONY: build run tidy test clean

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/kubekit

run: build
	./$(BIN_DIR)/$(BINARY)

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

clean:
	rm -rf $(BIN_DIR)
