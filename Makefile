.PHONY: build run test test-coverage lint fmt clean tidy install help

# Binary name
BINARY_NAME=opencode-model-selector
BINARY_DIR=bin
BINARY_PATH=$(BINARY_DIR)/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Coverage parameters
COVERAGE_DIR=coverage
COVERAGE_FILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html

# Default target
all: build

# Display help
help:
	@echo "opencode-model-selector - Interactive Model Selector for OpenCode Agents"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build          Build the binary"
	@echo "  run            Run the application"
	@echo "  install        Install binary to GOPATH/bin"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run all tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format code"
	@echo "  tidy           Run go mod tidy"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean          Remove binaries and coverage"

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_PATH) ./cmd
	@echo "Binary built: $(BINARY_PATH)"

# Run the application
run:
	$(GOCMD) run ./cmd

# Run tests without coverage
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE)

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	$(GOLINT) run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Code formatted"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BINARY_DIR)
	@rm -rf $(COVERAGE_DIR)
	$(GOCLEAN)
	@echo "Clean complete"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	@echo "Dependencies tidied"

# Install binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	$(GOCMD) install ./cmd
	@echo "Installation complete"
