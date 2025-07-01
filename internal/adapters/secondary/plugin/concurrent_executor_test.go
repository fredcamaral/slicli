package plugin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/test/builders"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// ConcurrentMockPlugin implements the plugin interface for testing concurrent execution
type ConcurrentMockPlugin struct {
	mock.Mock
	ExecuteDelay   time.Duration
	PanicOnExecute bool
}

func (m *ConcurrentMockPlugin) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *ConcurrentMockPlugin) Version() string {
	args := m.Called()
	return args.String(0)
}

func (m *ConcurrentMockPlugin) Description() string {
	args := m.Called()
	return args.String(0)
}

func (m *ConcurrentMockPlugin) Init(config map[string]interface{}) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *ConcurrentMockPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	if m.PanicOnExecute {
		panic("test panic")
	}

	if m.ExecuteDelay > 0 {
		select {
		case <-time.After(m.ExecuteDelay):
		case <-ctx.Done():
			return pluginapi.PluginOutput{}, ctx.Err()
		}
	}

	args := m.Called(ctx, input)
	return args.Get(0).(pluginapi.PluginOutput), args.Error(1)
}

func (m *ConcurrentMockPlugin) Cleanup() error {
	args := m.Called()
	return args.Error(0)
}

func NewConcurrentMockPlugin(name string) *ConcurrentMockPlugin {
	plugin := &ConcurrentMockPlugin{}
	plugin.On("Name").Return(name).Maybe()
	plugin.On("Version").Return("1.0.0").Maybe()
	plugin.On("Description").Return("Mock plugin for testing").Maybe()
	plugin.On("Init", mock.Anything).Return(nil).Maybe()
	plugin.On("Cleanup").Return(nil).Maybe()
	return plugin
}

// Test helper functions
func createTestJob(id string, plugin *ConcurrentMockPlugin, content string) ExecutionJob {
	metadata := builders.NewPluginMetadataBuilder().
		WithName(plugin.Name()).
		Build()

	instance := entities.PluginInstance{
		Instance: plugin,
		Metadata: metadata,
	}

	return ExecutionJob{
		ID:     id,
		Plugin: instance,
		Input: pluginapi.PluginInput{
			Content:  content,
			Language: "text",
			Options:  make(map[string]interface{}),
		},
		Result:    make(chan ExecutionResult, 1),
		StartTime: time.Now(),
		Timeout:   5 * time.Second,
	}
}

func TestNewConcurrentExecutor(t *testing.T) {
	t.Run("creates executor with default max concurrent", func(t *testing.T) {
		executor := NewConcurrentExecutor(0)

		assert.NotNil(t, executor)
		assert.Equal(t, 10, executor.maxConcurrent)
		assert.Equal(t, 10, cap(executor.semaphore))
		assert.NotNil(t, executor.resultCache)
		assert.NotNil(t, executor.activeJobs)
	})

	t.Run("creates executor with custom max concurrent", func(t *testing.T) {
		executor := NewConcurrentExecutor(5)

		assert.Equal(t, 5, executor.maxConcurrent)
		assert.Equal(t, 5, cap(executor.semaphore))
	})

	t.Run("handles negative max concurrent", func(t *testing.T) {
		executor := NewConcurrentExecutor(-1)

		assert.Equal(t, 10, executor.maxConcurrent)
	})
}

func TestConcurrentExecutor_ExecuteConcurrent(t *testing.T) {
	t.Run("executes single job successfully", func(t *testing.T) {
		executor := NewConcurrentExecutor(2)
		plugin := NewConcurrentMockPlugin("test-plugin")

		expectedOutput := pluginapi.PluginOutput{
			HTML: "<div>test output</div>",
		}
		plugin.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil)

		job := createTestJob("job1", plugin, "test content")
		jobs := []ExecutionJob{job}

		result := executor.ExecuteConcurrent(context.Background(), jobs)

		assert.NotNil(t, result)
		assert.Equal(t, 1, result.Success)
		assert.Equal(t, 0, result.Failures)
		assert.Len(t, result.Results, 1)

		jobResult := result.Results["job1"]
		assert.NoError(t, jobResult.Error)
		assert.Equal(t, expectedOutput.HTML, jobResult.Output.HTML)
		assert.False(t, jobResult.Cached)

		plugin.AssertExpectations(t)
	})

	t.Run("executes multiple jobs concurrently", func(t *testing.T) {
		executor := NewConcurrentExecutor(3)

		// Create multiple plugins with delays to verify concurrency
		plugin1 := NewConcurrentMockPlugin("plugin1")
		plugin2 := NewConcurrentMockPlugin("plugin2")
		plugin3 := NewConcurrentMockPlugin("plugin3")

		executionDelay := 100 * time.Millisecond
		plugin1.ExecuteDelay = executionDelay
		plugin2.ExecuteDelay = executionDelay
		plugin3.ExecuteDelay = executionDelay

		output1 := pluginapi.PluginOutput{HTML: "<div>output1</div>"}
		output2 := pluginapi.PluginOutput{HTML: "<div>output2</div>"}
		output3 := pluginapi.PluginOutput{HTML: "<div>output3</div>"}

		plugin1.On("Execute", mock.Anything, mock.Anything).Return(output1, nil)
		plugin2.On("Execute", mock.Anything, mock.Anything).Return(output2, nil)
		plugin3.On("Execute", mock.Anything, mock.Anything).Return(output3, nil)

		jobs := []ExecutionJob{
			createTestJob("job1", plugin1, "content1"),
			createTestJob("job2", plugin2, "content2"),
			createTestJob("job3", plugin3, "content3"),
		}

		start := time.Now()
		result := executor.ExecuteConcurrent(context.Background(), jobs)
		elapsed := time.Since(start)

		// Should complete in roughly one delay period due to concurrency
		assert.Less(t, elapsed, 2*executionDelay, "Should execute concurrently")
		assert.Equal(t, 3, result.Success)
		assert.Equal(t, 0, result.Failures)
		assert.Len(t, result.Results, 3)

		// Verify all results
		assert.Equal(t, output1.HTML, result.Results["job1"].Output.HTML)
		assert.Equal(t, output2.HTML, result.Results["job2"].Output.HTML)
		assert.Equal(t, output3.HTML, result.Results["job3"].Output.HTML)
	})

	t.Run("handles plugin execution errors", func(t *testing.T) {
		executor := NewConcurrentExecutor(2)
		plugin := NewConcurrentMockPlugin("failing-plugin")

		expectedError := errors.New("plugin execution failed")
		plugin.On("Execute", mock.Anything, mock.Anything).Return(pluginapi.PluginOutput{}, expectedError)

		job := createTestJob("job1", plugin, "test content")
		jobs := []ExecutionJob{job}

		result := executor.ExecuteConcurrent(context.Background(), jobs)

		assert.Equal(t, 0, result.Success)
		assert.Equal(t, 1, result.Failures)

		jobResult := result.Results["job1"]
		assert.Error(t, jobResult.Error)
		assert.Equal(t, expectedError, jobResult.Error)

		plugin.AssertExpectations(t)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("slow-plugin")

		// Plugin that would take a long time
		plugin.ExecuteDelay = 1 * time.Second
		plugin.On("Execute", mock.Anything, mock.Anything).Return(pluginapi.PluginOutput{}, nil)

		job := createTestJob("job1", plugin, "test content")
		jobs := []ExecutionJob{job}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		result := executor.ExecuteConcurrent(ctx, jobs)

		assert.Equal(t, 0, result.Success)
		assert.Equal(t, 1, result.Failures)

		jobResult := result.Results["job1"]
		assert.Error(t, jobResult.Error)
		assert.Contains(t, jobResult.Error.Error(), "context deadline exceeded")
	})

	t.Run("respects concurrency limits", func(t *testing.T) {
		maxConcurrent := 2
		executor := NewConcurrentExecutor(maxConcurrent)

		var activeCount int64
		var maxActiveCount int64

		// Create plugins that track concurrent execution
		plugins := make([]*ConcurrentMockPlugin, 5)
		jobs := make([]ExecutionJob, 5)

		for i := 0; i < 5; i++ {
			plugin := NewConcurrentMockPlugin("plugin" + string(rune(i+'1')))
			plugin.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				current := atomic.AddInt64(&activeCount, 1)
				defer atomic.AddInt64(&activeCount, -1)

				// Track maximum concurrent executions
				for {
					max := atomic.LoadInt64(&maxActiveCount)
					if current <= max || atomic.CompareAndSwapInt64(&maxActiveCount, max, current) {
						break
					}
				}

				// Simulate work
				time.Sleep(50 * time.Millisecond)
			}).Return(pluginapi.PluginOutput{HTML: "<div>output</div>"}, nil)

			plugins[i] = plugin
			jobs[i] = createTestJob("job"+string(rune(i+'1')), plugin, "content")
		}

		result := executor.ExecuteConcurrent(context.Background(), jobs)

		assert.Equal(t, 5, result.Success)
		assert.Equal(t, 0, result.Failures)

		// Verify concurrency was limited
		finalMaxActive := atomic.LoadInt64(&maxActiveCount)
		assert.LessOrEqual(t, finalMaxActive, int64(maxConcurrent),
			"Should not exceed max concurrent limit")
	})
}

func TestConcurrentExecutor_Caching(t *testing.T) {
	t.Run("caches successful results", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("cacheable-plugin")

		expectedOutput := pluginapi.PluginOutput{HTML: "<div>cached output</div>"}

		// Should only be called once due to caching
		plugin.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil).Once()

		job := createTestJob("job1", plugin, "same content")

		// First execution
		result1 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job})
		assert.Equal(t, 1, result1.Success)
		assert.False(t, result1.Results["job1"].Cached)

		// Second execution with same content should use cache
		job2 := createTestJob("job2", plugin, "same content")
		result2 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job2})
		assert.Equal(t, 1, result2.Success)
		assert.True(t, result2.Results["job2"].Cached)
		assert.Equal(t, expectedOutput.HTML, result2.Results["job2"].Output.HTML)

		plugin.AssertExpectations(t)
	})

	t.Run("does not cache failed results", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("failing-plugin")

		expectedError := errors.New("execution failed")

		// Should be called twice since failures are not cached
		plugin.On("Execute", mock.Anything, mock.Anything).Return(pluginapi.PluginOutput{}, expectedError).Times(2)

		job1 := createTestJob("job1", plugin, "same content")
		job2 := createTestJob("job2", plugin, "same content")

		// First execution
		result1 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job1})
		assert.Equal(t, 1, result1.Failures)

		// Second execution should not use cache
		result2 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job2})
		assert.Equal(t, 1, result2.Failures)
		assert.False(t, result2.Results["job2"].Cached)

		plugin.AssertExpectations(t)
	})

	t.Run("cache expires after TTL", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("expiring-plugin")

		expectedOutput := pluginapi.PluginOutput{HTML: "<div>output</div>"}

		// Should be called twice due to cache expiration
		plugin.On("Execute", mock.Anything, mock.Anything).Return(expectedOutput, nil).Times(2)

		job := createTestJob("job1", plugin, "same content")

		// First execution
		result1 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job})
		assert.Equal(t, 1, result1.Success)
		assert.False(t, result1.Results["job1"].Cached)

		// Manually expire cache by setting timestamp in the past
		cacheKey := executor.generateCacheKey(plugin.Name(), job.Input)
		if cached, found := executor.resultCache.Load(cacheKey); found {
			if cachedItem, ok := cached.(cachedResult); ok {
				expiredCache := cachedResult{
					result:    cachedItem.result,
					timestamp: time.Now().Add(-10 * time.Minute), // Expired
				}
				executor.resultCache.Store(cacheKey, expiredCache)
			}
		}

		// Second execution should not use expired cache
		job2 := createTestJob("job2", plugin, "same content")
		result2 := executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job2})
		assert.Equal(t, 1, result2.Success)
		assert.False(t, result2.Results["job2"].Cached)

		plugin.AssertExpectations(t)
	})
}

func TestConcurrentExecutor_ExecuteWithPriority(t *testing.T) {
	t.Run("executes priority groups sequentially", func(t *testing.T) {
		executor := NewConcurrentExecutor(5)

		// Create plugins for different priorities
		highPriorityPlugin1 := NewConcurrentMockPlugin("high1")
		highPriorityPlugin2 := NewConcurrentMockPlugin("high2")
		lowPriorityPlugin := NewConcurrentMockPlugin("low1")

		output := pluginapi.PluginOutput{HTML: "<div>output</div>"}

		// Track execution order
		var executionOrder []string
		var mu sync.Mutex

		addToOrder := func(name string) {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
		}

		// High priority plugins
		highPriorityPlugin1.On("Execute", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { addToOrder("high1") }).
			Return(output, nil)
		highPriorityPlugin2.On("Execute", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { addToOrder("high2") }).
			Return(output, nil)

		// Low priority plugin (should execute after high priority)
		lowPriorityPlugin.On("Execute", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { addToOrder("low1") }).
			Return(output, nil)

		// Create priority groups
		prioritizedJobs := [][]ExecutionJob{
			// High priority group
			{
				createTestJob("high1", highPriorityPlugin1, "content1"),
				createTestJob("high2", highPriorityPlugin2, "content2"),
			},
			// Low priority group
			{
				createTestJob("low1", lowPriorityPlugin, "content3"),
			},
		}

		result := executor.ExecuteWithPriority(context.Background(), prioritizedJobs)

		assert.Equal(t, 3, result.Success)
		assert.Equal(t, 0, result.Failures)
		assert.Len(t, result.Results, 3)

		// Verify high priority plugins executed before low priority
		mu.Lock()
		defer mu.Unlock()
		assert.Len(t, executionOrder, 3)

		// Find indices of high and low priority executions
		var highIndices, lowIndices []int
		for i, name := range executionOrder {
			switch name {
			case "high1", "high2":
				highIndices = append(highIndices, i)
			case "low1":
				lowIndices = append(lowIndices, i)
			}
		}

		// All high priority should execute before low priority
		for _, highIdx := range highIndices {
			for _, lowIdx := range lowIndices {
				assert.Less(t, highIdx, lowIdx, "High priority should execute before low priority")
			}
		}
	})

	t.Run("handles empty priority groups", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("test-plugin")

		output := pluginapi.PluginOutput{HTML: "<div>output</div>"}
		plugin.On("Execute", mock.Anything, mock.Anything).Return(output, nil)

		prioritizedJobs := [][]ExecutionJob{
			{}, // Empty group
			{createTestJob("job1", plugin, "content")},
			{}, // Another empty group
		}

		result := executor.ExecuteWithPriority(context.Background(), prioritizedJobs)

		assert.Equal(t, 1, result.Success)
		assert.Equal(t, 0, result.Failures)
		assert.Len(t, result.Results, 1)

		plugin.AssertExpectations(t)
	})
}

func TestConcurrentExecutor_PanicHandling(t *testing.T) {
	t.Run("handles plugin panic gracefully", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("panic-plugin")
		plugin.PanicOnExecute = true

		job := createTestJob("job1", plugin, "test content")
		jobs := []ExecutionJob{job}

		result := executor.ExecuteConcurrent(context.Background(), jobs)

		assert.Equal(t, 0, result.Success)
		assert.Equal(t, 1, result.Failures)

		jobResult := result.Results["job1"]
		assert.Error(t, jobResult.Error)
		assert.Contains(t, jobResult.Error.Error(), "plugin panic")
	})
}

func TestConcurrentExecutor_CacheManagement(t *testing.T) {
	t.Run("gets cache stats", func(t *testing.T) {
		executor := NewConcurrentExecutor(5)

		stats := executor.GetCacheStats()

		assert.Contains(t, stats, "cache_size")
		assert.Contains(t, stats, "max_concurrent")
		assert.Contains(t, stats, "active_jobs")
		assert.Equal(t, 0, stats["cache_size"])
		assert.Equal(t, 5, stats["max_concurrent"])
		assert.Equal(t, 0, stats["active_jobs"])
	})

	t.Run("clears cache", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("test-plugin")

		output := pluginapi.PluginOutput{HTML: "<div>output</div>"}
		plugin.On("Execute", mock.Anything, mock.Anything).Return(output, nil)

		// Execute to populate cache
		job := createTestJob("job1", plugin, "content")
		executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job})

		// Verify cache has items
		stats := executor.GetCacheStats()
		assert.Greater(t, stats["cache_size"], 0)

		// Clear cache
		executor.ClearCache()

		// Verify cache is empty
		stats = executor.GetCacheStats()
		assert.Equal(t, 0, stats["cache_size"])

		plugin.AssertExpectations(t)
	})

	t.Run("clears expired cache entries", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)

		// Manually add expired cache entry
		expiredCache := cachedResult{
			result: ExecutionResult{
				Output: pluginapi.PluginOutput{HTML: "<div>expired</div>"},
			},
			timestamp: time.Now().Add(-10 * time.Minute),
		}

		// Add fresh cache entry
		freshCache := cachedResult{
			result: ExecutionResult{
				Output: pluginapi.PluginOutput{HTML: "<div>fresh</div>"},
			},
			timestamp: time.Now(),
		}

		executor.resultCache.Store("expired-key", expiredCache)
		executor.resultCache.Store("fresh-key", freshCache)

		// Verify both entries exist
		stats := executor.GetCacheStats()
		assert.Equal(t, 2, stats["cache_size"])

		// Clear expired entries
		executor.ClearExpiredCache()

		// Verify only fresh entry remains
		stats = executor.GetCacheStats()
		assert.Equal(t, 1, stats["cache_size"])

		// Verify the correct entry remains
		_, expiredExists := executor.resultCache.Load("expired-key")
		_, freshExists := executor.resultCache.Load("fresh-key")
		assert.False(t, expiredExists)
		assert.True(t, freshExists)
	})
}

func TestConcurrentExecutor_OptimizeForContent(t *testing.T) {
	t.Run("categorizes plugins by priority", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)

		// Create plugins with different names
		syntaxPlugin := entities.PluginInstance{
			Metadata: builders.NewPluginMetadataBuilder().WithName("syntax-highlight").Build(),
		}
		codeExecPlugin := entities.PluginInstance{
			Metadata: builders.NewPluginMetadataBuilder().WithName("code-exec").Build(),
		}
		mermaidPlugin := entities.PluginInstance{
			Metadata: builders.NewPluginMetadataBuilder().WithName("mermaid").Build(),
		}
		otherPlugin := entities.PluginInstance{
			Metadata: builders.NewPluginMetadataBuilder().WithName("other-plugin").Build(),
		}

		plugins := []entities.PluginInstance{
			syntaxPlugin, codeExecPlugin, mermaidPlugin, otherPlugin,
		}

		priorityGroups := executor.OptimizeForContent(plugins, "test content")

		// Should have 3 priority groups: high, medium, low
		assert.Len(t, priorityGroups, 3)

		// High priority group should have syntax-highlight and code-exec
		highPriority := priorityGroups[0]
		assert.Len(t, highPriority, 2)
		highNames := []string{highPriority[0].Plugin.Metadata.Name, highPriority[1].Plugin.Metadata.Name}
		assert.Contains(t, highNames, "syntax-highlight")
		assert.Contains(t, highNames, "code-exec")

		// Medium priority group should have mermaid
		mediumPriority := priorityGroups[1]
		assert.Len(t, mediumPriority, 1)
		assert.Equal(t, "mermaid", mediumPriority[0].Plugin.Metadata.Name)

		// Low priority group should have other-plugin
		lowPriority := priorityGroups[2]
		assert.Len(t, lowPriority, 1)
		assert.Equal(t, "other-plugin", lowPriority[0].Plugin.Metadata.Name)
	})

	t.Run("handles empty plugin list", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)

		priorityGroups := executor.OptimizeForContent([]entities.PluginInstance{}, "content")

		assert.Empty(t, priorityGroups)
	})
}

func TestConcurrentExecutor_SetMaxConcurrent(t *testing.T) {
	t.Run("updates max concurrent limit", func(t *testing.T) {
		executor := NewConcurrentExecutor(5)

		executor.SetMaxConcurrent(10)

		assert.Equal(t, 10, executor.maxConcurrent)
		assert.Equal(t, 10, cap(executor.semaphore))
	})

	t.Run("handles invalid max concurrent", func(t *testing.T) {
		executor := NewConcurrentExecutor(5)

		executor.SetMaxConcurrent(-1)

		assert.Equal(t, 10, executor.maxConcurrent)
		assert.Equal(t, 10, cap(executor.semaphore))
	})
}

func TestConcurrentExecutor_GetActiveJobs(t *testing.T) {
	t.Run("tracks active jobs during execution", func(t *testing.T) {
		executor := NewConcurrentExecutor(1)
		plugin := NewConcurrentMockPlugin("slow-plugin")

		// Create a plugin that takes time to execute
		plugin.ExecuteDelay = 200 * time.Millisecond
		plugin.On("Execute", mock.Anything, mock.Anything).Return(pluginapi.PluginOutput{}, nil)

		job := createTestJob("job1", plugin, "content")

		// Start execution in background
		go executor.ExecuteConcurrent(context.Background(), []ExecutionJob{job})

		// Give it time to start
		time.Sleep(50 * time.Millisecond)

		// Check active jobs
		activeJobs := executor.GetActiveJobs()
		assert.Len(t, activeJobs, 1)
		assert.Equal(t, "job1", activeJobs[0].ID)

		// Wait for completion and check again
		time.Sleep(200 * time.Millisecond)
		activeJobs = executor.GetActiveJobs()
		assert.Empty(t, activeJobs)

		plugin.AssertExpectations(t)
	})
}
