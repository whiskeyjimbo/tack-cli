# ╔════════════════════════════════════════════════════════════════════════════╗
# ║                               TACK MAKEFILE                               ║
# ╚════════════════════════════════════════════════════════════════════════════╝
#
# Usage: make <target>
# Run 'make help' for a list of available targets
#

.PHONY: all build build-embed clean test test-v test-cover lint fmt fmt-check vet help install dev tidy check

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────

BINARY_NAME := tack
CMD         := ./cmd/cli
MODULE      := github.com/whiskeyjimb/tack-cli

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(MODULE)/internal/cli.Version=$(VERSION) \
  -X $(MODULE)/internal/cli.Commit=$(COMMIT) \
  -X $(MODULE)/internal/cli.BuildTime=$(BUILDTIME)

GOFLAGS  ?=
TAGS     ?=

# Go commands
GOCMD   := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST  := $(GOCMD) test
GOGET   := $(GOCMD) get
GOMOD   := $(GOCMD) mod
GOFMT   := gofmt
GOVET   := $(GOCMD) vet
GOLINT  := golangci-lint

# ─────────────────────────────────────────────────────────────────────────────
# Colors and Formatting
# ─────────────────────────────────────────────────────────────────────────────

# Colors
RESET   := \033[0m
BOLD    := \033[1m
RED     := \033[31m
GREEN   := \033[32m
YELLOW  := \033[33m
BLUE    := \033[34m
MAGENTA := \033[35m
CYAN    := \033[36m
WHITE   := \033[37m

# Styled prefixes
INFO    := @printf "$(BOLD)$(CYAN)▸$(RESET) "
SUCCESS := @printf "$(BOLD)$(GREEN)✓$(RESET) "
WARN    := @printf "$(BOLD)$(YELLOW)⚠$(RESET) "
ERROR   := @printf "$(BOLD)$(RED)✗$(RESET) "
STEP    := @printf "$(BOLD)$(MAGENTA)→$(RESET) "

# ═══════════════════════════════════════════════════════════════════════════════
# PRIMARY TARGETS
# ═══════════════════════════════════════════════════════════════════════════════

.DEFAULT_GOAL := help

##@ Primary

all: clean lint test build  ## Run full pipeline: clean → lint → test → build
	$(SUCCESS)
	@printf "$(GREEN)All tasks completed successfully!$(RESET)\n"

build: ## Build the CLI binary
	$(INFO)
	@printf "Building $(BOLD)$(BINARY_NAME)$(RESET)...\n"
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOBUILD) $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) $(CMD)
	$(SUCCESS)
	@printf "Binary built: $(GREEN)bin/$(BINARY_NAME)$(RESET)\n"

build-embed: TAGS += embed_plugins
build-embed: build ## Build with embedded plugins

install: ## Install to GOBIN
	$(INFO)
	@printf "Installing $(BOLD)$(BINARY_NAME)$(RESET)...\n"
	@CGO_ENABLED=0 $(GOCMD) install $(GOFLAGS) -tags "$(TAGS)" -ldflags "$(LDFLAGS)" $(CMD)
	$(SUCCESS)
	@printf "Installed to $(GREEN)$(shell $(GOCMD) env GOBIN || $(GOCMD) env GOPATH)/bin/$(BINARY_NAME)$(RESET)\n"

dev: tidy build ## Tidy, build — quick iteration loop

clean: ## Remove build artifacts
	$(INFO)
	@printf "Cleaning build artifacts...\n"
	@rm -rf bin/
	@rm -f $(BINARY_NAME)
	@$(GOCLEAN) -cache -testcache 2>/dev/null || true
	@rm -rf coverage.out coverage.html
	$(SUCCESS)
	@printf "$(GREEN)Clean complete$(RESET)\n"

# ═══════════════════════════════════════════════════════════════════════════════
# TESTING
# ═══════════════════════════════════════════════════════════════════════════════

##@ Testing

test: ## Run tests
	$(INFO)
	@printf "Running tests...\n"
	@$(GOTEST) ./... -count=1 2>&1 | while IFS= read -r line; do \
		case "$$line" in \
			ok*) printf "  \033[32m✓\033[0m %s\n" "$$line" ;; \
			FAIL*) printf "  \033[31m✗\033[0m %s\n" "$$line" ;; \
			'?'*) printf "  \033[2m%s\033[0m\n" "$$line" ;; \
			*) printf "    %s\n" "$$line" ;; \
		esac; \
	done

test-v: ## Run tests (verbose)
	$(INFO)
	@printf "Running tests (verbose)...\n"
	@$(GOTEST) -v ./... -count=1

test-cover: ## Run tests with coverage report
	$(INFO)
	@printf "Running tests with coverage...\n"
	@$(GOTEST) ./... -count=1 -coverprofile=coverage.out
	@$(GOCMD) tool cover -func=coverage.out | tail -1 | awk '{printf "  $(SUCCESS)Coverage: $(BOLD)%s$(RESET)\n", $$3}'
	@rm -f coverage.out

# ═══════════════════════════════════════════════════════════════════════════════
# CODE QUALITY
# ═══════════════════════════════════════════════════════════════════════════════

##@ Code Quality

lint: ## Run golangci-lint
	$(INFO)
	@printf "Running linters...\n"
	@$(GOLINT) run ./...
	$(SUCCESS)
	@printf "$(GREEN)Linting passed$(RESET)\n"

vet: ## Run go vet
	$(INFO)
	@printf "Running go vet...\n"
	@$(GOVET) ./...
	$(SUCCESS)
	@printf "$(GREEN)Vet passed$(RESET)\n"

fmt: ## Format code
	$(INFO)
	@printf "Formatting code...\n"
	@$(GOFMT) -w .
	$(SUCCESS)
	@printf "$(GREEN)Code formatted$(RESET)\n"

fmt-check: ## Check formatting (no changes)
	$(INFO)
	@printf "Checking format...\n"
	@test -z "$$($(GOFMT) -l .)" || ($(GOFMT) -l . && printf "  $(ERROR)Files need formatting$(RESET)\n" && exit 1)
	$(SUCCESS)
	@printf "$(GREEN)Format check passed$(RESET)\n"

tidy: ## Tidy and verify dependencies
	$(INFO)
	@printf "Tidying modules...\n"
	@$(GOMOD) tidy
	@$(GOMOD) verify >/dev/null
	$(SUCCESS)
	@printf "$(GREEN)Dependencies tidied$(RESET)\n"

check: fmt-check vet test ## Run all checks (format, vet, test)

# ═══════════════════════════════════════════════════════════════════════════════
# HELP
# ═══════════════════════════════════════════════════════════════════════════════

##@ Help

help: ## Show this help message
	@printf "\n"
	@printf "$(BOLD)$(CYAN)╔════════════════════════════════════════════════════════════════╗$(RESET)\n"
	@printf "$(BOLD)$(CYAN)║$(RESET)                     $(BOLD)TACK$(RESET) - Makefile Help                       $(BOLD)$(CYAN)║$(RESET)\n"
	@printf "$(BOLD)$(CYAN)╚════════════════════════════════════════════════════════════════╝$(RESET)\n"
	@printf "\n"
	@printf "$(BOLD)Usage:$(RESET) make $(CYAN)<target>$(RESET)\n\n"
	@awk 'BEGIN {FS = ":.*##"; section=""} \
		/^##@/ { \
			section=substr($$0, 5); \
			printf "\n$(BOLD)$(YELLOW)%s$(RESET)\n", section \
		} \
		/^[a-zA-Z_-]+:.*?##/ { \
			if (section != "") { \
				printf "  $(CYAN)%-18s$(RESET) %s\n", $$1, $$2 \
			} \
		}' $(MAKEFILE_LIST)
	@printf "\n"
	@printf "$(BOLD)Examples:$(RESET)\n"
	@printf "  make build         $(WHITE)# Build the binary$(RESET)\n"
	@printf "  make test          $(WHITE)# Run all tests$(RESET)\n"
	@printf "  make dev           $(WHITE)# Quick iteration loop$(RESET)\n"
	@printf "\n"
