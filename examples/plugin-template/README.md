# Example Plugin Template

This is a template for creating slicli plugins. It demonstrates the plugin API and provides a starting point for developing your own plugins.

## Overview

This example plugin shows how to:
- Implement the required `Plugin` interface
- Process content based on language hints
- Generate HTML output with custom styling
- Provide CSS assets
- Handle configuration options
- Support multiple rendering modes

## Building the Plugin

Plugins must be compiled as Go shared libraries (.so files):

```bash
# Build the plugin
make build

# Install to ~/.slicli/plugins/
make install

# Clean build artifacts
make clean
```

## Plugin Structure

```
example-plugin/
├── main.go         # Plugin implementation
├── plugin.toml     # Plugin manifest
├── Makefile        # Build configuration
└── README.md       # Documentation
```

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

The plugin can be configured in your slicli configuration file:

```toml
[plugins.example]
enabled = true
style = "dark"  # or "default"

[plugins.example.options]
max_lines = 1000
```

## Developing Your Own Plugin

To create your own plugin:

1. Copy this template directory
2. Rename the plugin in `main.go` (change the `Name()` method)
3. Update `plugin.toml` with your plugin's information
4. Implement your custom processing logic in the `Execute()` method
5. Build and test your plugin

### Required Methods

Your plugin must implement all methods of the `plugin.Plugin` interface:

- `Name() string` - Unique plugin identifier
- `Version() string` - Semantic version
- `Description() string` - Human-readable description
- `Init(config map[string]interface{}) error` - Initialize with config
- `Execute(ctx context.Context, input PluginInput) (PluginOutput, error)` - Process content
- `Cleanup() error` - Clean up resources

### Best Practices

1. **Error Handling**: Always check for context cancellation
2. **Resource Management**: Clean up resources in the `Cleanup()` method
3. **Configuration**: Validate configuration in `Init()`
4. **Performance**: Be mindful of processing time, especially for large inputs
5. **Thread Safety**: Ensure your plugin is thread-safe if `concurrent = true`

## Testing

You can test your plugin by:

1. Building it with `make build`
2. Creating a test markdown file that uses your plugin
3. Running slicli with your test file
4. Checking the generated output

## Debugging

To debug plugin loading issues:

```bash
# Check if the plugin exports the required symbol
make check

# Run slicli with debug logging
slicli serve --log-level debug presentation.md
```

## Common Issues

1. **Plugin won't load**: Ensure you're building with `-buildmode=plugin`
2. **Symbol not found**: The plugin must export a variable named `Plugin`
3. **Version mismatch**: Rebuild the plugin with the same Go version as slicli
4. **OS compatibility**: Plugins only work on Linux and macOS

## License

This template is provided under the MIT license. Feel free to use it as a starting point for your own plugins.