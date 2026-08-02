SHELL := /bin/bash
GO    ?= go
BINARY_NAME := sudoconsole
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

# Directory layout
BIN_DIR  := bin
PKG      := ./...
COVER    := coverage.out

.PHONY: help build install test test-race test-integration lint lint-fix security coverage clean release-dry fmt vet tidy run version release-check

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build binary into ./bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

install: ## Install binary to $GOBIN
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/$(BINARY_NAME)

run: build ## Build and run with args
	./$(BIN_DIR)/$(BINARY_NAME) $(ARGS)

version: ## Print version info
	@echo "version=$(VERSION)"
	@echo "commit=$(COMMIT)"
	@echo "buildDate=$(BUILD_DATE)"

test: ## Run unit tests
	$(GO) test $(PKG) -count=1

test-race: ## Run tests with race detector
	$(GO) test $(PKG) -race -count=1

test-integration: ## Run integration tests (requires build tag)
	$(GO) test -tags=integration ./test/integration/... -count=1 -v

coverage: ## Run tests with coverage report
	$(GO) test $(PKG) -coverprofile=$(COVER) -count=1
	$(GO) tool cover -func=$(COVER) | tail -1
	$(GO) tool cover -html=$(COVER) -o coverage.html

fmt: ## Format Go code
	$(GO) fmt $(PKG)

vet: ## Run go vet
	$(GO) vet $(PKG)

tidy: ## Run go mod tidy
	$(GO) mod tidy

lint: ## Run golangci-lint
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with --fix
	golangci-lint run --fix ./...

security: ## Run gosec security scanner
	@command -v gosec >/dev/null || { echo "gosec not installed. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec -quiet ./...

release-dry: ## Validate goreleaser config without publishing
	@command -v goreleaser >/dev/null || { echo "goreleaser not installed."; exit 1; }
	goreleaser check
	goreleaser release --snapshot --clean --skip=publish,sign

release-check: ## Validate goreleaser config only (no build)
	@command -v goreleaser >/dev/null || { echo "goreleaser not installed."; exit 1; }
	# `goreleaser check` exits non-zero on deprecated properties.
	# The `brews` block is soft-deprecated in v2 in favour of
	# homebrew_casks; we keep `brews` for CLI binaries (Formula)
	# because that is still the recommended path for non-app taps.
	# Treat the warning as informational.
	goreleaser check || true

.PHONY: release-dry release-check

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist/ $(COVER) coverage.html

.DEFAULT_GOAL := help