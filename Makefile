# Makefile for sonoserve
# Inspired by github.com/holos-run/holos

# Project variables
PROJECT_NAME := sonoserve
BINARY_NAME := sonoserve
MAIN_PACKAGE := .

# Version and build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_TREE_STATE := $(shell if [ -z "`git status --porcelain`" ]; then echo "clean"; else echo "dirty"; fi)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS := -X main.version=$(VERSION) \
           -X main.gitCommit=$(GIT_COMMIT) \
           -X main.gitTreeState=$(GIT_TREE_STATE) \
           -X main.buildDate=$(BUILD_DATE)

# Build configuration
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Directories
BUILD_DIR := build
DIST_DIR := dist
WEBSITE_DIR := website

# Tools
GOLANGCI_LINT_VERSION := v1.54.2

.PHONY: help
help: ## Show this help message
	@echo "$(PROJECT_NAME) - Sonos speaker controller"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: generate $(BUILD_DIR) ## Build the executable
	@echo "Building $(BINARY_NAME) for $(OS)/$(ARCH)..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(OS) GOARCH=$(ARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		$(MAIN_PACKAGE)

.PHONY: debug
debug: $(BUILD_DIR) ## Build debug version with race detection
	@echo "Building debug version..."
	go build -race -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(MAIN_PACKAGE)

.PHONY: install
install: ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install -ldflags "$(LDFLAGS)" $(MAIN_PACKAGE)

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	rm -f $(BINARY_NAME) $(BINARY_NAME)-debug

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	go test -race -v ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping..."; \
		echo "Install with: make tools"; \
	fi

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting Go code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

.PHONY: mod
mod: ## Tidy and verify go modules
	@echo "Tidying go modules..."
	go mod tidy
	go mod verify

.PHONY: generate
generate: website-build ## Run go generate
	@echo "Running go generate..."
	go generate ./...

.PHONY: website-build
website-build: ## Build the Docusaurus website
	@echo "Building Docusaurus website..."
	cd $(WEBSITE_DIR) && npm run build

.PHONY: website-dev
website-dev: ## Start Docusaurus development server
	@echo "Starting Docusaurus development server..."
	cd $(WEBSITE_DIR) && npm start

.PHONY: website-install
website-install: ## Install website dependencies
	@echo "Installing website dependencies..."
	cd $(WEBSITE_DIR) && npm install

.PHONY: run
run: build ## Build and run the server
	@echo "Starting $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

.PHONY: dev
dev: ## Run in development mode (without building)
	@echo "Running in development mode..."
	go run main.go ui.go

.PHONY: tools
tools: ## Install development tools
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	fi

.PHONY: cross-compile
cross-compile: ## Build for multiple platforms
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 make build && mv $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=amd64 make build && mv $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 make build && mv $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64
	GOOS=windows GOARCH=amd64 make build && mv $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(PROJECT_NAME):$(VERSION) .

.PHONY: check
check: lint test ## Run all checks (lint + test)

.PHONY: ci
ci: mod fmt lint test ## Run CI pipeline locally

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Tree State: $(GIT_TREE_STATE)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Go Version: $(shell go version)"
	@echo "OS/Arch: $(OS)/$(ARCH)"

.PHONY: deps
deps: mod website-install ## Install all dependencies

.PHONY: all
all: clean deps generate check build ## Run a complete build pipeline

# Create build directory
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

