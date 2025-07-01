package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type MockPluginLoader struct {
	mock.Mock
}

func (m *MockPluginLoader) Discover(ctx context.Context, dirs []string) ([]pluginapi.PluginInfo, error) {
	args := m.Called(ctx, dirs)
	if info := args.Get(0); info != nil {
		return info.([]pluginapi.PluginInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginLoader) Load(ctx context.Context, path string) (pluginapi.Plugin, error) {
	args := m.Called(ctx, path)
	if p := args.Get(0); p != nil {
		return p.(pluginapi.Plugin), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginLoader) Unload(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockPluginLoader) Validate(p pluginapi.Plugin) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *MockPluginLoader) LoadManifest(ctx context.Context, path string) (*entities.PluginManifest, error) {
	args := m.Called(ctx, path)
	if manifest := args.Get(0); manifest != nil {
		return manifest.(*entities.PluginManifest), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockPluginExecutor struct {
	mock.Mock
}

func (m *MockPluginExecutor) Execute(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	args := m.Called(ctx, p, input)
	return args.Get(0).(pluginapi.PluginOutput), args.Error(1)
}

func (m *MockPluginExecutor) ExecuteWithTimeout(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, timeout time.Duration) (pluginapi.PluginOutput, error) {
	args := m.Called(ctx, p, input, timeout)
	return args.Get(0).(pluginapi.PluginOutput), args.Error(1)
}

func (m *MockPluginExecutor) ExecuteAsync(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput) <-chan ports.PluginResult {
	args := m.Called(ctx, p, input)
	return args.Get(0).(<-chan ports.PluginResult)
}

type MockPluginRegistry struct {
	mock.Mock
	plugins map[string]pluginapi.Plugin
}

func NewMockPluginRegistry() *MockPluginRegistry {
	return &MockPluginRegistry{
		plugins: make(map[string]pluginapi.Plugin),
	}
}

func (m *MockPluginRegistry) Register(name string, p pluginapi.Plugin, metadata entities.PluginMetadata) error {
	args := m.Called(name, p, metadata)
	if args.Error(0) == nil {
		m.plugins[name] = p
	}
	return args.Error(0)
}

func (m *MockPluginRegistry) Get(name string) (pluginapi.Plugin, bool) {
	args := m.Called(name)
	if p := args.Get(0); p != nil {
		return p.(pluginapi.Plugin), args.Bool(1)
	}
	// Also check internal map
	if p, exists := m.plugins[name]; exists {
		return p, true
	}
	return nil, args.Bool(1)
}

func (m *MockPluginRegistry) GetAll() map[string]pluginapi.Plugin {
	args := m.Called()
	if result := args.Get(0); result != nil {
		return result.(map[string]pluginapi.Plugin)
	}
	return m.plugins
}

func (m *MockPluginRegistry) GetByType(pluginType entities.PluginType) []pluginapi.Plugin {
	args := m.Called(pluginType)
	if result := args.Get(0); result != nil {
		return result.([]pluginapi.Plugin)
	}
	return nil
}

func (m *MockPluginRegistry) Remove(name string) error {
	args := m.Called(name)
	if args.Error(0) == nil {
		delete(m.plugins, name)
	}
	return args.Error(0)
}

func (m *MockPluginRegistry) Clear() {
	m.Called()
	m.plugins = make(map[string]pluginapi.Plugin)
}

func (m *MockPluginRegistry) GetMetadata(name string) (*entities.PluginMetadata, bool) {
	args := m.Called(name)
	if metadata := args.Get(0); metadata != nil {
		return metadata.(*entities.PluginMetadata), args.Bool(1)
	}
	return nil, args.Bool(1)
}

func (m *MockPluginRegistry) GetStatistics(name string) (*entities.PluginStatistics, bool) {
	args := m.Called(name)
	if stats := args.Get(0); stats != nil {
		return stats.(*entities.PluginStatistics), args.Bool(1)
	}
	return nil, args.Bool(1)
}

func (m *MockPluginRegistry) UpdateStatistics(name string, duration time.Duration, success bool, bytesIn, bytesOut int64) {
	m.Called(name, duration, success, bytesIn, bytesOut)
}

func (m *MockPluginRegistry) IncrementTimeout(name string) {
	m.Called(name)
}

func (m *MockPluginRegistry) IncrementPanic(name string) {
	m.Called(name)
}

func (m *MockPluginRegistry) GetLoadedPlugin(name string) (*entities.LoadedPlugin, error) {
	args := m.Called(name)
	if plugin := args.Get(0); plugin != nil {
		return plugin.(*entities.LoadedPlugin), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginRegistry) ListLoadedPlugins() []entities.LoadedPlugin {
	args := m.Called()
	if plugins := args.Get(0); plugins != nil {
		return plugins.([]entities.LoadedPlugin)
	}
	return nil
}

func (m *MockPluginRegistry) SetPluginStatus(name string, status entities.PluginStatus, errorMsg string) error {
	args := m.Called(name, status, errorMsg)
	return args.Error(0)
}

type MockPluginCache struct {
	mock.Mock
}

func (m *MockPluginCache) Get(key string) (*pluginapi.PluginOutput, bool) {
	args := m.Called(key)
	if output := args.Get(0); output != nil {
		return output.(*pluginapi.PluginOutput), args.Bool(1)
	}
	return nil, args.Bool(1)
}

func (m *MockPluginCache) Set(key string, output *pluginapi.PluginOutput, ttl time.Duration) {
	m.Called(key, output, ttl)
}

func (m *MockPluginCache) Remove(key string) {
	m.Called(key)
}

func (m *MockPluginCache) Clear() {
	m.Called()
}

func (m *MockPluginCache) Stats() entities.CacheStats {
	args := m.Called()
	return args.Get(0).(entities.CacheStats)
}

type MockPluginMatcher struct {
	mock.Mock
}

func (m *MockPluginMatcher) Match(content string, language string, metadata map[string]interface{}) []string {
	args := m.Called(content, language, metadata)
	if result := args.Get(0); result != nil {
		return result.([]string)
	}
	return nil
}

func (m *MockPluginMatcher) MatchByType(content string, pluginType entities.PluginType) []string {
	args := m.Called(content, pluginType)
	if result := args.Get(0); result != nil {
		return result.([]string)
	}
	return nil
}

func (m *MockPluginMatcher) AddRule(pluginName string, rule ports.MatchRule) {
	m.Called(pluginName, rule)
}

func (m *MockPluginMatcher) RemoveRule(pluginName string, ruleID string) {
	m.Called(pluginName, ruleID)
}

// Test helpers
type TestPlugin struct {
	name    string
	version string
}

func (p *TestPlugin) Name() string                             { return p.name }
func (p *TestPlugin) Version() string                          { return p.version }
func (p *TestPlugin) Description() string                      { return "Test plugin" }
func (p *TestPlugin) Init(config map[string]interface{}) error { return nil }
func (p *TestPlugin) Cleanup() error                           { return nil }
func (p *TestPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	return pluginapi.PluginOutput{HTML: "<div>test</div>"}, nil
}

func createTestService(t *testing.T) (*PluginService, *MockPluginLoader, *MockPluginExecutor, *MockPluginRegistry, *MockPluginCache, *MockPluginMatcher) {
	loader := new(MockPluginLoader)
	executor := new(MockPluginExecutor)
	registry := NewMockPluginRegistry()
	cache := new(MockPluginCache)
	matcher := new(MockPluginMatcher)

	config := PluginServiceConfig{
		PluginDirs:     []string{"/plugins"},
		DefaultTimeout: 5 * time.Second,
		MaxConcurrent:  10,
		CacheEnabled:   true,
		CacheTTL:       5 * time.Minute,
	}

	service := NewPluginService(loader, executor, registry, cache, matcher, config, nil)
	return service, loader, executor, registry, cache, matcher
}

func TestPluginService_LoadPlugin(t *testing.T) {
	service, loader, _, registry, _, _ := createTestService(t)
	ctx := context.Background()

	testPlugin := &TestPlugin{name: "test", version: "1.0.0"}
	manifest := &entities.PluginManifest{
		Metadata: entities.PluginMetadata{
			Name:        "test",
			Version:     "1.0.0",
			Description: "Test plugin",
			Type:        entities.PluginTypeProcessor,
		},
	}

	loader.On("Load", ctx, "/path/to/plugin.so").Return(testPlugin, nil)
	loader.On("LoadManifest", ctx, "/path/to/plugin.toml").Return(manifest, nil)
	registry.On("Register", "test", testPlugin, manifest.Metadata).Return(nil)

	err := service.LoadPlugin(ctx, "/path/to/plugin.so")
	require.NoError(t, err)

	loader.AssertExpectations(t)
	registry.AssertExpectations(t)
}

func TestPluginService_LoadPlugin_NoManifest(t *testing.T) {
	service, loader, _, registry, _, _ := createTestService(t)
	ctx := context.Background()

	testPlugin := &TestPlugin{name: "test", version: "1.0.0"}

	loader.On("Load", ctx, "/path/to/plugin.so").Return(testPlugin, nil)
	loader.On("LoadManifest", ctx, "/path/to/plugin.toml").Return(nil, errors.New("not found"))
	registry.On("Register", "test", testPlugin, mock.Anything).Return(nil)

	err := service.LoadPlugin(ctx, "/path/to/plugin.so")
	require.NoError(t, err)

	loader.AssertExpectations(t)
	registry.AssertExpectations(t)
}

func TestPluginService_UnloadPlugin(t *testing.T) {
	service, loader, _, registry, cache, _ := createTestService(t)
	ctx := context.Background()

	testPlugin := &TestPlugin{name: "test", version: "1.0.0"}

	registry.On("Get", "test").Return(testPlugin, true)
	registry.On("Remove", "test").Return(nil)
	loader.On("Unload", ctx, "test").Return(nil)
	cache.On("Clear").Return()

	err := service.UnloadPlugin(ctx, "test")
	require.NoError(t, err)

	loader.AssertExpectations(t)
	registry.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestPluginService_ExecutePlugin(t *testing.T) {
	service, _, executor, registry, cache, _ := createTestService(t)
	ctx := context.Background()

	testPlugin := &TestPlugin{name: "test", version: "1.0.0"}
	input := pluginapi.PluginInput{Content: "test content"}
	output := pluginapi.PluginOutput{HTML: "<div>output</div>"}

	registry.On("Get", "test").Return(testPlugin, true)
	registry.On("GetMetadata", "test").Return((*entities.PluginMetadata)(nil), false)
	cache.On("Get", mock.Anything).Return(nil, false)
	executor.On("ExecuteWithTimeout", ctx, testPlugin, input, 5*time.Second).Return(output, nil)
	registry.On("UpdateStatistics", "test", mock.Anything, true, int64(12), int64(17))
	cache.On("Set", mock.Anything, &output, 5*time.Minute)

	result, err := service.ExecutePlugin(ctx, "test", input)
	require.NoError(t, err)
	assert.Equal(t, output, result)

	executor.AssertExpectations(t)
	registry.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestPluginService_ExecutePlugin_FromCache(t *testing.T) {
	service, _, _, registry, cache, _ := createTestService(t)
	ctx := context.Background()

	testPlugin := &TestPlugin{name: "test", version: "1.0.0"}
	input := pluginapi.PluginInput{Content: "test content"}
	cachedOutput := &pluginapi.PluginOutput{HTML: "<div>cached</div>"}

	registry.On("Get", "test").Return(testPlugin, true)
	cache.On("Get", mock.Anything).Return(cachedOutput, true)

	result, err := service.ExecutePlugin(ctx, "test", input)
	require.NoError(t, err)
	assert.Equal(t, *cachedOutput, result)

	registry.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestPluginService_DiscoverPlugins(t *testing.T) {
	service, loader, _, _, _, _ := createTestService(t)
	ctx := context.Background()

	plugins := []pluginapi.PluginInfo{
		{Name: "plugin1", Version: "1.0.0", Path: "/plugins/plugin1.so", Compatible: true},
		{Name: "plugin2", Version: "2.0.0", Path: "/plugins/plugin2.so", Compatible: false},
	}

	loader.On("Discover", ctx, []string{"/plugins"}).Return(plugins, nil)

	// With auto-discover disabled
	service.config.AutoDiscover = false
	discovered, err := service.DiscoverPlugins(ctx)
	require.NoError(t, err)
	assert.Equal(t, plugins, discovered)

	loader.AssertExpectations(t)
}

func TestPluginService_ProcessContent(t *testing.T) {
	service, _, executor, registry, cache, matcher := createTestService(t)
	ctx := context.Background()

	t.Run("single plugin sequential execution", func(t *testing.T) {
		plugin1 := &TestPlugin{name: "plugin1", version: "1.0.0"}
		output1 := pluginapi.PluginOutput{HTML: "<div>output1</div>"}

		matcher.On("Match", "test content", "markdown", mock.Anything).Return([]string{"plugin1"})
		registry.On("Get", "plugin1").Return(plugin1, true)
		registry.On("GetMetadata", "plugin1").Return((*entities.PluginMetadata)(nil), false)
		cache.On("Get", mock.Anything).Return(nil, false)
		executor.On("ExecuteWithTimeout", ctx, plugin1, mock.Anything, 5*time.Second).Return(output1, nil)
		registry.On("UpdateStatistics", "plugin1", mock.Anything, true, mock.Anything, mock.Anything)
		cache.On("Set", mock.Anything, &output1, 5*time.Minute)

		outputs, err := service.ProcessContent(ctx, "test content", "markdown")
		require.NoError(t, err)
		assert.Len(t, outputs, 1)
		assert.Equal(t, output1, outputs[0])

		matcher.AssertExpectations(t)
		executor.AssertExpectations(t)
		registry.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("multiple plugins concurrent execution", func(t *testing.T) {
		// Reset mocks for second test
		matcher.ExpectedCalls = nil
		executor.ExpectedCalls = nil
		registry.ExpectedCalls = nil
		cache.ExpectedCalls = nil

		plugin1 := &TestPlugin{name: "plugin1", version: "1.0.0"}
		plugin2 := &TestPlugin{name: "plugin2", version: "1.0.0"}

		matcher.On("Match", "test content", "markdown", mock.Anything).Return([]string{"plugin1", "plugin2"})

		// Mock GetLoadedPlugin calls for concurrent execution path
		registry.On("GetLoadedPlugin", "plugin1").Return(&entities.LoadedPlugin{
			Metadata: entities.PluginMetadata{Name: "plugin1", Version: "1.0.0"},
			Status:   entities.PluginStatusLoaded,
		}, nil)
		registry.On("GetLoadedPlugin", "plugin2").Return(&entities.LoadedPlugin{
			Metadata: entities.PluginMetadata{Name: "plugin2", Version: "1.0.0"},
			Status:   entities.PluginStatusLoaded,
		}, nil)

		registry.On("Get", "plugin1").Return(plugin1, true)
		registry.On("Get", "plugin2").Return(plugin2, true)

		outputs, err := service.ProcessContent(ctx, "test content", "markdown")
		require.NoError(t, err)
		assert.Len(t, outputs, 2)
		// The concurrent execution will use the actual plugin execution, so expect the TestPlugin output
		assert.Equal(t, "<div>test</div>", outputs[0].HTML)
		assert.Equal(t, "<div>test</div>", outputs[1].HTML)

		matcher.AssertExpectations(t)
		registry.AssertExpectations(t)
	})
}

func TestPluginService_Shutdown(t *testing.T) {
	service, loader, _, registry, cache, _ := createTestService(t)
	ctx := context.Background()

	plugin1 := &TestPlugin{name: "plugin1", version: "1.0.0"}
	plugin2 := &TestPlugin{name: "plugin2", version: "1.0.0"}

	registry.On("GetAll").Return(map[string]pluginapi.Plugin{
		"plugin1": plugin1,
		"plugin2": plugin2,
	})
	registry.On("Get", "plugin1").Return(plugin1, true)
	registry.On("Get", "plugin2").Return(plugin2, true)
	registry.On("Remove", "plugin1").Return(nil)
	registry.On("Remove", "plugin2").Return(nil)
	loader.On("Unload", ctx, "plugin1").Return(nil)
	loader.On("Unload", ctx, "plugin2").Return(nil)
	cache.On("Clear")

	err := service.Shutdown(ctx)
	require.NoError(t, err)

	loader.AssertExpectations(t)
	registry.AssertExpectations(t)
	cache.AssertExpectations(t)
}
