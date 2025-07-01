package plugin

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// MockMemoryTestPlugin simulates a plugin that can consume memory
type MockMemoryTestPlugin struct {
	name          string
	memoryToUse   int64
	sleepDuration time.Duration
	shouldFail    bool
}

func (p *MockMemoryTestPlugin) Name() string {
	return p.name
}

func (p *MockMemoryTestPlugin) Version() string {
	return "1.0.0"
}

func (p *MockMemoryTestPlugin) Description() string {
	return "Mock plugin for memory testing"
}

func (p *MockMemoryTestPlugin) Init(config map[string]interface{}) error {
	return nil
}

func (p *MockMemoryTestPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	if p.shouldFail {
		return pluginapi.PluginOutput{}, assert.AnError
	}

	// Simulate memory usage by allocating a slice
	if p.memoryToUse > 0 {
		// Allocate memory (this is just for testing, real plugins would have different memory patterns)
		buffer := make([]byte, p.memoryToUse)
		// Touch the memory to ensure it's actually allocated
		for i := range buffer {
			if i%1024 == 0 {
				buffer[i] = byte(i % 256)
			}
		}
		// Keep the buffer alive during sleep
		defer func() {
			_ = buffer[0] // Prevent compiler optimization
		}()
	}

	// Simulate processing time
	if p.sleepDuration > 0 {
		select {
		case <-ctx.Done():
			return pluginapi.PluginOutput{}, ctx.Err()
		case <-time.After(p.sleepDuration):
		}
	}

	return pluginapi.PluginOutput{
		HTML: "<p>Memory test completed</p>",
	}, nil
}

func (p *MockMemoryTestPlugin) Cleanup() error {
	return nil
}

func TestMemoryLimiter_Initialization(t *testing.T) {
	tests := []struct {
		name           string
		shouldSucceed  bool
		skipOnPlatform string
	}{
		{
			name:          "successful initialization",
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnPlatform != "" && runtime.GOOS == tt.skipOnPlatform {
				t.Skipf("Skipping test on %s", tt.skipOnPlatform)
			}

			limiter, err := NewMemoryLimiter()

			if tt.shouldSucceed {
				assert.NoError(t, err)
				assert.NotNil(t, limiter)

				// Cleanup
				err = limiter.Cleanup()
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMemoryLimiter_IsMemoryLimitingAvailable(t *testing.T) {
	available := IsMemoryLimitingAvailable()

	switch runtime.GOOS {
	case "linux":
		// Should be available on most Linux systems with cgroups
		t.Logf("Memory limiting available on Linux: %v", available)
	case "darwin":
		// Should be available on macOS with basic resource limits
		assert.True(t, available)
	case "windows":
		// Windows job objects are now implemented
		assert.True(t, available)
	default:
		// Other platforms not supported
		assert.False(t, available)
	}
}

func TestMemoryLimitedExecutor_Creation(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		maxConcurrent  int
		memoryLimit    int64
		shouldSucceed  bool
		skipOnPlatform string
	}{
		{
			name:          "valid parameters",
			timeout:       5 * time.Second,
			maxConcurrent: 5,
			memoryLimit:   100 * 1024 * 1024, // 100MB
			shouldSucceed: true,
		},
		{
			name:          "zero concurrent limit",
			timeout:       5 * time.Second,
			maxConcurrent: 0,
			memoryLimit:   100 * 1024 * 1024,
			shouldSucceed: true, // Should use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnPlatform != "" && runtime.GOOS == tt.skipOnPlatform {
				t.Skipf("Skipping test on %s", tt.skipOnPlatform)
			}

			executor, err := NewMemoryLimitedExecutor(tt.timeout, tt.maxConcurrent, tt.memoryLimit)

			if tt.shouldSucceed {
				assert.NoError(t, err)
				assert.NotNil(t, executor)
				assert.Equal(t, tt.memoryLimit, executor.memoryLimit)

				// Test memory limiting support
				supported := executor.IsMemoryLimitingSupported()
				t.Logf("Memory limiting supported: %v", supported)

				// Cleanup
				err = executor.Cleanup()
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMemoryLimitedExecutor_BasicExecution(t *testing.T) {

	executor, err := NewMemoryLimitedExecutor(5*time.Second, 5, 50*1024*1024) // 50MB limit
	require.NoError(t, err)
	defer func() { _ = executor.Cleanup() }()

	plugin := &MockMemoryTestPlugin{
		name:          "test-plugin",
		memoryToUse:   1024 * 1024, // 1MB
		sleepDuration: 100 * time.Millisecond,
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	output, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 2*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "<p>Memory test completed</p>", output.HTML)
}

func TestMemoryLimitedExecutor_TimeoutHandling(t *testing.T) {

	executor, err := NewMemoryLimitedExecutor(5*time.Second, 5, 50*1024*1024)
	require.NoError(t, err)
	defer func() { _ = executor.Cleanup() }()

	plugin := &MockMemoryTestPlugin{
		name:          "slow-plugin",
		sleepDuration: 2 * time.Second, // Longer than timeout
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	// Set a short timeout
	_, err = executor.ExecuteWithMemoryLimit(ctx, plugin, input, 500*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestMemoryLimitedExecutor_ContextCancellation(t *testing.T) {

	executor, err := NewMemoryLimitedExecutor(5*time.Second, 5, 50*1024*1024)
	require.NoError(t, err)
	defer func() { _ = executor.Cleanup() }()

	plugin := &MockMemoryTestPlugin{
		name:          "long-running-plugin",
		sleepDuration: 2 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	_, err = executor.ExecuteWithMemoryLimit(ctx, plugin, input, 5*time.Second)
	assert.Error(t, err)
	// Should be cancelled or timeout (both are acceptable for context cancellation)
	errorMsg := err.Error()
	assert.True(t, strings.Contains(errorMsg, "cancel") || strings.Contains(errorMsg, "timeout"),
		"Expected error to contain 'cancel' or 'timeout', got: %s", errorMsg)
}

func TestMemoryLimitedExecutor_MemoryUsageTracking(t *testing.T) {

	executor, err := NewMemoryLimitedExecutor(10*time.Second, 5, 100*1024*1024)
	require.NoError(t, err)
	defer func() { _ = executor.Cleanup() }()

	// Initially no memory usage
	usage := executor.GetMemoryUsage()
	assert.Empty(t, usage)

	// Test that we can get memory usage (though it may be empty for this simple test)
	plugin := &MockMemoryTestPlugin{
		name:          "memory-tracker-test",
		memoryToUse:   10 * 1024 * 1024, // 10MB
		sleepDuration: 100 * time.Millisecond,
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	output, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 5*time.Second)
	assert.NoError(t, err)
	assert.NotEmpty(t, output.HTML)
}

func TestMemoryLimitedExecutor_ConcurrentExecution(t *testing.T) {

	executor, err := NewMemoryLimitedExecutor(10*time.Second, 3, 200*1024*1024) // 200MB total
	require.NoError(t, err)
	defer func() { _ = executor.Cleanup() }()

	// Run multiple plugins concurrently
	const numPlugins = 5
	results := make(chan error, numPlugins)

	for i := 0; i < numPlugins; i++ {
		go func(id int) {
			plugin := &MockMemoryTestPlugin{
				name:          fmt.Sprintf("concurrent-plugin-%d", id),
				memoryToUse:   5 * 1024 * 1024, // 5MB each
				sleepDuration: 200 * time.Millisecond,
			}

			ctx := context.Background()
			input := pluginapi.PluginInput{
				Content: fmt.Sprintf("test content %d", id),
			}

			_, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 5*time.Second)
			results <- err
		}(i)
	}

	// Collect results
	var errors []error
	for i := 0; i < numPlugins; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	// Some executions should succeed (limited by maxConcurrent=3)
	t.Logf("Concurrent execution errors: %d/%d", len(errors), numPlugins)
}

func TestMemoryLimitedExecutor_FallbackExecution(t *testing.T) {
	// Test fallback when memory limiter is nil
	executor := &MemoryLimitedExecutor{
		SandboxExecutor: NewSandboxExecutor(5*time.Second, 5),
		memoryLimit:     100 * 1024 * 1024,
		memoryLimiter:   nil, // Force fallback
	}

	plugin := &MockMemoryTestPlugin{
		name:          "fallback-test",
		memoryToUse:   1024 * 1024,
		sleepDuration: 100 * time.Millisecond,
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "test content",
	}

	output, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 2*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "<p>Memory test completed</p>", output.HTML)
}

// Benchmark tests
func BenchmarkMemoryLimitedExecutor_SmallPlugin(b *testing.B) {

	executor, err := NewMemoryLimitedExecutor(5*time.Second, 10, 100*1024*1024)
	require.NoError(b, err)
	defer func() { _ = executor.Cleanup() }()

	plugin := &MockMemoryTestPlugin{
		name:        "benchmark-plugin",
		memoryToUse: 1024 * 1024, // 1MB
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "benchmark content",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 2*time.Second)
		if err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}
}

func BenchmarkMemoryLimitedExecutor_LargePlugin(b *testing.B) {

	executor, err := NewMemoryLimitedExecutor(10*time.Second, 5, 500*1024*1024) // 500MB
	require.NoError(b, err)
	defer func() { _ = executor.Cleanup() }()

	plugin := &MockMemoryTestPlugin{
		name:        "large-benchmark-plugin",
		memoryToUse: 50 * 1024 * 1024, // 50MB
	}

	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content: "large benchmark content",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteWithMemoryLimit(ctx, plugin, input, 5*time.Second)
		if err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}
}
