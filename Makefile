# ──────────────────────────────────────────────────────────────
#  Reglet CLI
# ──────────────────────────────────────────────────────────────

BINARY   := cli
CMD      := ./cmd/cli
MODULE   := github.com/reglet-dev/cli

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -s -w \
  -X $(MODULE)/internal/cli.Version=$(VERSION) \
  -X $(MODULE)/internal/cli.Commit=$(COMMIT) \
  -X $(MODULE)/internal/cli.BuildTime=$(BUILDTIME)

GOFLAGS  ?=
TAGS     ?=

# Colors
C_RESET  := \033[0m
C_BOLD   := \033[1m
C_DIM    := \033[2m
C_CYAN   := \033[36m
C_GREEN  := \033[32m
C_YELLOW := \033[33m
C_RED    := \033[31m
C_CHECK  := $(C_GREEN)✓$(C_RESET)
C_ARROW  := $(C_CYAN)→$(C_RESET)

# ──────────────────────────────────────────────────────────────
#  Build
# ──────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build the CLI binary
	@printf "  $(C_ARROW) Building $(C_BOLD)$(BINARY)$(C_RESET) $(C_DIM)$(VERSION)$(C_RESET)\n"
	@CGO_ENABLED=0 go build $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	@printf "  $(C_CHECK) $(C_BOLD)$(BINARY)$(C_RESET) $(C_DIM)$(shell ls -lh $(BINARY) 2>/dev/null | awk '{print $$5}')$(C_RESET)\n"

.PHONY: build-embed
build-embed: TAGS += embed_plugins
build-embed: build ## Build with embedded plugins

.PHONY: install
install: ## Install to GOBIN
	@printf "  $(C_ARROW) Installing $(C_BOLD)$(BINARY)$(C_RESET)\n"
	@CGO_ENABLED=0 go install $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" $(CMD)
	@printf "  $(C_CHECK) Installed to $(C_DIM)$$(go env GOBIN || go env GOPATH)/bin$(C_RESET)\n"

.PHONY: clean
clean: ## Remove build artifacts
	@rm -f $(BINARY)
	@go clean -cache -testcache 2>/dev/null || true
	@printf "  $(C_CHECK) Clean\n"

# ──────────────────────────────────────────────────────────────
#  Test
# ──────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run tests
	@printf "  $(C_ARROW) Testing\n"
	@go test ./... -count=1 2>&1 | while IFS= read -r line; do \
		case "$$line" in \
			ok*) printf "  \033[32m✓\033[0m %s\n" "$$line" ;; \
			FAIL*) printf "  \033[31m✗\033[0m %s\n" "$$line" ;; \
			'?'*) printf "  \033[2m%s\033[0m\n" "$$line" ;; \
			*) printf "    %s\n" "$$line" ;; \
		esac; \
	done

.PHONY: test-v
test-v: ## Run tests (verbose)
	@go test -v ./... -count=1

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	@printf "  $(C_ARROW) Testing with coverage\n"
	@go test ./... -count=1 -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1 | awk '{printf "  $(C_CHECK) Coverage: $(C_BOLD)%s$(C_RESET)\n", $$3}'
	@rm -f coverage.out

# ──────────────────────────────────────────────────────────────
#  Lint
# ──────────────────────────────────────────────────────────────

.PHONY: vet
vet: ## Run go vet
	@printf "  $(C_ARROW) Vet\n"
	@go vet ./...
	@printf "  $(C_CHECK) Clean\n"

.PHONY: lint
lint: ## Run golangci-lint
	@printf "  $(C_ARROW) Lint\n"
	@golangci-lint run ./...
	@printf "  $(C_CHECK) Clean\n"

.PHONY: fmt
fmt: ## Format code
	@printf "  $(C_ARROW) Formatting\n"
	@gofmt -w .
	@printf "  $(C_CHECK) Done\n"

.PHONY: fmt-check
fmt-check: ## Check formatting (no changes)
	@printf "  $(C_ARROW) Checking format\n"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && printf "  $(C_RED)✗ Files need formatting$(C_RESET)\n" && exit 1)
	@printf "  $(C_CHECK) Clean\n"

# ──────────────────────────────────────────────────────────────
#  Dev
# ──────────────────────────────────────────────────────────────

.PHONY: tidy
tidy: ## Tidy and verify dependencies
	@printf "  $(C_ARROW) Tidying modules\n"
	@go mod tidy
	@go mod verify >/dev/null
	@printf "  $(C_CHECK) Done\n"

.PHONY: check
check: fmt-check vet test ## Run all checks (format, vet, test)

.PHONY: dev
dev: tidy build ## Tidy, build — quick iteration loop

# ──────────────────────────────────────────────────────────────
#  Help
# ──────────────────────────────────────────────────────────────

.PHONY: help
help:
	@printf "\n  $(C_BOLD)Reglet CLI$(C_RESET) $(C_DIM)$(VERSION)$(C_RESET)\n\n"
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  $(C_CYAN)%-14s$(C_RESET) %s\n", $$1, $$2}'
	@printf "\n"

.DEFAULT_GOAL := help
