# Makefile for mailrelay

# Binary name
BINARY_NAME=mailrelay

# Build directories
BUILD_DIR=build
LINUX_DIR=$(BUILD_DIR)/linux_amd64
WINDOWS_DIR=$(BUILD_DIR)/windows_amd64
OSX_DIR=$(BUILD_DIR)/osx_amd64
OPENBSD_DIR=$(BUILD_DIR)/openbsd_amd64
ARM_DIR=$(BUILD_DIR)/linux_arm64

# Default target
.PHONY: all
all: build

# Build for current architecture
.PHONY: build
build:
	go build -o $(BINARY_NAME)

# Run tests
.PHONY: test
test:
	go test ./...

# Build for all supported architectures
.PHONY: buildall
buildall: clean
	@echo "Building for all architectures..."
	@mkdir -p $(LINUX_DIR) $(WINDOWS_DIR) $(OSX_DIR) $(OPENBSD_DIR) $(ARM_DIR)
	@echo "Building Linux AMD64..."
	env GOOS=linux GOARCH=amd64 go build -o $(LINUX_DIR)/$(BINARY_NAME)-linux-amd64
	@echo "Building Windows AMD64..."
	env GOOS=windows GOARCH=amd64 go build -o $(WINDOWS_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@echo "Building macOS AMD64..."
	env GOOS=darwin GOARCH=amd64 go build -o $(OSX_DIR)/$(BINARY_NAME)-osx-amd64
	@echo "Building OpenBSD AMD64..."
	env GOOS=openbsd GOARCH=amd64 go build -o $(OPENBSD_DIR)/$(BINARY_NAME)-openbsd-amd64
	@echo "Building Linux ARM64..."
	env GOOS=linux GOARCH=arm64 go build -o $(ARM_DIR)/$(BINARY_NAME)-linux-arm64
	@echo "Build complete!"

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME)

# Run with test configuration
.PHONY: test-config
test-config: build
	./$(BINARY_NAME) -config=./mailrelay.json -test -sender=test@example.com -rcpt=recipient@example.com

# Check IP address
.PHONY: check-ip
check-ip: build
	./$(BINARY_NAME) -config=./mailrelay.json -checkIP -ip=$(IP)

# Install golangci-lint and run code style checks
.PHONY: check-style
check-style:
	@echo "Installing golangci-lint v1.63.4..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.63.4
	@echo "Running golangci-lint..."
	golangci-lint run

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build      - Build for current architecture"
	@echo "  buildall   - Build for all supported architectures"
	@echo "  test       - Run tests"
	@echo "  check-style - Install golangci-lint and run code style checks"
	@echo "  clean      - Remove build artifacts"
	@echo "  run        - Build and run the application"
	@echo "  test-config - Test configuration with sample email"
	@echo "  check-ip   - Check if IP is allowed (use IP=x.x.x.x)"
	@echo "  help       - Show this help message"