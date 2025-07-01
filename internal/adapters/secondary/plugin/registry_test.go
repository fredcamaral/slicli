package plugin

import (
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestMetadata(name string, pluginType entities.PluginType) entities.PluginMetadata {
	return entities.PluginMetadata{
		Name:        name,
		Version:     "1.0.0",
		Description: "Test plugin",
		Type:        pluginType,
	}
}

func TestInMemoryRegistry_Register(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{
		name:        "test-plugin",
		version:     "1.0.0",
		description: "Test plugin",
	}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)

	// Register plugin
	err := registry.Register("test-plugin", plugin, metadata)
	require.NoError(t, err)

	// Verify plugin is registered
	p, exists := registry.Get("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, plugin, p)

	// Try to register same plugin again
	err = registry.Register("test-plugin", plugin, metadata)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestInMemoryRegistry_RegisterInvalidMetadata(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{
		name:        "test-plugin",
		version:     "1.0.0",
		description: "Test plugin",
	}

	// Invalid metadata (missing name)
	metadata := entities.PluginMetadata{
		Version: "1.0.0",
		Type:    entities.PluginTypeProcessor,
	}

	err := registry.Register("test-plugin", plugin, metadata)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata")
}

func TestInMemoryRegistry_Get(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Get existing plugin
	p, exists := registry.Get("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, plugin, p)

	// Get non-existing plugin
	_, exists = registry.Get("non-existing")
	assert.False(t, exists)
}

func TestInMemoryRegistry_GetAll(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register multiple plugins
	plugin1 := &MockPlugin{name: "plugin1"}
	plugin2 := &MockPlugin{name: "plugin2"}
	metadata1 := createTestMetadata("plugin1", entities.PluginTypeProcessor)
	metadata2 := createTestMetadata("plugin2", entities.PluginTypeExporter)

	_ = registry.Register("plugin1", plugin1, metadata1)
	_ = registry.Register("plugin2", plugin2, metadata2)

	all := registry.GetAll()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "plugin1")
	assert.Contains(t, all, "plugin2")
}

func TestInMemoryRegistry_GetByType(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register plugins of different types
	processor1 := &MockPlugin{name: "processor1"}
	processor2 := &MockPlugin{name: "processor2"}
	exporter := &MockPlugin{name: "exporter"}

	_ = registry.Register("processor1", processor1, createTestMetadata("processor1", entities.PluginTypeProcessor))
	_ = registry.Register("processor2", processor2, createTestMetadata("processor2", entities.PluginTypeProcessor))
	_ = registry.Register("exporter", exporter, createTestMetadata("exporter", entities.PluginTypeExporter))

	// Get processors
	processors := registry.GetByType(entities.PluginTypeProcessor)
	assert.Len(t, processors, 2)

	// Get exporters
	exporters := registry.GetByType(entities.PluginTypeExporter)
	assert.Len(t, exporters, 1)

	// Get non-existing type
	themes := registry.GetByType(entities.PluginTypeTheme)
	assert.Empty(t, themes)
}

func TestInMemoryRegistry_Remove(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Remove existing plugin
	err := registry.Remove("test-plugin")
	require.NoError(t, err)

	// Verify plugin is removed
	_, exists := registry.Get("test-plugin")
	assert.False(t, exists)

	// Try to remove non-existing plugin
	err = registry.Remove("non-existing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryRegistry_Clear(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register multiple plugins
	_ = registry.Register("plugin1", &MockPlugin{name: "plugin1"}, createTestMetadata("plugin1", entities.PluginTypeProcessor))
	_ = registry.Register("plugin2", &MockPlugin{name: "plugin2"}, createTestMetadata("plugin2", entities.PluginTypeProcessor))

	// Clear registry
	registry.Clear()

	// Verify all plugins are removed
	all := registry.GetAll()
	assert.Empty(t, all)
}

func TestInMemoryRegistry_GetMetadata(t *testing.T) {
	registry := NewInMemoryRegistry()

	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", &MockPlugin{name: "test-plugin"}, metadata)

	// Get existing metadata
	retrieved, exists := registry.GetMetadata("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, metadata.Name, retrieved.Name)
	assert.Equal(t, metadata.Type, retrieved.Type)

	// Get non-existing metadata
	_, exists = registry.GetMetadata("non-existing")
	assert.False(t, exists)
}

func TestInMemoryRegistry_UpdateStatistics(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Update statistics
	registry.UpdateStatistics("test-plugin", 100*time.Millisecond, true, 1024, 2048)

	// Get statistics
	stats, exists := registry.GetStatistics("test-plugin")
	require.True(t, exists)
	assert.Equal(t, int64(1), stats.ExecutionCount)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, int64(0), stats.ErrorCount)
	assert.Equal(t, 100*time.Millisecond, stats.TotalDuration)
	assert.Equal(t, 100*time.Millisecond, stats.AverageDuration)
	assert.Equal(t, int64(1024), stats.BytesProcessed)
	assert.Equal(t, int64(2048), stats.BytesGenerated)

	// Update with error
	registry.UpdateStatistics("test-plugin", 50*time.Millisecond, false, 512, 0)
	stats, _ = registry.GetStatistics("test-plugin")
	assert.Equal(t, int64(2), stats.ExecutionCount)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, int64(1), stats.ErrorCount)
	assert.Equal(t, 75*time.Millisecond, stats.AverageDuration) // (100+50)/2
}

func TestInMemoryRegistry_IncrementTimeout(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Increment timeout
	registry.IncrementTimeout("test-plugin")

	stats, _ := registry.GetStatistics("test-plugin")
	assert.Equal(t, int64(1), stats.TimeoutCount)
	assert.Equal(t, int64(1), stats.ErrorCount)
}

func TestInMemoryRegistry_IncrementPanic(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Increment panic
	registry.IncrementPanic("test-plugin")

	stats, _ := registry.GetStatistics("test-plugin")
	assert.Equal(t, int64(1), stats.PanicCount)
	assert.Equal(t, int64(1), stats.ErrorCount)

	// Check plugin status
	loaded, err := registry.GetLoadedPlugin("test-plugin")
	require.NoError(t, err)
	assert.Equal(t, entities.PluginStatusError, loaded.Status)
}

func TestInMemoryRegistry_ListLoadedPlugins(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register multiple plugins
	_ = registry.Register("plugin1", &MockPlugin{name: "plugin1"}, createTestMetadata("plugin1", entities.PluginTypeProcessor))
	_ = registry.Register("plugin2", &MockPlugin{name: "plugin2"}, createTestMetadata("plugin2", entities.PluginTypeExporter))

	loaded := registry.ListLoadedPlugins()
	assert.Len(t, loaded, 2)

	// Verify plugin information
	names := make(map[string]bool)
	for _, p := range loaded {
		names[p.Metadata.Name] = true
		assert.Equal(t, entities.PluginStatusLoaded, p.Status)
		assert.NotZero(t, p.LoadedAt)
	}
	assert.True(t, names["plugin1"])
	assert.True(t, names["plugin2"])
}

func TestInMemoryRegistry_SetPluginStatus(t *testing.T) {
	registry := NewInMemoryRegistry()

	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Set status to error
	err := registry.SetPluginStatus("test-plugin", entities.PluginStatusError, "Plugin failed to initialize")
	require.NoError(t, err)

	loaded, _ := registry.GetLoadedPlugin("test-plugin")
	assert.Equal(t, entities.PluginStatusError, loaded.Status)
	assert.Equal(t, "Plugin failed to initialize", loaded.ErrorMsg)

	// Try to set status for non-existing plugin
	err = registry.SetPluginStatus("non-existing", entities.PluginStatusActive, "")
	assert.Error(t, err)
}

func TestInMemoryRegistry_Concurrent(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register a plugin
	plugin := &MockPlugin{name: "test-plugin"}
	metadata := createTestMetadata("test-plugin", entities.PluginTypeProcessor)
	_ = registry.Register("test-plugin", plugin, metadata)

	// Concurrent operations
	done := make(chan bool, 3)

	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = registry.Get("test-plugin")
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Statistics updater
	go func() {
		for i := 0; i < 100; i++ {
			registry.UpdateStatistics("test-plugin", time.Millisecond, true, 100, 200)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Type getter
	go func() {
		for i := 0; i < 100; i++ {
			_ = registry.GetByType(entities.PluginTypeProcessor)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify final state
	stats, _ := registry.GetStatistics("test-plugin")
	assert.Equal(t, int64(100), stats.ExecutionCount)
}
