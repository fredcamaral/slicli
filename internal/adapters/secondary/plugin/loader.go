package plugin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// GoPluginLoader loads Go plugins from .so files.
type GoPluginLoader struct {
	mu       sync.RWMutex
	loaded   map[string]*loadedPlugin
	version  string
	platform string
}

type loadedPlugin struct {
	plugin   pluginapi.Plugin
	handle   *plugin.Plugin
	path     string
	loadedAt time.Time
}

// NewGoPluginLoader creates a new Go plugin loader.
func NewGoPluginLoader(slicliVersion string) *GoPluginLoader {
	return &GoPluginLoader{
		loaded:   make(map[string]*loadedPlugin),
		version:  slicliVersion,
		platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// Discover finds all available plugins in the given directories.
func (l *GoPluginLoader) Discover(ctx context.Context, dirs []string) ([]pluginapi.PluginInfo, error) {
	var plugins []pluginapi.PluginInfo

	for _, dir := range dirs {
		// Check if directory exists
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent directories
			}
			return nil, fmt.Errorf("checking directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			continue
		}

		// Walk directory looking for .so files
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error accessing path %s: %v", path, err)
				return nil // Continue walking
			}

			// Check if it's a .so file
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".so") {
				// Try to load plugin info
				manifest, err := l.LoadManifest(ctx, filepath.Join(filepath.Dir(path), "plugin.toml"))
				if err != nil {
					log.Printf("No manifest for plugin %s: %v", path, err)
					// Try to load the plugin to get basic info
					p, err := l.Load(ctx, path)
					if err != nil {
						log.Printf("Failed to load plugin %s: %v", path, err)
						return nil
					}
					plugins = append(plugins, pluginapi.PluginInfo{
						Name:        p.Name(),
						Version:     p.Version(),
						Description: p.Description(),
						Path:        path,
						Compatible:  true, // Assume compatible if it loads
					})
				} else {
					// Use manifest info
					compatible := manifest.Requirements.IsCompatible(l.version, runtime.GOOS, runtime.GOARCH)
					plugins = append(plugins, pluginapi.PluginInfo{
						Name:        manifest.Metadata.Name,
						Version:     manifest.Metadata.Version,
						Description: manifest.Metadata.Description,
						Path:        path,
						Compatible:  compatible,
					})
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("walking directory %s: %w", dir, err)
		}
	}

	return plugins, nil
}

// Load loads a plugin from the given path.
func (l *GoPluginLoader) Load(ctx context.Context, path string) (pluginapi.Plugin, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if already loaded
	if loaded, exists := l.loaded[path]; exists {
		return loaded.plugin, nil
	}

	// Validate file exists and is a .so file
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("plugin file not found: %w", err)
	}
	if !strings.HasSuffix(info.Name(), ".so") {
		return nil, errors.New("plugin file must have .so extension")
	}

	// Open the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening plugin: %w", err)
	}

	// Look for the Plugin symbol
	symPlugin, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin symbol not found: %w", err)
	}

	// Cast to plugin interface
	pluginInstance, ok := symPlugin.(pluginapi.Plugin)
	if !ok {
		// Try as a pointer
		pluginPtr, ok := symPlugin.(*pluginapi.Plugin)
		if !ok {
			return nil, errors.New("plugin does not implement the Plugin interface")
		}
		pluginInstance = *pluginPtr
	}

	// Validate the plugin
	if err := l.Validate(pluginInstance); err != nil {
		return nil, fmt.Errorf("plugin validation failed: %w", err)
	}

	// Store the loaded plugin
	l.loaded[path] = &loadedPlugin{
		plugin:   pluginInstance,
		handle:   p,
		path:     path,
		loadedAt: time.Now(),
	}

	return pluginInstance, nil
}

// Unload unloads a plugin and releases its resources.
func (l *GoPluginLoader) Unload(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Find the plugin by name
	var pathToRemove string
	for path, loaded := range l.loaded {
		if loaded.plugin.Name() == name {
			pathToRemove = path
			break
		}
	}

	if pathToRemove == "" {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Call cleanup on the plugin
	loaded := l.loaded[pathToRemove]
	if err := loaded.plugin.Cleanup(); err != nil {
		log.Printf("Error cleaning up plugin %s: %v", name, err)
	}

	// Remove from loaded map
	delete(l.loaded, pathToRemove)

	// Note: Go doesn't support unloading plugins, so the .so remains in memory
	// This is a limitation of the Go plugin system

	return nil
}

// Validate checks if a plugin is valid and compatible.
func (l *GoPluginLoader) Validate(p pluginapi.Plugin) error {
	// Check required methods return non-empty values
	if p.Name() == "" {
		return errors.New("plugin name cannot be empty")
	}
	if p.Version() == "" {
		return errors.New("plugin version cannot be empty")
	}
	if p.Description() == "" {
		return errors.New("plugin description cannot be empty")
	}

	// Validate name format
	if !isValidPluginName(p.Name()) {
		return fmt.Errorf("invalid plugin name: %s (must contain only alphanumeric characters, hyphens, and underscores)", p.Name())
	}

	// Validate version format
	if !isValidVersion(p.Version()) {
		return fmt.Errorf("invalid plugin version: %s (must follow semantic versioning)", p.Version())
	}

	return nil
}

// LoadManifest loads a plugin manifest from a file.
func (l *GoPluginLoader) LoadManifest(ctx context.Context, path string) (*entities.PluginManifest, error) {
	data, err := os.ReadFile(path) // #nosec G304 - path from controlled plugin directory scan
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest entities.PluginManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Validate the manifest
	if err := manifest.Metadata.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// isValidPluginName checks if a plugin name is valid.
func isValidPluginName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// isValidVersion checks if a version string follows semantic versioning.
func isValidVersion(version string) bool {
	// Simple semantic version check
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return false
	}
	for i, part := range parts {
		// Handle pre-release versions
		if i == 2 && strings.Contains(part, "-") {
			part = strings.Split(part, "-")[0]
		}
		// Check if it's a number
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
