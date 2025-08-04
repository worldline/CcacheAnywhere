# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean

# Build directory
BUILD_DIR=bin

# Output binary name
BINARY_NAME = ccache-backend-client
BINARY_HTTP_NAME = ccache-backend-http
BINARY_GS_NAME = ccache-backend-gs

# Source directory for the main application
CMD_DIR = ./cmd/ccache-backend-client

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
	go install $(CMD_DIR)

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)/$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	go test ./...

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