.PHONY: build test test-coverage check-coverage bench bench-compare clean install static-lint lint-fix install-hooks mod-tidy help

# Default target
.DEFAULT_GOAL := help

# Coverage threshold (percentage, integer)
COVERAGE_THRESHOLD ?= 100

# Binary name
BINARY_NAME=colref
BUILD_DIR=./cmd/colref

# Go parameters
GOCMD=go
GOLINT=golangci-lint
LINT_CONFIG=--config=./.golangci.yml
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOINSTALL=$(GOCMD) install
GOMOD=$(GOCMD) mod
GOCLEAN=$(GOCMD) clean

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) $(BUILD_DIR)

test: ## Run unit tests
	@echo "Running tests..."
	$(GOTEST) -race ./... -v

test-coverage: ## Run tests with coverage (report only)
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	$(GOTEST) ./... -coverprofile=coverage/coverage.out
	$(GOCMD) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

check-coverage: ## Run tests with coverage and enforce minimum threshold
	@echo "Running tests with coverage (threshold: $(COVERAGE_THRESHOLD)%)..."
	@coverage_file=$$(mktemp); \
	trap 'rm -f "$$coverage_file"' EXIT; \
	$(GOTEST) ./... -coverprofile="$$coverage_file"; \
	total=$$($(GOCMD) tool cover -func="$$coverage_file" | grep '^total' | awk '{print $$3}' | tr -d '%'); \
	echo "Total coverage: $${total}%"; \
	if ! awk "BEGIN { exit !($$total >= $(COVERAGE_THRESHOLD)) }"; then \
		echo "FAIL: coverage $${total}% is below threshold $(COVERAGE_THRESHOLD)%"; exit 1; \
	fi; \
	echo "Coverage OK."

bench: ## Run benchmark tests
	@echo "Running benchmark tests..."
	$(GOTEST) -bench=. -benchmem ./... -run=^$$

bench-compare: ## Compare benchmarks against origin/main; blocks on ⚠️ +10%+ regression
	@bash scripts/bench-compare.sh

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf coverage

install: ## Install the binary locally
	@echo "Installing $(BINARY_NAME)..."
	$(GOINSTALL) $(BUILD_DIR)

static-lint: ## Run golangci-lint for static analysis
	@echo "Running golangci-lint..."
	$(GOLINT) run $(LINT_CONFIG)

lint-fix: ## Run golangci-lint and fix issues automatically
	@echo "Running golangci-lint fix..."
	$(GOLINT) run $(LINT_CONFIG) --fix

install-hooks: ## Install git hooks (pre-push)
	@echo "Installing git hooks..."
	@HOOKS_DIR=$$(git rev-parse --git-path hooks); \
	mkdir -p "$$HOOKS_DIR"; \
	cp scripts/pre-push "$$HOOKS_DIR/pre-push"; \
	chmod +x "$$HOOKS_DIR/pre-push"; \
	echo "pre-push hook installed to $$HOOKS_DIR/pre-push."

mod-tidy: ## Tidy go.mod and go.sum
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
