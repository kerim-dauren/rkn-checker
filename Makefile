.PHONY: help build clean deps fmt vet lint check run test test-unit test-integration test-race docker-build docker-run

# Go tool invocations
GOCMD      := go
GOBUILD    := $(GOCMD) build
GOCLEAN    := $(GOCMD) clean
GOTEST     := $(GOCMD) test
GOMOD      := $(GOCMD) mod
GOFMT      := $(GOCMD) fmt
GOVET      := $(GOCMD) vet

# Build settings
BINARY_NAME := rkn-checker
BUILD_DIR   := build
MAIN_PATH   := ./cmd/app

# Test settings
TEST_TIMEOUT := 30s

# Docker settings
DOCKER_IMAGE := rkn-checker
DOCKER_TAG   := latest

help: ## Display available commands
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build               Build the application binary"
	@echo "  clean               Remove build artifacts"
	@echo "  deps                Download and verify dependencies"
	@echo "  fmt                 Format source code"
	@echo "  vet                 Run go vet"
	@echo "  lint                Run lint checks"
	@echo "  check               Run fmt, vet, and lint"
	@echo "  run                 Build and run the application"
	@echo "  test                Run unit and integration tests"
	@echo "  test-unit           Run unit tests only"
	@echo "  test-integration    Run integration tests only"
	@echo "  test-race           Run tests with race detector"
	@echo "  docker-build        Build the Docker image"
	@echo "  docker-run          Run the Docker container"

build: deps ## Build the application
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)

clean: ## Remove build artifacts
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

deps: ## Download and verify dependencies
	@$(GOMOD) download
	@$(GOMOD) verify
	@$(GOMOD) tidy

fmt: ## Format source code
	@$(GOFMT) ./...

vet: ## Static analysis
	@$(GOVET) ./...

lint: ## Lint code
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  golangci-lint run; \
	else \
	  echo "golangci-lint not found"; \
	fi

check: fmt vet lint ## Run all checks

run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

test: test-unit test-integration

test-unit: ## Run unit tests
	@$(GOTEST) -timeout $(TEST_TIMEOUT) -v \
	  $(shell go list ./... | grep -v /test/integration)

test-integration: ## Run integration tests
	@$(GOTEST) -timeout $(TEST_TIMEOUT) -v ./test/integration/...

test-race: ## Run tests with race detector
	@$(GOTEST) -race -timeout $(TEST_TIMEOUT) ./...

docker-build: ## Build Docker image
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run: ## Run Docker container
	@docker run -p 9090:9090 -p 80:80 $(DOCKER_IMAGE):$(DOCKER_TAG)