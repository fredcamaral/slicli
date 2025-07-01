# SliCLI Plugin Template

This is a comprehensive template for creating SliCLI plugins. It demonstrates the complete plugin API and provides a production-ready starting point for developing your own plugins.

## Overview

This example plugin demonstrates how to:
- Implement the required `Plugin` interface (`pkg/plugin/api.go`)
- Process content based on language hints and patterns
- Generate HTML output with custom styling and assets
- Handle plugin configuration and validation
- Support multiple rendering modes and content types
- Implement proper error handling and resource management
- Follow SliCLI plugin best practices and security guidelines

## Building the Plugin

Plugins are compiled as Go shared libraries (.so files) using the `buildmode=plugin` flag:

```bash
# Build the plugin
make build

# Run quality checks (formatting, linting, security)
make check

# Test the plugin
make test

# Install to ~/.slicli/plugins/ directory
make install

# Development mode (build + install)
make dev

# Clean build artifacts
make clean
```

### Quality Assurance Pipeline
```bash
# Complete quality check pipeline
make fmt vet lint security vuln test

# Individual checks
make fmt        # Format Go code
make vet        # Static analysis
make lint       # Code linting (requires golangci-lint)
make security   # Security scan (requires gosec)
make vuln       # Vulnerability check (requires govulncheck)
```

## Plugin Structure

```
plugin-template/
├── main.go         # Plugin implementation (implements plugin.Plugin interface)
├── plugin.toml     # Plugin manifest (metadata, requirements, capabilities)
├── Makefile        # Build configuration (targets for build, test, quality checks)
├── README.md       # Documentation (this file)
└── build/          # Build output directory (created during build)
    └── example.so  # Compiled shared library
```

### Key Files

- **`main.go`**: Core plugin implementation with the required `Plugin` interface
- **`plugin.toml`**: Manifest file with metadata, requirements, and configuration options
- **`Makefile`**: Build automation with quality assurance pipeline
- **`build/`**: Output directory for compiled `.so` files

## Usage

Once installed, the plugin will automatically be loaded by slicli. You can use it in your markdown files:

### Example Box

````markdown
```example-box
title: Important Note
This content will be rendered in a styled box
with a title bar and custom styling.
```
````

### Example Highlight

````markdown
```example-highlight
func main() {
    fmt.Println("Hello, World!")
}
```
````

### Default Rendering

````markdown
```example
Any content here will be rendered
with default example styling.
```
````

## Configuration

The plugin can be configured in your SliCLI configuration file (`slicli.toml`):

```toml
[plugins]
enabled = true
directory = ""  # Use default plugin directory
whitelist = ["example"]  # Only load this plugin (optional)

[plugins.example]
enabled = true
style = "dark"  # or "default"
timeout = "5s"
cache_results = true

[plugins.example.options]
max_lines = 1000
custom_css = true
```

### Configuration Options

- **`enabled`**: Enable/disable the plugin (default: `true`)
- **`style`**: Visual style theme (`"default"` or `"dark"`)
- **`timeout`**: Maximum execution time (default: `"5s"`)
- **`cache_results`**: Cache plugin output (default: `true`)
- **`max_lines`**: Maximum lines to process (default: `1000`)
- **`custom_css`**: Include custom CSS assets (default: `true`)

## Developing Your Own Plugin

To create your own plugin:

1. Copy this template directory
2. Rename the plugin in `main.go` (change the `Name()` method)
3. Update `plugin.toml` with your plugin's information
4. Implement your custom processing logic in the `Execute()` method
5. Build and test your plugin

### Required Methods

Your plugin must implement all methods of the `plugin.Plugin` interface (defined in `pkg/plugin/api.go`):

```go
type Plugin interface {
    Name() string                                                      // Unique plugin identifier  
    Version() string                                                   // Semantic version (e.g., "1.0.0")
    Description() string                                              // Human-readable description
    Init(config map[string]interface{}) error                        // Initialize with configuration
    Execute(ctx context.Context, input PluginInput) (PluginOutput, error) // Process content
    Cleanup() error                                                   // Clean up resources
}
```

### Plugin Input/Output Types

```go
type PluginInput struct {
    Content  string                 // Raw content to process (e.g., code, text)
    Language string                 // Content type hint (e.g., "go", "mermaid")
    Options  map[string]interface{} // Plugin-specific options
    Metadata map[string]interface{} // Additional context from presentation
}

type PluginOutput struct {
    HTML     string  // Rendered HTML output
    Assets   []Asset // Additional static assets (CSS, JS, etc.)
    Metadata map[string]interface{} // Output metadata for other plugins
}
```

### Best Practices

1. **Error Handling**: Always check for context cancellation and wrap errors with context
   ```go
   if err := ctx.Err(); err != nil {
       return plugin.PluginOutput{}, fmt.Errorf("context cancelled: %w", err)
   }
   ```

2. **Resource Management**: Clean up resources in `Cleanup()` method
   - Close file handles, network connections, temporary files
   - Release memory-intensive data structures
   - Stop background goroutines

3. **Configuration Validation**: Validate configuration in `Init()` method
   ```go
   func (p *MyPlugin) Init(config map[string]interface{}) error {
       if timeout, ok := config["timeout"].(string); ok {
           if _, err := time.ParseDuration(timeout); err != nil {
               return fmt.Errorf("invalid timeout format: %w", err)
           }
       }
       return nil
   }
   ```

4. **Performance Optimization**: 
   - Be mindful of processing time, especially for large inputs
   - Use streaming for large content when possible
   - Implement caching for expensive operations
   - Set reasonable timeouts and limits

5. **Thread Safety**: Ensure your plugin is thread-safe if `concurrent = true` in `plugin.toml`
   - Use sync.Mutex for shared state
   - Avoid global variables
   - Test with race detection: `go test -race`

6. **Security**: Follow security best practices
   - Validate and sanitize all inputs
   - Avoid executing arbitrary code from user input
   - Use secure defaults for configuration
   - Handle sensitive data appropriately

## Testing

### Automated Testing
```bash
# Run unit tests
make test

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run benchmark tests
go test -bench=. ./...
```

### Manual Testing
1. Build the plugin: `make build`
2. Install the plugin: `make install`
3. Create a test markdown file:
   ```markdown
   # Test Presentation
   
   ```example
   This is test content for the example plugin.
   ```
   ```
4. Run SliCLI: `slicli serve test.md`
5. Verify the plugin output in the browser

### Integration Testing
```bash
# Test plugin loading
slicli serve --log-level debug test.md

# Verify plugin is loaded
slicli plugins list

# Test specific plugin functionality
echo '```example\ntest content\n```' | slicli process --stdin
```

## Debugging

### Plugin Loading Issues
```bash
# Check if the plugin exports the required symbol
nm -D build/example.so | grep Plugin

# Run SliCLI with debug logging
slicli serve --log-level debug presentation.md

# Verify plugin discovery
slicli plugins list --debug

# Check plugin manifest
slicli plugins info example
```

### Development Debugging
```bash
# Build with debug symbols
go build -buildmode=plugin -gcflags="-N -l" -o build/example.so .

# Use dlv for debugging (limited support for plugins)
dlv exec slicli -- serve presentation.md
```

### Performance Profiling
```bash
# Enable pprof in your plugin (development only)
import _ "net/http/pprof"

# Profile CPU usage
go tool pprof http://localhost:6060/debug/pprof/profile

# Profile memory usage
go tool pprof http://localhost:6060/debug/pprof/heap
```

## Common Issues & Solutions

### Plugin Loading
1. **Plugin won't load**: 
   - Ensure you're building with `-buildmode=plugin`
   - Check Go version compatibility with SliCLI
   - Verify the `.so` file is in the correct directory

2. **Symbol not found**: 
   - The plugin must export a variable named `Plugin`
   - Ensure the variable is globally accessible (not inside a function)
   - Check symbol export: `nm -D plugin.so | grep Plugin`

3. **Version mismatch**: 
   - Rebuild the plugin with the same Go version as SliCLI
   - Check `go version` and SliCLI version compatibility
   - Ensure compatible GOOS and GOARCH

4. **OS compatibility**: 
   - Plugins only work on Linux and macOS (not Windows)
   - Use Docker for Windows development: `docker run -v $(pwd):/workspace golang:1.21`

### Runtime Issues
5. **Context timeout**: 
   - Implement proper context cancellation handling
   - Reduce processing time for large inputs
   - Increase timeout in `plugin.toml`

6. **Memory leaks**: 
   - Implement proper cleanup in `Cleanup()` method
   - Use tools like `go tool pprof` to identify leaks
   - Test with `-race` flag

7. **Thread safety**: 
   - Ensure concurrent safety if `concurrent = true`
   - Use appropriate synchronization primitives
   - Test with `go test -race`

## License

This template is provided under the MIT license. Feel free to use it as a starting point for your own plugins.