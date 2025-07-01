package ports

import (
	"context"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/pkg/plugin"
)

// PluginLoader handles the discovery and loading of plugins.
type PluginLoader interface {
	// Discover finds all available plugins in the given directories.
	Discover(ctx context.Context, dirs []string) ([]plugin.PluginInfo, error)

	// Load loads a plugin from the given path.
	Load(ctx context.Context, path string) (plugin.Plugin, error)

	// Unload unloads a plugin and releases its resources.
	Unload(ctx context.Context, name string) error

	// Validate checks if a plugin is valid and compatible.
	Validate(p plugin.Plugin) error

	// LoadManifest loads a plugin manifest from a file.
	LoadManifest(ctx context.Context, path string) (*entities.PluginManifest, error)
}

// PluginExecutor handles the safe execution of plugins.
type PluginExecutor interface {
	// Execute runs a plugin with the default timeout.
	Execute(ctx context.Context, p plugin.Plugin, input plugin.PluginInput) (plugin.PluginOutput, error)

	// ExecuteWithTimeout runs a plugin with a specific timeout.
	ExecuteWithTimeout(ctx context.Context, p plugin.Plugin, input plugin.PluginInput, timeout time.Duration) (plugin.PluginOutput, error)

	// ExecuteAsync runs a plugin asynchronously and returns a channel for the result.
	ExecuteAsync(ctx context.Context, p plugin.Plugin, input plugin.PluginInput) <-chan PluginResult
}

// PluginResult represents the result of an asynchronous plugin execution.
type PluginResult struct {
	Output plugin.PluginOutput
	Error  error
}

// PluginRegistry manages loaded plugins.
type PluginRegistry interface {
	// Register adds a plugin to the registry.
	Register(name string, p plugin.Plugin, metadata entities.PluginMetadata) error

	// Get retrieves a plugin by name.
	Get(name string) (plugin.Plugin, bool)

	// GetAll returns all registered plugins.
	GetAll() map[string]plugin.Plugin

	// GetByType returns all plugins of a specific type.
	GetByType(pluginType entities.PluginType) []plugin.Plugin

	// Remove removes a plugin from the registry.
	Remove(name string) error

	// Clear removes all plugins from the registry.
	Clear()

	// GetMetadata returns metadata for a plugin.
	GetMetadata(name string) (*entities.PluginMetadata, bool)

	// GetStatistics returns statistics for a plugin.
	GetStatistics(name string) (*entities.PluginStatistics, bool)

	// UpdateStatistics updates statistics for a plugin.
	UpdateStatistics(name string, duration time.Duration, success bool, bytesIn, bytesOut int64)

	// IncrementTimeout increments the timeout counter for a plugin.
	IncrementTimeout(name string)

	// IncrementPanic increments the panic counter for a plugin.
	IncrementPanic(name string)

	// GetLoadedPlugin returns detailed information about a loaded plugin.
	GetLoadedPlugin(name string) (*entities.LoadedPlugin, error)

	// ListLoadedPlugins returns information about all loaded plugins.
	ListLoadedPlugins() []entities.LoadedPlugin

	// SetPluginStatus sets the status of a plugin.
	SetPluginStatus(name string, status entities.PluginStatus, errorMsg string) error
}

// PluginCache caches plugin execution results.
type PluginCache interface {
	// Get retrieves a cached result.
	Get(key string) (*plugin.PluginOutput, bool)

	// Set stores a result in the cache.
	Set(key string, output *plugin.PluginOutput, ttl time.Duration)

	// Remove removes a result from the cache.
	Remove(key string)

	// Clear removes all results from the cache.
	Clear()

	// Stats returns cache statistics.
	Stats() entities.CacheStats
}

// PluginService orchestrates plugin operations.
type PluginService interface {
	// LoadPlugin loads a plugin from a path.
	LoadPlugin(ctx context.Context, path string) error

	// UnloadPlugin unloads a plugin by name.
	UnloadPlugin(ctx context.Context, name string) error

	// DiscoverPlugins discovers plugins in configured directories.
	DiscoverPlugins(ctx context.Context) ([]plugin.PluginInfo, error)

	// ExecutePlugin executes a plugin by name.
	ExecutePlugin(ctx context.Context, name string, input plugin.PluginInput) (plugin.PluginOutput, error)

	// GetPlugin retrieves a plugin by name.
	GetPlugin(name string) (plugin.Plugin, error)

	// GetPluginInfo returns information about a plugin.
	GetPluginInfo(name string) (*entities.LoadedPlugin, error)

	// ListPlugins returns information about all loaded plugins.
	ListPlugins() []entities.LoadedPlugin

	// ProcessContent processes content using matching plugins.
	ProcessContent(ctx context.Context, content string, language string) ([]plugin.PluginOutput, error)

	// Shutdown gracefully shuts down the plugin service.
	Shutdown(ctx context.Context) error
}

// PluginMatcher determines which plugins should process given content.
type PluginMatcher interface {
	// Match returns plugins that should process the given content.
	Match(content string, language string, metadata map[string]interface{}) []string

	// MatchByType returns plugins of a specific type that match the content.
	MatchByType(content string, pluginType entities.PluginType) []string

	// AddRule adds a matching rule.
	AddRule(pluginName string, rule MatchRule)

	// RemoveRule removes a matching rule.
	RemoveRule(pluginName string, ruleID string)
}

// MatchRule defines a rule for matching content to plugins.
type MatchRule struct {
	ID          string
	Priority    int
	Language    string
	Pattern     string
	FileExt     string
	ContentType string
}
