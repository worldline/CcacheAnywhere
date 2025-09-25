# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean

# Build directory
BUILD_DIR=bin

# Output binary name
BINARY_NAME = ccache-backend-client
BINARY_HTTP_NAME = ccache-http-storage
BINARY_GS_NAME = ccache-gs-storage

# Source directory for the main application
CMD_DIR = ./cmd/ccache-backend-client

# install directory
INSTALL_DIR = /usr/local/libexec/ccache

# Default target
.PHONY: all
all: build

# Build the application
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_GS_NAME)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_HTTP_NAME)

# Install the binary (to GOPATH/bin or GOBIN)
.PHONY: install
install:
	make build
	cp $(BUILD_DIR)/$(BINARY_GS_NAME) /usr/local/libexec/ccache
	cp $(BUILD_DIR)/$(BINARY_HTTP_NAME) /usr/local/libexec/ccache

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/$(BINARY_NAME)

# Run the built binary
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Help message
.PHONY: help
help:
	@echo "Makefile commands:"
	@echo "  build     Build the application"
	@echo "  install   Install the binary to GOPATH/bin or GOBIN"
	@echo "  clean     Remove build artifacts"
	@echo "  test      Run tests"
	@echo "  run       Build and run the application"

# Everything around testing

# Run all tests
.PHONY: test
test:
	go test ./...

.PHONY: test-unit test-integration test-bench test-coverage

test-unit:
	go test -v ./internal/...

# Integrations only
test-integration:
	go test -v -tags=integration ./test/integration/...

# Benchmarks
test-bench:
	go test -bench=. -benchmem ./internal/...

# With coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
