.PHONY: all fmt vet test coverage lint build clean

# Default target
all: fmt vet test

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: brew install golangci-lint"; \
	fi

# Build the binary
build:
	@echo "Building slicli..."
	@mkdir -p bin
	@go build -ldflags="-s -w" -o bin/slicli ./cmd/slicli

# Build for all platforms
build-all-platforms:
	@echo "Building for all platforms..."
	@mkdir -p dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then \
				continue; \
			fi; \
			echo "Building for $$os/$$arch..."; \
			output="dist/slicli-$$os-$$arch"; \
			if [ "$$os" = "windows" ]; then \
				output="$$output.exe"; \
			fi; \
			GOOS=$$os GOARCH=$$arch go build -ldflags="-s -w" -o "$$output" ./cmd/slicli; \
		done \
	done

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/ coverage.out coverage.html *.test
	@go clean -testcache
	@for plugin in plugins/*; do \
		if [ -d "$$plugin" ] && [ -f "$$plugin/Makefile" ]; then \
			$(MAKE) -C $$plugin clean; \
		fi \
	done

# Build plugins
build-plugins:
	@echo "Building plugins..."
	@for plugin in plugins/*; do \
		if [ -d "$$plugin" ] && [ -f "$$plugin/Makefile" ]; then \
			echo "Building $$plugin..."; \
			$(MAKE) -C $$plugin build || exit 1; \
		fi \
	done

# Test plugins
test-plugins:
	@echo "Testing plugins..."
	@for plugin in plugins/*; do \
		if [ -d "$$plugin" ] && [ -f "$$plugin/Makefile" ]; then \
			echo "Testing $$plugin..."; \
			$(MAKE) -C $$plugin test || exit 1; \
		fi \
	done

# Install slicli and plugins
install: build build-plugins
	@echo "Installing slicli..."
	@sudo cp bin/slicli /usr/local/bin/
	@echo "Installing plugins..."
	@mkdir -p ~/.config/slicli/plugins
	@for plugin in plugins/*; do \
		if [ -d "$$plugin" ] && [ -f "$$plugin/Makefile" ]; then \
			$(MAKE) -C $$plugin install || exit 1; \
		fi \
	done

# Run example
run: build
	@./bin/slicli serve examples/simple-ppt/presentation.md

# Development mode with live reload
dev: build
	@./bin/slicli serve examples/simple-ppt/presentation.md

# Create release packages
release: build build-plugins
	@echo "Creating release packages..."
	@./scripts/create-release.sh

# Show help
help:
	@echo "Available commands:"
	@echo "  make build              - Build the slicli binary"
	@echo "  make build-all-platforms- Build for all platforms"
	@echo "  make test               - Run all tests"
	@echo "  make test-plugins       - Test all plugins"
	@echo "  make build-plugins      - Build all plugins"
	@echo "  make install            - Install slicli and plugins"
	@echo "  make run                - Run slicli with example.md"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make dev                - Run with live reload"
	@echo "  make lint               - Run linters"
	@echo "  make fmt                - Format code"
	@echo "  make coverage           - Generate test coverage report"
	@echo "  make release            - Create release packages"