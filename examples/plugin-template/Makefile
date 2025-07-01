# Makefile for building the example plugin

PLUGIN_NAME = example
PLUGIN_FILE = $(PLUGIN_NAME).so
GO_FILES = main.go

# Build flags
GO_FLAGS = -buildmode=plugin
LDFLAGS = -ldflags="-s -w"

.PHONY: all build clean install test

all: build

build:
	@echo "Building plugin..."
	go build $(GO_FLAGS) $(LDFLAGS) -o $(PLUGIN_FILE) $(GO_FILES)
	@echo "Plugin built: $(PLUGIN_FILE)"

clean:
	@echo "Cleaning..."
	rm -f $(PLUGIN_FILE)

install: build
	@echo "Installing plugin..."
	mkdir -p ~/.slicli/plugins
	cp $(PLUGIN_FILE) ~/.slicli/plugins/
	cp plugin.toml ~/.slicli/plugins/$(PLUGIN_NAME).toml
	@echo "Plugin installed to ~/.slicli/plugins/"

test:
	@echo "Testing plugin..."
	go test -v ./...

# Development build with race detector
dev:
	go build $(GO_FLAGS) -race -o $(PLUGIN_FILE) $(GO_FILES)

# Check plugin compatibility
check:
	@echo "Checking plugin..."
	@go version
	@echo "Plugin file: $(PLUGIN_FILE)"
	@if [ -f $(PLUGIN_FILE) ]; then \
		file $(PLUGIN_FILE); \
		nm -gU $(PLUGIN_FILE) | grep -E "(Plugin|plugin)"; \
	else \
		echo "Plugin not built yet. Run 'make build' first."; \
	fi