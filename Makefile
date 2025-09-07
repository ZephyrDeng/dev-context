# Makefile for Frontend News MCP Server

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Project parameters
BINARY_NAME=dev-context
BINARY_UNIX=$(BINARY_NAME)_unix
MAIN_PATH=./cmd/server
PKG_LIST=$$(go list ./... | grep -v /vendor/)

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Docker parameters
DOCKER_IMAGE=dev-context
DOCKER_TAG ?= $(VERSION)

.PHONY: help build test test-coverage clean fmt vet lint deps dev docker-build docker-run

help: ## Display this help message
	@echo "Frontend News MCP Server - Makefile Commands"
	@echo "============================================"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build completed: $(BINARY_NAME)"

build-linux: ## Build for Linux
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) $(MAIN_PATH)
	@echo "Linux build completed: $(BINARY_UNIX)"

build-all: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Multi-platform builds completed in dist/"

##@ Testing

test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	$(GOTEST) -v -coverprofile=coverage/coverage.out ./...
	$(GOCMD) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report generated: coverage/coverage.html"

test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

test-benchmark: ## Run benchmark tests
	@echo "Running benchmark tests..."
	$(GOTEST) -bench=. -benchmem ./...

##@ Code Quality

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Code formatting completed"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "go vet completed"

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run
	@echo "Linting completed"

check: fmt vet lint ## Run all code quality checks

##@ Dependencies

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated"

deps-upgrade: ## Upgrade dependencies
	@echo "Upgrading dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "Dependencies upgraded"

##@ Development

dev: ## Run development server with hot reload
	@echo "Starting development server..."
	@which air > /dev/null || (echo "Installing air for hot reload..." && go install github.com/cosmtrek/air@latest)
	air -c .air.toml || $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH) && ./$(BINARY_NAME)

run: build ## Build and run the server
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

run-debug: ## Run with debug logging
	@echo "Running $(BINARY_NAME) with debug logging..."
	./$(BINARY_NAME) -log-level debug

##@ Docker

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

##@ Cleanup

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -rf dist/
	rm -rf coverage/
	rm -rf tmp/
	@echo "Clean completed"

clean-docker: ## Clean Docker images and containers
	@echo "Cleaning Docker resources..."
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true
	docker system prune -f

##@ Utilities

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

info: ## Show project information
	@echo "Project: Frontend News MCP Server"
	@echo "Binary: $(BINARY_NAME)"
	@echo "Main Path: $(MAIN_PATH)"
	@echo "Go Version: $$(go version)"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development tools installed"

# Default target
all: clean deps check test build ## Run full CI pipeline

# Development workflow
dev-setup: deps install-tools ## Set up development environment
	@echo "Development environment setup completed"