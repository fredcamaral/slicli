package monitoring

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPerformanceMonitor(t *testing.T) {
	monitor := NewPerformanceMonitor()

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.metrics)
	assert.NotNil(t, monitor.stopCh)
	assert.False(t, monitor.running)
	assert.NotZero(t, monitor.metrics.AppStartTime)
}

func TestPerformanceMonitor_StartStop(t *testing.T) {
	t.Run("start monitoring", func(t *testing.T) {
		monitor := NewPerformanceMonitor()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start monitoring
		monitor.Start(ctx)
		assert.True(t, monitor.running)
		assert.NotNil(t, monitor.ticker)

		// Stop monitoring
		monitor.Stop()
		assert.False(t, monitor.running)
	})

	t.Run("multiple starts do nothing", func(t *testing.T) {
		monitor := NewPerformanceMonitor()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		monitor.Start(ctx)
		originalTicker := monitor.ticker

		// Second start should not change anything
		monitor.Start(ctx)
		assert.Equal(t, originalTicker, monitor.ticker)

		monitor.Stop()
	})

	t.Run("multiple stops do nothing", func(t *testing.T) {
		monitor := NewPerformanceMonitor()

		// Stop without starting should not panic
		monitor.Stop()
		assert.False(t, monitor.running)

		// Multiple stops should not panic
		monitor.Stop()
		assert.False(t, monitor.running)
	})
}

func TestPerformanceMonitor_RecordOperations(t *testing.T) {
	monitor := NewPerformanceMonitor()

	t.Run("record slide render", func(t *testing.T) {
		duration := 100 * time.Millisecond

		monitor.RecordSlideRender(duration)

		metrics := monitor.GetMetrics()
		assert.Equal(t, int64(1), metrics.SlideRenderCount)
		assert.Equal(t, duration, metrics.AverageRenderTime)

		// Record another render to test average calculation
		duration2 := 200 * time.Millisecond
		monitor.RecordSlideRender(duration2)

		metrics2 := monitor.GetMetrics()
		assert.Equal(t, int64(2), metrics2.SlideRenderCount)
		// Average should be between the two values due to exponential moving average
		assert.Greater(t, metrics2.AverageRenderTime, duration)
		assert.Less(t, metrics2.AverageRenderTime, duration2)
	})

	t.Run("record plugin execution", func(t *testing.T) {
		duration := 50 * time.Millisecond

		monitor.RecordPluginExecution(duration)

		metrics := monitor.GetMetrics()
		assert.Equal(t, int64(1), metrics.PluginExecutions)
		assert.Equal(t, duration, metrics.PluginLoadDuration)
	})

	t.Run("record HTTP request", func(t *testing.T) {
		monitor.RecordHTTPRequest()
		monitor.RecordHTTPRequest()

		metrics := monitor.GetMetrics()
		assert.Equal(t, int64(2), metrics.HTTPRequests)
	})

	t.Run("record WebSocket connection", func(t *testing.T) {
		monitor.RecordWebSocketConnection()

		metrics := monitor.GetMetrics()
		assert.Equal(t, int64(1), metrics.WebSocketConnections)
	})
}

func TestPerformanceMonitor_GetMetrics(t *testing.T) {
	monitor := NewPerformanceMonitor()

	// Record some operations
	monitor.RecordSlideRender(100 * time.Millisecond)
	monitor.RecordPluginExecution(50 * time.Millisecond)
	monitor.RecordHTTPRequest()
	monitor.RecordWebSocketConnection()

	metrics := monitor.GetMetrics()

	// Verify all counters
	assert.Equal(t, int64(1), metrics.SlideRenderCount)
	assert.Equal(t, int64(1), metrics.PluginExecutions)
	assert.Equal(t, int64(1), metrics.HTTPRequests)
	assert.Equal(t, int64(1), metrics.WebSocketConnections)

	// Verify timing
	assert.Equal(t, 100*time.Millisecond, metrics.AverageRenderTime)
	assert.Equal(t, 50*time.Millisecond, metrics.PluginLoadDuration)

	// Trigger metrics update to populate memory metrics
	monitor.updateMetrics()
	metrics = monitor.GetMetrics()

	// Verify memory metrics are populated
	assert.GreaterOrEqual(t, metrics.MemoryUsage, int64(0))
	assert.GreaterOrEqual(t, metrics.GoroutineCount, 1) // At least current goroutine
	assert.GreaterOrEqual(t, metrics.HeapSize, int64(0))
}

func TestPerformanceMonitor_GetUptime(t *testing.T) {
	monitor := NewPerformanceMonitor()

	// Sleep a bit to ensure uptime is measurable
	time.Sleep(10 * time.Millisecond)

	uptime := monitor.GetUptime()
	assert.Greater(t, uptime, 10*time.Millisecond)
	assert.Less(t, uptime, 1*time.Second) // Should not be too long for a test
}

func TestPerformanceMonitor_HealthCheck(t *testing.T) {
	monitor := NewPerformanceMonitor()

	t.Run("is healthy", func(t *testing.T) {
		healthy := monitor.IsHealthy()
		assert.True(t, healthy, "Monitor should be healthy in test environment")
	})

	t.Run("get health status", func(t *testing.T) {
		status := monitor.GetHealthStatus()

		assert.Contains(t, status, "healthy")
		assert.Contains(t, status, "uptime")
		assert.Contains(t, status, "memory_mb")
		assert.Contains(t, status, "heap_mb")
		assert.Contains(t, status, "goroutines")
		assert.Contains(t, status, "gc_cycles")
		assert.Contains(t, status, "operations")
		assert.Contains(t, status, "performance")

		// Verify operations sub-map
		operations, ok := status["operations"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, operations, "slides_rendered")
		assert.Contains(t, operations, "plugins_executed")
		assert.Contains(t, operations, "http_requests")
		assert.Contains(t, operations, "websocket_connections")

		// Verify performance sub-map
		performance, ok := status["performance"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, performance, "avg_render_time_ms")
		assert.Contains(t, performance, "plugin_load_time_ms")
	})
}

func TestPerformanceMonitor_MemoryStats(t *testing.T) {
	monitor := NewPerformanceMonitor()

	stats := monitor.GetMemoryStats()

	expectedKeys := []string{
		"alloc_mb", "total_alloc_mb", "sys_mb", "heap_alloc_mb",
		"heap_sys_mb", "heap_objects", "stack_inuse_mb", "gc_cycles",
		"gc_pause_ns", "next_gc_mb",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, stats, key, "Memory stats should contain %s", key)
	}

	// Verify values are reasonable
	assert.GreaterOrEqual(t, stats["alloc_mb"], int64(0))
	assert.GreaterOrEqual(t, stats["total_alloc_mb"], int64(0))
	assert.GreaterOrEqual(t, stats["sys_mb"], int64(0))
	assert.GreaterOrEqual(t, stats["heap_objects"], int64(0))
}

func TestPerformanceMonitor_TriggerGC(t *testing.T) {
	monitor := NewPerformanceMonitor()

	beforeGC := monitor.GetMetrics().GCCount
	monitor.TriggerGC()
	afterGC := monitor.GetMetrics().GCCount

	// GC count should have increased
	assert.GreaterOrEqual(t, afterGC, beforeGC)
}

func TestPerformanceMonitor_ConcurrentAccess(t *testing.T) {
	monitor := NewPerformanceMonitor()

	// Test concurrent recording of operations
	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				monitor.RecordSlideRender(time.Millisecond)
				monitor.RecordPluginExecution(time.Millisecond)
				monitor.RecordHTTPRequest()
				monitor.RecordWebSocketConnection()
			}
		}()
	}

	wg.Wait()

	metrics := monitor.GetMetrics()
	expectedOperations := int64(numGoroutines * operationsPerGoroutine)

	assert.Equal(t, expectedOperations, metrics.SlideRenderCount)
	assert.Equal(t, expectedOperations, metrics.PluginExecutions)
	assert.Equal(t, expectedOperations, metrics.HTTPRequests)
	assert.Equal(t, expectedOperations, metrics.WebSocketConnections)
}

func TestPerformanceMonitor_MetricCollection(t *testing.T) {
	monitor := NewPerformanceMonitor()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Record the time before starting to have a valid comparison baseline
	time.Sleep(1 * time.Millisecond) // Ensure some time passes
	beforeStart := time.Now()

	monitor.Start(ctx)
	defer monitor.Stop()

	// Override ticker to collect metrics faster for testing after starting
	monitor.mu.Lock()
	if monitor.ticker != nil {
		monitor.ticker.Stop()
	}
	monitor.ticker = time.NewTicker(10 * time.Millisecond)
	monitor.mu.Unlock()
	defer monitor.ticker.Stop()

	// Wait for at least one metric collection cycle
	time.Sleep(50 * time.Millisecond)

	afterStart := monitor.GetMetrics().LastOperationTime

	// LastOperationTime should have been updated by the collection loop
	assert.True(t, afterStart.After(beforeStart), "Metrics should be updated by collection loop")
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected int64
	}{
		{
			name:     "small value",
			input:    1024,
			expected: 1024,
		},
		{
			name:     "max int64 value",
			input:    9223372036854775807, // math.MaxInt64
			expected: 9223372036854775807,
		},
		{
			name:     "overflow value",
			input:    18446744073709551615, // math.MaxUint64
			expected: 9223372036854775807,  // Should be capped at MaxInt64
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeUint64ToInt64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPerformanceMonitor_ContextCancellation(t *testing.T) {
	monitor := NewPerformanceMonitor()
	ctx, cancel := context.WithCancel(context.Background())

	monitor.Start(ctx)
	assert.True(t, monitor.running)

	// Cancel context to stop collection loop
	cancel()

	// Give some time for the goroutine to exit
	time.Sleep(10 * time.Millisecond)

	// Monitor should still be running (Stop() hasn't been called)
	// but the collection goroutine should have exited due to context cancellation
	assert.True(t, monitor.running)

	monitor.Stop()
	assert.False(t, monitor.running)
}
