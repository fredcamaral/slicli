# Code Execution Plugin Makefile

PLUGIN_NAME = code-exec
BUILD_DIR = build
PLUGIN_FILE = $(BUILD_DIR)/$(PLUGIN_NAME).so

# Go build flags for plugin
GO_BUILD_FLAGS = -buildmode=plugin -ldflags="-s -w"

# Default target
.PHONY: all
all: build

# Build the plugin
.PHONY: build
build: clean
	@echo "Building $(PLUGIN_NAME) plugin..."
	@mkdir -p $(BUILD_DIR)
	go build $(GO_BUILD_FLAGS) -o $(PLUGIN_FILE) .
	@echo "Plugin built: $(PLUGIN_FILE)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning $(PLUGIN_NAME) plugin build artifacts..."
	@rm -rf $(BUILD_DIR)

# Test the plugin
.PHONY: test
test:
	@echo "Testing $(PLUGIN_NAME) plugin..."
	go test -v .

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting $(PLUGIN_NAME) plugin code..."
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting $(PLUGIN_NAME) plugin code..."
	golangci-lint run ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting $(PLUGIN_NAME) plugin code..."
	go vet ./...

# Security scan
.PHONY: security
security:
	@echo "Running security scan on $(PLUGIN_NAME) plugin..."
	gosec ./...

# Vulnerability check
.PHONY: vuln
vuln:
	@echo "Checking vulnerabilities in $(PLUGIN_NAME) plugin..."
	govulncheck ./...

# Full quality check
.PHONY: check
check: fmt vet lint security vuln test
	@echo "Quality check completed for $(PLUGIN_NAME) plugin"

# Install plugin to system
.PHONY: install
install: build
	@echo "Installing $(PLUGIN_NAME) plugin..."
	@mkdir -p ~/.slicli/plugins
	@cp $(PLUGIN_FILE) ~/.slicli/plugins/
	@echo "Plugin installed to ~/.slicli/plugins/"

# Development mode - build and install
.PHONY: dev
dev: build install
	@echo "$(PLUGIN_NAME) plugin ready for development"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build     - Build the plugin"
	@echo "  clean     - Clean build artifacts"
	@echo "  test      - Run tests"
	@echo "  fmt       - Format code"
	@echo "  lint      - Lint code"
	@echo "  vet       - Vet code"
	@echo "  security  - Run security scan"
	@echo "  vuln      - Check vulnerabilities"
	@echo "  check     - Run full quality check"
	@echo "  install   - Install plugin to system"
	@echo "  dev       - Build and install for development"
	@echo "  help      - Show this help"