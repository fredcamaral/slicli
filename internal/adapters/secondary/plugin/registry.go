package plugin

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// InMemoryRegistry is an in-memory implementation of the plugin registry.
type InMemoryRegistry struct {
	mu         sync.RWMutex
	plugins    map[string]pluginapi.Plugin
	metadata   map[string]entities.PluginMetadata
	statistics map[string]*entities.PluginStatistics
	loaded     map[string]*entities.LoadedPlugin
}

// NewInMemoryRegistry creates a new in-memory plugin registry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		plugins:    make(map[string]pluginapi.Plugin),
		metadata:   make(map[string]entities.PluginMetadata),
		statistics: make(map[string]*entities.PluginStatistics),
		loaded:     make(map[string]*entities.LoadedPlugin),
	}
}

// Register adds a plugin to the registry.
func (r *InMemoryRegistry) Register(name string, p pluginapi.Plugin, metadata entities.PluginMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	// Check if plugin already exists
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	// Register the plugin
	r.plugins[name] = p
	r.metadata[name] = metadata
	r.statistics[name] = &entities.PluginStatistics{}
	r.loaded[name] = &entities.LoadedPlugin{
		Metadata:   metadata,
		Status:     entities.PluginStatusLoaded,
		LoadedAt:   time.Now(),
		Statistics: *r.statistics[name],
	}

	return nil
}

// Get retrieves a plugin by name.
func (r *InMemoryRegistry) Get(name string) (pluginapi.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[name]
	return p, exists
}

// GetAll returns all registered plugins.
func (r *InMemoryRegistry) GetAll() map[string]pluginapi.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]pluginapi.Plugin, len(r.plugins))
	for k, v := range r.plugins {
		result[k] = v
	}
	return result
}

// GetByType returns all plugins of a specific type.
func (r *InMemoryRegistry) GetByType(pluginType entities.PluginType) []pluginapi.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []pluginapi.Plugin
	for name, p := range r.plugins {
		if metadata, exists := r.metadata[name]; exists && metadata.Type == pluginType {
			result = append(result, p)
		}
	}
	return result
}

// Remove removes a plugin from the registry.
func (r *InMemoryRegistry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Remove all related data
	delete(r.plugins, name)
	delete(r.metadata, name)
	delete(r.statistics, name)
	delete(r.loaded, name)

	return nil
}

// Clear removes all plugins from the registry.
func (r *InMemoryRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]pluginapi.Plugin)
	r.metadata = make(map[string]entities.PluginMetadata)
	r.statistics = make(map[string]*entities.PluginStatistics)
	r.loaded = make(map[string]*entities.LoadedPlugin)
}

// GetMetadata returns metadata for a plugin.
func (r *InMemoryRegistry) GetMetadata(name string) (*entities.PluginMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid mutations
	copy := metadata
	return &copy, true
}

// GetStatistics returns statistics for a plugin.
func (r *InMemoryRegistry) GetStatistics(name string) (*entities.PluginStatistics, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, exists := r.statistics[name]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid mutations
	copy := *stats
	return &copy, true
}

// UpdateStatistics updates statistics for a plugin.
func (r *InMemoryRegistry) UpdateStatistics(name string, duration time.Duration, success bool, bytesIn, bytesOut int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, exists := r.statistics[name]
	if !exists {
		return
	}

	// Update statistics
	stats.ExecutionCount++
	stats.TotalDuration += duration
	stats.AverageDuration = stats.TotalDuration / time.Duration(stats.ExecutionCount)
	stats.LastExecuted = time.Now()
	stats.BytesProcessed += bytesIn
	stats.BytesGenerated += bytesOut

	if success {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}

	// Update loaded plugin info
	if loaded, exists := r.loaded[name]; exists {
		loaded.LastUsed = time.Now()
		loaded.Statistics = *stats
		if success {
			loaded.Status = entities.PluginStatusActive
		}
	}
}

// IncrementTimeout increments the timeout counter for a plugin.
func (r *InMemoryRegistry) IncrementTimeout(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stats, exists := r.statistics[name]; exists {
		stats.TimeoutCount++
		stats.ErrorCount++
	}

	if loaded, exists := r.loaded[name]; exists {
		loaded.Statistics.TimeoutCount++
		loaded.Statistics.ErrorCount++
	}
}

// IncrementPanic increments the panic counter for a plugin.
func (r *InMemoryRegistry) IncrementPanic(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stats, exists := r.statistics[name]; exists {
		stats.PanicCount++
		stats.ErrorCount++
	}

	if loaded, exists := r.loaded[name]; exists {
		loaded.Statistics.PanicCount++
		loaded.Statistics.ErrorCount++
		loaded.Status = entities.PluginStatusError
	}
}

// GetLoadedPlugin returns detailed information about a loaded plugin.
func (r *InMemoryRegistry) GetLoadedPlugin(name string) (*entities.LoadedPlugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	loaded, exists := r.loaded[name]
	if !exists {
		return nil, errors.New("plugin not found")
	}

	// Return a copy
	copy := *loaded
	return &copy, nil
}

// ListLoadedPlugins returns information about all loaded plugins.
func (r *InMemoryRegistry) ListLoadedPlugins() []entities.LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]entities.LoadedPlugin, 0, len(r.loaded))
	for _, loaded := range r.loaded {
		result = append(result, *loaded)
	}
	return result
}

// SetPluginStatus updates the status of a plugin.
func (r *InMemoryRegistry) SetPluginStatus(name string, status entities.PluginStatus, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	loaded, exists := r.loaded[name]
	if !exists {
		return errors.New("plugin not found")
	}

	loaded.Status = status
	if errorMsg != "" {
		loaded.ErrorMsg = errorMsg
	}

	return nil
}
