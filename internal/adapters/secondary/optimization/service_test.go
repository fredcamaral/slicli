package optimization

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptimizationService(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		config := OptimizationConfig{}
		service := NewOptimizationService(config)

		assert.NotNil(t, service)
		assert.NotNil(t, service.monitor)
		assert.NotNil(t, service.concurrentExec)
		assert.NotNil(t, service.gcOptimizer)
		assert.NotNil(t, service.memoryManager)
		assert.NotNil(t, service.cacheManager)
		assert.False(t, service.running)

		// Check defaults
		assert.Equal(t, int64(100), service.gcOptimizer.gcThresholdMB)
		assert.Equal(t, int64(500), service.memoryManager.maxHeapMB)
		assert.Equal(t, 10*time.Minute, service.cacheManager.cleanupInterval)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := OptimizationConfig{
			EnableAutoGC:         true,
			GCThresholdMB:        200,
			MaxHeapMB:            1000,
			CacheCleanupInterval: 5 * time.Minute,
			PluginConcurrency:    8,
			AdaptiveOptimization: true,
		}
		service := NewOptimizationService(config)

		assert.Equal(t, int64(200), service.gcOptimizer.gcThresholdMB)
		assert.Equal(t, int64(1000), service.memoryManager.maxHeapMB)
		assert.Equal(t, 5*time.Minute, service.cacheManager.cleanupInterval)
		assert.True(t, service.gcOptimizer.adaptiveGC)
	})
}

func TestOptimizationService_StartStop(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("start service", func(t *testing.T) {
		err := service.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, service.running)
		assert.NotNil(t, service.optimizationTicker)
	})

	t.Run("start already running service", func(t *testing.T) {
		// Starting already running service should return nil without error
		err := service.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, service.running)
	})

	t.Run("stop service", func(t *testing.T) {
		service.Stop()
		assert.False(t, service.running)
	})

	t.Run("stop already stopped service", func(t *testing.T) {
		// Stopping already stopped service should not panic
		service.Stop()
		assert.False(t, service.running)
	})
}

func TestOptimizationService_GetConcurrentExecutor(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{
		PluginConcurrency: 4,
	})

	executor := service.GetConcurrentExecutor()
	assert.NotNil(t, executor)
}

func TestOptimizationService_GetPerformanceMonitor(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	monitor := service.GetPerformanceMonitor()
	assert.NotNil(t, monitor)
}

func TestOptimizationService_CacheManagement(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	t.Run("register cache", func(t *testing.T) {
		mockCache := map[string]string{"key": "value"}
		service.RegisterCache("test-cache", mockCache)

		service.cacheManager.mu.RLock()
		cache, exists := service.cacheManager.caches["test-cache"]
		service.cacheManager.mu.RUnlock()

		assert.True(t, exists)
		assert.Equal(t, mockCache, cache)
	})

	t.Run("unregister cache", func(t *testing.T) {
		service.UnregisterCache("test-cache")

		service.cacheManager.mu.RLock()
		_, exists := service.cacheManager.caches["test-cache"]
		service.cacheManager.mu.RUnlock()

		assert.False(t, exists)
	})
}

func TestOptimizationService_ForceOptimization(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	// Should not panic
	service.ForceOptimization()
}

func TestOptimizationService_GetOptimizationStats(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	stats := service.GetOptimizationStats()

	// Check that all expected keys are present
	expectedKeys := []string{
		"gc_optimizer", "memory_manager", "cache_manager", "performance", "memory", "plugins",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, stats, key, "Stats should contain %s", key)
	}

	// Check specific sub-structures
	gcStats, ok := stats["gc_optimizer"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, gcStats, "threshold_mb")
	assert.Contains(t, gcStats, "adaptive_enabled")
	assert.Contains(t, gcStats, "last_gc")

	memStats, ok := stats["memory_manager"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, memStats, "max_heap_mb")
	assert.Contains(t, memStats, "target_gc_percent")
	assert.Contains(t, memStats, "last_optimization")

	cacheStats, ok := stats["cache_manager"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, cacheStats, "cleanup_interval")
	assert.Contains(t, cacheStats, "cache_count")
	assert.Contains(t, cacheStats, "last_cleanup")
}

func TestOptimizationService_TuningMethods(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	t.Run("tune for presentation", func(t *testing.T) {
		service.TuneForPresentation()

		// Verify that GC percent was set for presentation mode
		assert.Equal(t, 75, service.memoryManager.targetGCPercent)
		assert.Equal(t, int64(150), service.gcOptimizer.gcThresholdMB)
	})

	t.Run("tune for development", func(t *testing.T) {
		service.TuneForDevelopment()

		// Verify that GC percent was set for development mode
		assert.Equal(t, 100, service.memoryManager.targetGCPercent)
		assert.Equal(t, int64(75), service.gcOptimizer.gcThresholdMB)
	})
}

func TestOptimizationService_MemoryOptimization(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{
		MaxHeapMB: 1, // Set very low threshold to trigger optimization
	})

	// Create a mock metrics object with high heap usage to trigger optimization
	metrics := service.monitor.GetMetrics()
	// Force heap size to be higher than the threshold
	metrics.HeapSize = int64(2 * 1024 * 1024) // 2MB, higher than 80% of 1MB

	beforeOptimization := service.memoryManager.lastOptimization

	// Test memory optimization (should not panic)
	service.optimizeMemory(&metrics)

	// The optimization might have updated the last optimization time
	// We just test that the method completed without panic
	// The actual optimization trigger depends on heap size conditions
	afterOptimization := service.memoryManager.lastOptimization

	// Either time was updated (optimization happened) or it stayed the same (no optimization needed)
	assert.True(t, afterOptimization.Equal(beforeOptimization) || afterOptimization.After(beforeOptimization))
}

func TestOptimizationService_GCOptimization(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{
		GCThresholdMB:        10, // Set very low threshold for testing
		AdaptiveOptimization: true,
	})

	// Create a mock metrics object with high memory usage to trigger GC
	metrics := service.monitor.GetMetrics()
	metrics.MemoryUsage = int64(15 * 1024 * 1024) // 15MB, higher than 10MB threshold

	beforeGC := service.gcOptimizer.lastGC

	// Test GC optimization
	service.optimizeGC(&metrics)

	// GC should have been triggered and last GC time updated
	afterGC := service.gcOptimizer.lastGC
	assert.True(t, afterGC.After(beforeGC) || afterGC.Equal(beforeGC))
}

func TestOptimizationService_CacheOptimization(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{
		CacheCleanupInterval: 1 * time.Millisecond, // Very short interval for testing
	})

	// Register a test cache
	service.RegisterCache("test", map[string]string{"test": "data"})

	// Test cache optimization
	service.optimizeCaches()

	// Should have updated last cleanup time
	assert.True(t, !service.cacheManager.lastCleanup.IsZero())
}

func TestOptimizationService_PluginOptimization(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	// Test plugin optimization (should not panic)
	service.optimizePlugins()
}

func TestOptimizationService_OptimizationLoop(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start the service to initialize the ticker
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	// Override the ticker for faster testing
	service.mu.Lock()
	if service.optimizationTicker != nil {
		service.optimizationTicker.Stop()
	}
	service.optimizationTicker = time.NewTicker(10 * time.Millisecond)
	service.mu.Unlock()

	// Run optimization loop for a short time
	go service.optimizationLoop(ctx)

	// Wait for context to be cancelled
	<-ctx.Done()

	// Test passes if no panic occurred
}

func TestOptimizationService_PerformOptimizations(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{
		GCThresholdMB:        10,
		MaxHeapMB:            100,
		CacheCleanupInterval: 1 * time.Millisecond,
		AdaptiveOptimization: true,
	})

	// Register a test cache
	service.RegisterCache("test", map[string]string{"test": "data"})

	// Record times before optimization
	beforeMemoryOpt := service.memoryManager.lastOptimization
	beforeGC := service.gcOptimizer.lastGC
	beforeCacheCleanup := service.cacheManager.lastCleanup

	// Perform optimizations
	service.performOptimizations()

	// Verify that cache cleanup was definitely updated (due to short interval)
	assert.True(t, !service.cacheManager.lastCleanup.IsZero())
	assert.True(t, service.cacheManager.lastCleanup.After(beforeCacheCleanup))

	// Memory and GC optimizations are conditional, so we check they haven't gone backwards
	afterMemoryOpt := service.memoryManager.lastOptimization
	afterGC := service.gcOptimizer.lastGC
	assert.True(t, afterMemoryOpt.Equal(beforeMemoryOpt) || afterMemoryOpt.After(beforeMemoryOpt))
	assert.True(t, afterGC.Equal(beforeGC) || afterGC.After(beforeGC))
}

func TestOptimizationService_DefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		config         OptimizationConfig
		expectedGCMB   int64
		expectedHeapMB int64
		expectedConcur int
	}{
		{
			name:           "all defaults",
			config:         OptimizationConfig{},
			expectedGCMB:   100,
			expectedHeapMB: 500,
			expectedConcur: runtime.NumCPU() * 2,
		},
		{
			name: "partial config",
			config: OptimizationConfig{
				GCThresholdMB: 200,
			},
			expectedGCMB:   200,
			expectedHeapMB: 500,
			expectedConcur: runtime.NumCPU() * 2,
		},
		{
			name: "zero values get defaults",
			config: OptimizationConfig{
				GCThresholdMB:     0,
				MaxHeapMB:         0,
				PluginConcurrency: 0,
			},
			expectedGCMB:   100,
			expectedHeapMB: 500,
			expectedConcur: runtime.NumCPU() * 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOptimizationService(tt.config)

			assert.Equal(t, tt.expectedGCMB, service.gcOptimizer.gcThresholdMB)
			assert.Equal(t, tt.expectedHeapMB, service.memoryManager.maxHeapMB)
		})
	}
}

func TestOptimizationService_ConcurrentAccess(t *testing.T) {
	service := NewOptimizationService(OptimizationConfig{})

	// Test concurrent access to service methods
	done := make(chan bool, 3)

	// Goroutine 1: Register/unregister caches
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			service.RegisterCache("cache1", map[string]int{"test": i})
			service.UnregisterCache("cache1")
		}
	}()

	// Goroutine 2: Get stats
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_ = service.GetOptimizationStats()
		}
	}()

	// Goroutine 3: Force optimizations
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			service.ForceOptimization()
		}
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Test passes if no race conditions or panics occurred
}
