# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

slicli is a CLI-based slide presentation generator that transforms markdown files into interactive web presentations. It uses a clean architecture pattern with a plugin system for extensibility, built entirely in Go with no client-side compilation requirements.

**Key Architecture**: Clean Architecture (Hexagonal) with Domain-Driven Design
- `cmd/slicli/` - CLI application entry point using Cobra framework
- `internal/domain/` - Core business logic and entities
- `internal/adapters/primary/` - Incoming adapters (HTTP server, CLI)
- `internal/adapters/secondary/` - Outgoing adapters (config, plugins, themes)
- `plugins/` - Extensible plugin system using Go shared objects (.so files)
- `themes/` - Presentation theme system with CSS/JS assets (10+ professional themes included)

## Development Commands

### Core Development Workflow
```bash
# Setup and build everything
make build build-plugins

# Development with live reload
make dev

# Run comprehensive tests
make test test-plugins

# Code quality pipeline (run before commits)
make fmt vet lint

# Complete Go quality validation pipeline
go fmt ./...          # Format code 
go test ./...         # Run all tests
go vet ./...          # Static analysis
gosec ./...           # Security scanning
govulncheck ./...     # Vulnerability checking
staticcheck ./...     # Advanced static analysis

# Generate test coverage report
make coverage

# Clean all build artifacts
make clean
```

### Plugin Development
```bash
# Build specific plugin
cd plugins/[plugin-name] && make build

# Test specific plugin  
cd plugins/[plugin-name] && make test

# Create new plugin from template
cp -r examples/plugin-template plugins/my-plugin
cd plugins/my-plugin && make build
```

### Theme Development
```bash
# List available themes
find themes/ -name "theme.toml" -exec dirname {} \;

# Test theme with example presentation
./bin/slicli serve --theme [theme-name] examples/simple-ppt/presentation.md

# Create new theme
mkdir themes/my-theme
cp themes/default/theme.toml themes/my-theme/
# Edit theme.toml and create style.css
```

### Marketplace Commands
```bash
# List available marketplace items (when marketplace server is available)
./bin/slicli marketplace list

# Search for plugins
./bin/slicli marketplace search [query]

# Install plugin from marketplace
./bin/slicli marketplace install [plugin-id]

# Show plugin information
./bin/slicli marketplace info [plugin-id]
```

### Single Test Execution
```bash
# Run specific test package
go test -v ./internal/domain/services

# Run specific test function
go test -v ./internal/adapters/primary/http -run TestServer

# Run with race detection
go test -race ./...

# Run benchmark tests
go test -bench=. ./internal/adapters/secondary/parser
```

### Docker Development
```bash
# Build Docker image
docker build -t slicli .

# Run with Docker (development)
docker run -p 8080:8080 -v $(pwd):/workspace slicli serve /workspace/examples/simple-ppt/presentation.md

# Docker image includes Chromium for PDF export functionality
```

## Architecture Deep Dive

### Plugin System Architecture
The plugin system is the core differentiator of slicli. Plugins are Go shared objects (.so files) that implement a standardized interface defined in `pkg/plugin/api.go`. This enables type-safe, high-performance extensibility.

**Plugin Lifecycle**: Discovery → Loading → Initialization → Execution → Cleanup
- Plugins cannot be unloaded due to Go runtime limitations
- Each plugin runs in the same process but with isolated configuration
- Plugin execution is currently sequential (performance optimization opportunity)

**Current Plugins**:
- `code-exec`: Execute code blocks in presentations (bash, go, js, python) with safety sandboxing
- `syntax-highlight`: Chroma-based syntax highlighting with 200+ languages and theme support
- `mermaid`: Diagram generation from markdown with lazy-loading CDN integration

### HTTP Server & Security
The HTTP server (`internal/adapters/primary/http/`) implements enterprise-grade security:
- **Security Middleware Stack**: Headers → Rate Limiting → Logging → Recovery
- **Input Sanitization**: BlueMonday HTML sanitization prevents XSS
- **Path Traversal Protection**: Secure file serving with validation
- **CORS Configuration**: Restricted to localhost origins
- **WebSocket Live Reload**: For development workflow with connection pooling

### Domain Services
Core business logic is isolated in domain services:
- **PresentationService**: Main orchestrator for slide generation
- **PluginService**: Plugin discovery, loading, and execution with caching
- **ThemeService**: Theme loading with inheritance and asset management
- **ConfigService**: TOML/YAML configuration with validation
- **LiveReloadService**: File watching and WebSocket notifications

### Theme System Architecture
The theme system supports 10+ professional themes with industry-specific designs:
- **Theme Structure**: TOML configuration + CSS styling + optional assets
- **Categories**: Corporate, Educational, Technical, Creative, Medical, Financial
- **Features**: Responsive design, print support, accessibility compliance
- **Customization**: CSS custom properties, modular organization
- **Examples**: Executive Pro (C-suite), Developer Dark (coding), Academic Research (scholarly)

### Export System
Advanced export capabilities for multiple formats:
- **PDF Export**: Chromium-based PDF generation with print-optimized styling
- **Image Export**: PNG/JPEG slide images via browser automation
- **Web Export**: Static HTML bundle for hosting
- **Container Support**: Docker image includes Chromium runtime for headless export

### Marketplace Infrastructure
Open source marketplace system for community plugins and themes:
- **MarketplaceClient**: HTTP-based marketplace interaction (`internal/adapters/secondary/plugin/marketplace.go`)
- **Community Focus**: All plugins and themes are free and MIT-licensed
- **Analytics**: Community engagement tracking without revenue metrics
- **CLI Integration**: Built-in marketplace commands for discovery and installation

### Performance Characteristics
- **Startup Time**: ~85ms + 20ms per plugin
- **Memory Usage**: ~25MB base + 5MB per plugin
- **Major Bottleneck**: Sequential plugin execution (650ms vs 300ms potential with concurrency)
- **Caching Strategy**: LRU cache for themes with TTL, plugin results cached in memory

## Critical Development Patterns

### Error Handling
slicli follows Go error handling best practices:
- All functions that can fail return explicit errors
- Errors are wrapped with context using `fmt.Errorf("context: %w", err)`
- HTTP errors are sanitized to prevent information disclosure
- Plugin errors are isolated and don't crash the main application

### Security-First Development
Recent security audit established these patterns:
- All user input must be sanitized before processing
- File paths require validation to prevent traversal attacks
- Rate limiting is applied to all HTTP endpoints
- Security headers are mandatory for all responses

### Plugin Interface Implementation
When creating plugins, implement the standardized interface:
```go
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Init(config map[string]interface{}) error
    Execute(ctx context.Context, input PluginInput) (PluginOutput, error)
    Cleanup() error
}
```

### Configuration Management
Configuration follows a hierarchical merge pattern:
1. Default values (`internal/adapters/secondary/config/defaults.go`)
2. Configuration file (TOML/YAML)
3. Command-line flags
4. Environment variables (highest priority)

## Business Context & Open Source Model

slicli targets CLI-comfortable developers who need presentation tools that integrate with their existing workflows. The zero-compilation approach and markdown-first design differentiate it from web-based alternatives like Reveal.js.

**Open Source Model**: Fully open source with MIT-licensed themes and plugins
**Target Community**: CLI developers, educators, researchers, and technical presenters
**Competitive Advantage**: CLI-first workflow integration with extensive theme library and plugin extensibility
**Community Infrastructure**: Plugin marketplace, theme collection, and community analytics (all free)

## Testing Strategy

### Test Organization
- **Unit Tests**: Domain services and entities (`*_test.go`)
- **Integration Tests**: HTTP handlers and plugin loading (`integration_test.go`)
- **Plugin Tests**: Individual plugin functionality (`plugins/*/main_test.go`)
- **Security Tests**: Input validation and sanitization
- **Benchmark Tests**: Performance testing (`benchmark_test.go`)
- **Export Tests**: Browser automation and rendering (`export/*_test.go`)

### Mock Patterns
The codebase uses interface-based dependency injection enabling easy mocking:
- HTTP tests use `MockPresentationService`
- Plugin tests use `MockPluginService`
- All external dependencies have interface abstractions
- Test builders pattern in `internal/test/builders/` for complex test data setup

## Known Technical Debt

1. **Plugin Memory Management**: Go runtime prevents plugin unloading, causing memory retention
2. **Sequential Plugin Execution**: Major performance bottleneck requiring concurrency implementation
3. **Theme Asset Pipeline**: Eager loading causes startup delays, needs lazy loading
4. **Configuration Schema**: Lacks formal validation, relies on runtime error detection

## Performance Optimization Priorities

Based on sequence diagram analysis:
1. **Concurrent Plugin Processing** (50-70% performance improvement)
2. **Lazy Theme Loading** (40-60% startup improvement)  
3. **Plugin Timeout Management** (prevent system hangs)
4. **Adaptive File Watching** (optimize live reload)

## Development Workflow Integration

slicli is designed for integration into existing developer workflows:
- Git-friendly markdown source files
- CI/CD pipeline integration for automated slide generation
- Plugin system enables custom workflow adaptations
- Configuration supports team-shared settings

The codebase prioritizes maintainability and extensibility while delivering a CLI-first user experience that requires zero compilation steps from end users.