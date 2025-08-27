# Makefile for go-slim.dev/env

GO       ?= go
PKG      ?= ./...
ARTIFACTS?= artifacts

.PHONY: help
help: ## Show this help
	@echo "Common targets:"
	@echo "  make ci           # tidy check + build + vet + fmt check + test"
	@echo "  make tidy         # run go mod tidy"
	@echo "  make tidy-check   # ensure go.mod/go.sum are tidy (no diff)"
	@echo "  make fmt          # format code in-place (gofmt -s -w)"
	@echo "  make fmt-check    # check formatting (no changes)"
	@echo "  make vet          # run go vet"
	@echo "  make build        # go build ./..."
	@echo "  make test         # go test -v ./..."
	@echo "  make bench        # run benchmarks"
	@echo "  make bench-save   # run benchmarks and save to artifacts/bench.txt"
	@echo "  make clean        # remove artifacts"

.PHONY: ci
ci: tidy-check build vet fmt-check test ## Run CI-like checks locally

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: tidy ## Ensure go.mod/go.sum are tidy (no diff)
	@git diff --exit-code -- go.mod go.sum

.PHONY: fmt
fmt: ## Format code in-place
	@gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check formatting (no changes expected)
	@fmtout=$$(gofmt -s -l .); \
	if [ -n "$$fmtout" ]; then \
	  echo "The following files are not properly formatted:" >&2; \
	  echo "$$fmtout" >&2; \
	  exit 1; \
	fi

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKG)

.PHONY: build
build: ## Build all packages
	$(GO) build $(PKG)

.PHONY: test
test: ## Run tests verbosely
	$(GO) test -v $(PKG)

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -bench . -benchmem -run '^$$' $(PKG)

.PHONY: bench-save
bench-save: ## Run benchmarks and save to artifacts/bench.txt
	@mkdir -p $(ARTIFACTS)
	$(GO) test -bench . -benchmem -run '^$$' $(PKG) | tee $(ARTIFACTS)/bench.txt

.PHONY: clean
clean: ## Remove artifacts directory
	@rm -rf $(ARTIFACTS)
