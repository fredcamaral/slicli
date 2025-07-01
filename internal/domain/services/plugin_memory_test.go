package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// TestPluginService_MemoryLimiting tests memory limiting functionality
func TestPluginService_MemoryLimiting(t *testing.T) {
	tests := []struct {
		name                string
		enableMemoryLimit   bool
		memoryLimit         int64
		expectMemoryLimited bool
	}{
		{
			name:                "memory limiting enabled",
			enableMemoryLimit:   true,
			memoryLimit:         50 * 1024 * 1024, // 50MB
			expectMemoryLimited: true,
		},
		{
			name:                "memory limiting disabled",
			enableMemoryLimit:   false,
			memoryLimit:         0,
			expectMemoryLimited: false,
		},
		{
			name:                "memory limiting enabled with default limit",
			enableMemoryLimit:   true,
			memoryLimit:         0, // Should use default
			expectMemoryLimited: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			loader := &MockPluginLoader{}
			executor := &MockPluginExecutor{}
			registry := &MockPluginRegistry{}
			cache := &MockPluginCache{}
			matcher := &MockPluginMatcher{}

			// Configure plugin service with memory limiting
			config := PluginServiceConfig{
				DefaultTimeout:    5 * time.Second,
				MaxConcurrent:     5,
				CacheEnabled:      true,
				CacheTTL:          5 * time.Minute,
				EnableMemoryLimit: tt.enableMemoryLimit,
				MemoryLimit:       tt.memoryLimit,
			}

			// Set up mock expectations for shutdown
			registry.On("GetAll").Return(map[string]pluginapi.Plugin{})
			cache.On("Clear").Return()

			service := NewPluginService(loader, executor, registry, cache, matcher, config, nil)

			// Check if memory limiting is properly configured
			enabled := service.IsMemoryLimitingEnabled()
			if tt.expectMemoryLimited {
				// On platforms that support memory limiting, it should be enabled
				// On platforms that don't, it should be disabled
				t.Logf("Memory limiting enabled: %v", enabled)
			} else {
				assert.False(t, enabled, "Memory limiting should be disabled")
			}

			// Check memory limit configuration
			configEnabled, limit := service.GetMemoryLimitConfig()
			assert.Equal(t, tt.enableMemoryLimit, configEnabled)
			if tt.memoryLimit > 0 {
				assert.Equal(t, tt.memoryLimit, limit)
			} else if tt.enableMemoryLimit {
				assert.Equal(t, int64(100*1024*1024), limit) // Default 100MB
			}

			// Test memory usage tracking
			usage := service.GetMemoryUsage()
			assert.NotNil(t, usage)
			assert.Empty(t, usage) // Should be empty initially

			// Cleanup
			err := service.Shutdown(context.Background())
			assert.NoError(t, err)

			// Verify expectations
			registry.AssertExpectations(t)
			cache.AssertExpectations(t)
		})
	}
}

// TestPluginService_MemoryMonitoring tests memory usage monitoring
func TestPluginService_MemoryMonitoring(t *testing.T) {
	// Create mocks
	loader := &MockPluginLoader{}
	executor := &MockPluginExecutor{}
	registry := &MockPluginRegistry{}
	cache := &MockPluginCache{}

	// Create test plugin
	plugin := &TestPlugin{
		name:    "memory-test",
		version: "1.0.0",
	}

	// Configure service with memory limiting
	config := PluginServiceConfig{
		DefaultTimeout:    5 * time.Second,
		MaxConcurrent:     5,
		EnableMemoryLimit: true,
		MemoryLimit:       100 * 1024 * 1024, // 100MB
	}

	// Set up mock expectations for shutdown
	registry.On("GetAll").Return(map[string]pluginapi.Plugin{})
	cache.On("Clear").Return()

	service := NewPluginService(loader, executor, registry, cache, nil, config, nil)

	// Set up mock expectations
	registry.On("Get", "memory-test").Return(plugin, true)
	registry.On("GetMetadata", "memory-test").Return((*entities.PluginMetadata)(nil), false)

	if service.IsMemoryLimitingEnabled() {
		// Memory limiting is available on this platform
		cache.On("Get", mock.AnythingOfType("string")).Return((*pluginapi.PluginOutput)(nil), false).Maybe()
		cache.On("Set", mock.AnythingOfType("string"), mock.AnythingOfType("*plugin.PluginOutput"), mock.AnythingOfType("time.Duration")).Return().Maybe()
		registry.On("UpdateStatistics", "memory-test", mock.AnythingOfType("time.Duration"), true, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return()

		// Execute plugin
		input := pluginapi.PluginInput{
			Content:  "test content",
			Language: "test",
		}

		output, err := service.ExecutePlugin(context.Background(), "memory-test", input)
		assert.NoError(t, err)
		assert.NotEmpty(t, output.HTML)

		// Check that memory usage might be tracked
		usage := service.GetMemoryUsage()
		assert.NotNil(t, usage)
		t.Logf("Memory usage tracked: %v", usage)
	} else {
		// Memory limiting not available, should fall back to regular execution
		executor.On("ExecuteWithTimeout", mock.Anything, plugin, mock.AnythingOfType("plugin.PluginInput"), mock.AnythingOfType("time.Duration")).Return(
			pluginapi.PluginOutput{HTML: "<p>test output</p>"}, nil)
		cache.On("Get", mock.AnythingOfType("string")).Return((*pluginapi.PluginOutput)(nil), false)
		cache.On("Set", mock.AnythingOfType("string"), mock.AnythingOfType("*plugin.PluginOutput"), mock.AnythingOfType("time.Duration")).Return()
		registry.On("UpdateStatistics", "memory-test", mock.AnythingOfType("time.Duration"), true, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return()

		// Execute plugin
		input := pluginapi.PluginInput{
			Content:  "test content",
			Language: "test",
		}

		output, err := service.ExecutePlugin(context.Background(), "memory-test", input)
		assert.NoError(t, err)
		assert.Equal(t, "<p>test output</p>", output.HTML)
	}

	// Cleanup
	err := service.Shutdown(context.Background())
	assert.NoError(t, err)

	// Verify all expectations
	registry.AssertExpectations(t)
	cache.AssertExpectations(t)
	if !service.IsMemoryLimitingEnabled() {
		executor.AssertExpectations(t)
	}
}

// TestPluginService_MemoryLimitingFallback tests fallback when memory limiting fails
func TestPluginService_MemoryLimitingFallback(t *testing.T) {
	// Create mocks
	loader := &MockPluginLoader{}
	executor := &MockPluginExecutor{}
	registry := &MockPluginRegistry{}
	cache := &MockPluginCache{}

	plugin := &TestPlugin{
		name:    "fallback-test",
		version: "1.0.0",
	}

	// Configure service with memory limiting
	config := PluginServiceConfig{
		DefaultTimeout:    5 * time.Second,
		MaxConcurrent:     5,
		EnableMemoryLimit: true,
		MemoryLimit:       100 * 1024 * 1024,
	}

	// Set up mock expectations for shutdown
	registry.On("GetAll").Return(map[string]pluginapi.Plugin{})
	cache.On("Clear").Return()

	service := NewPluginService(loader, executor, registry, cache, nil, config, nil)

	// Force fallback by setting memory executor to nil
	service.memoryExecutor = nil

	// Set up mock expectations for fallback execution
	registry.On("Get", "fallback-test").Return(plugin, true)
	registry.On("GetMetadata", "fallback-test").Return((*entities.PluginMetadata)(nil), false)
	executor.On("ExecuteWithTimeout", mock.Anything, plugin, mock.AnythingOfType("plugin.PluginInput"), mock.AnythingOfType("time.Duration")).Return(
		pluginapi.PluginOutput{HTML: "<p>fallback execution</p>"}, nil)
	cache.On("Get", mock.AnythingOfType("string")).Return((*pluginapi.PluginOutput)(nil), false).Maybe()
	cache.On("Set", mock.AnythingOfType("string"), mock.AnythingOfType("*plugin.PluginOutput"), mock.AnythingOfType("time.Duration")).Return().Maybe()
	registry.On("UpdateStatistics", "fallback-test", mock.AnythingOfType("time.Duration"), true, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return()

	// Execute plugin - should use fallback
	input := pluginapi.PluginInput{
		Content:  "test content",
		Language: "test",
	}

	output, err := service.ExecutePlugin(context.Background(), "fallback-test", input)
	assert.NoError(t, err)
	assert.Equal(t, "<p>fallback execution</p>", output.HTML)

	// Memory limiting should report as disabled
	assert.False(t, service.IsMemoryLimitingEnabled())

	// Cleanup
	err = service.Shutdown(context.Background())
	assert.NoError(t, err)

	// Verify expectations
	registry.AssertExpectations(t)
	executor.AssertExpectations(t)
	cache.AssertExpectations(t)
}

// TestPluginService_MemoryLimitingConfiguration tests various configuration scenarios
func TestPluginService_MemoryLimitingConfiguration(t *testing.T) {
	testCases := []struct {
		name                string
		config              PluginServiceConfig
		expectedMemoryLimit int64
		expectedEnabled     bool
	}{
		{
			name: "default configuration",
			config: PluginServiceConfig{
				EnableMemoryLimit: false,
			},
			expectedMemoryLimit: 100 * 1024 * 1024, // Default
			expectedEnabled:     false,
		},
		{
			name: "custom memory limit",
			config: PluginServiceConfig{
				EnableMemoryLimit: true,
				MemoryLimit:       256 * 1024 * 1024, // 256MB
			},
			expectedMemoryLimit: 256 * 1024 * 1024,
			expectedEnabled:     true,
		},
		{
			name: "zero memory limit uses default",
			config: PluginServiceConfig{
				EnableMemoryLimit: true,
				MemoryLimit:       0,
			},
			expectedMemoryLimit: 100 * 1024 * 1024, // Default
			expectedEnabled:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create minimal mocks
			loader := &MockPluginLoader{}
			executor := &MockPluginExecutor{}
			registry := &MockPluginRegistry{}
			cache := &MockPluginCache{}

			// Set up mock expectations for shutdown
			registry.On("GetAll").Return(map[string]pluginapi.Plugin{})
			cache.On("Clear").Return()

			service := NewPluginService(loader, executor, registry, cache, nil, tc.config, nil)

			// Check configuration
			enabled, limit := service.GetMemoryLimitConfig()
			assert.Equal(t, tc.expectedEnabled, enabled)
			assert.Equal(t, tc.expectedMemoryLimit, limit)

			// Cleanup
			err := service.Shutdown(context.Background())
			assert.NoError(t, err)

			// Verify expectations
			registry.AssertExpectations(t)
			cache.AssertExpectations(t)
		})
	}
}

// BenchmarkPluginService_MemoryLimitedExecution benchmarks execution with memory limiting
func BenchmarkPluginService_MemoryLimitedExecution(b *testing.B) {
	// Create mocks
	loader := &MockPluginLoader{}
	executor := &MockPluginExecutor{}
	registry := &MockPluginRegistry{}
	cache := &MockPluginCache{}

	plugin := &TestPlugin{
		name:    "benchmark-test",
		version: "1.0.0",
	}

	config := PluginServiceConfig{
		DefaultTimeout:    5 * time.Second,
		MaxConcurrent:     10,
		EnableMemoryLimit: true,
		MemoryLimit:       100 * 1024 * 1024,
		CacheEnabled:      false, // Disable cache for benchmark
	}

	// Set up mock expectations for shutdown
	registry.On("GetAll").Return(map[string]pluginapi.Plugin{})
	cache.On("Clear").Return()

	service := NewPluginService(loader, executor, registry, cache, nil, config, nil)

	// Set up mock expectations
	registry.On("Get", "benchmark-test").Return(plugin, true)
	registry.On("UpdateStatistics", "benchmark-test", mock.AnythingOfType("time.Duration"), true, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return()

	if !service.IsMemoryLimitingEnabled() {
		// Fallback to regular execution for benchmark
		executor.On("ExecuteWithTimeout", mock.Anything, plugin, mock.AnythingOfType("plugin.PluginInput"), mock.AnythingOfType("time.Duration")).Return(
			pluginapi.PluginOutput{HTML: "<p>benchmark output</p>"}, nil)
	}

	input := pluginapi.PluginInput{
		Content:  "benchmark content",
		Language: "test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ExecutePlugin(context.Background(), "benchmark-test", input)
		if err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}

	// Cleanup
	err := service.Shutdown(context.Background())
	require.NoError(b, err)
}
