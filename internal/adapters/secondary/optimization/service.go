package optimization

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/monitoring"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/plugin"
)

// OptimizationService provides performance optimization capabilities
type OptimizationService struct {
	monitor            *monitoring.PerformanceMonitor
	concurrentExec     *plugin.ConcurrentExecutor
	gcOptimizer        *GCOptimizer
	memoryManager      *MemoryManager
	cacheManager       *CacheManager
	mu                 sync.RWMutex
	running            bool
	optimizationTicker *time.Ticker
}

// GCOptimizer manages garbage collection optimization
type GCOptimizer struct {
	lastGC        time.Time
	gcThresholdMB int64
	adaptiveGC    bool
	mu            sync.RWMutex
}

// MemoryManager handles memory optimization
type MemoryManager struct {
	maxHeapMB        int64
	targetGCPercent  int
	lastOptimization time.Time
	mu               sync.RWMutex
}

// CacheManager handles various caches
type CacheManager struct {
	caches          map[string]interface{}
	cleanupInterval time.Duration
	lastCleanup     time.Time
	mu              sync.RWMutex
}

// OptimizationConfig contains optimization settings
type OptimizationConfig struct {
	EnableAutoGC         bool
	GCThresholdMB        int64
	MaxHeapMB            int64
	CacheCleanupInterval time.Duration
	PluginConcurrency    int
	AdaptiveOptimization bool
}

// NewOptimizationService creates a new optimization service
func NewOptimizationService(config OptimizationConfig) *OptimizationService {
	// Set defaults
	if config.GCThresholdMB <= 0 {
		config.GCThresholdMB = 100 // 100MB
	}
	if config.MaxHeapMB <= 0 {
		config.MaxHeapMB = 500 // 500MB
	}
	if config.CacheCleanupInterval <= 0 {
		config.CacheCleanupInterval = 10 * time.Minute
	}
	if config.PluginConcurrency <= 0 {
		config.PluginConcurrency = runtime.NumCPU() * 2
	}

	return &OptimizationService{
		monitor:        monitoring.NewPerformanceMonitor(),
		concurrentExec: plugin.NewConcurrentExecutor(config.PluginConcurrency),
		gcOptimizer: &GCOptimizer{
			gcThresholdMB: config.GCThresholdMB,
			adaptiveGC:    config.AdaptiveOptimization,
		},
		memoryManager: &MemoryManager{
			maxHeapMB:       config.MaxHeapMB,
			targetGCPercent: 100,
		},
		cacheManager: &CacheManager{
			caches:          make(map[string]interface{}),
			cleanupInterval: config.CacheCleanupInterval,
		},
	}
}

// Start begins optimization monitoring and management
func (s *OptimizationService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.running = true

	// Start performance monitoring
	s.monitor.Start(ctx)

	// Start optimization loop
	s.optimizationTicker = time.NewTicker(30 * time.Second)
	go s.optimizationLoop(ctx)

	return nil
}

// Stop stops the optimization service
func (s *OptimizationService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.monitor.Stop()

	if s.optimizationTicker != nil {
		s.optimizationTicker.Stop()
	}
}

// optimizationLoop runs periodic optimizations
func (s *OptimizationService) optimizationLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.optimizationTicker.C:
			s.performOptimizations()
		}
	}
}

// performOptimizations runs all optimization routines
func (s *OptimizationService) performOptimizations() {
	metrics := s.monitor.GetMetrics()

	// Memory optimization
	s.optimizeMemory(&metrics)

	// Garbage collection optimization
	s.optimizeGC(&metrics)

	// Cache optimization
	s.optimizeCaches()

	// Plugin optimization
	s.optimizePlugins()
}

// optimizeMemory performs memory optimizations
func (s *OptimizationService) optimizeMemory(metrics *monitoring.PerformanceMetrics) {
	s.memoryManager.mu.Lock()
	defer s.memoryManager.mu.Unlock()

	heapMB := metrics.HeapSize / (1024 * 1024)

	// If heap usage is high, trigger GC
	if heapMB > s.memoryManager.maxHeapMB*80/100 { // 80% threshold
		runtime.GC()
		s.memoryManager.lastOptimization = time.Now()
	}

	// Adjust GC target based on current usage
	if metrics.GoroutineCount > 100 {
		// More aggressive GC when we have many goroutines
		runtime.GOMAXPROCS(runtime.NumCPU())
		if s.memoryManager.targetGCPercent > 50 {
			s.memoryManager.targetGCPercent = 50
			runtime.GC()
		}
	} else {
		// Less aggressive GC when we have fewer goroutines
		if s.memoryManager.targetGCPercent < 100 {
			s.memoryManager.targetGCPercent = 100
		}
	}
}

// optimizeGC performs garbage collection optimizations
func (s *OptimizationService) optimizeGC(metrics *monitoring.PerformanceMetrics) {
	s.gcOptimizer.mu.Lock()
	defer s.gcOptimizer.mu.Unlock()

	memoryMB := metrics.MemoryUsage / (1024 * 1024)
	timeSinceLastGC := time.Since(s.gcOptimizer.lastGC)

	shouldTriggerGC := false

	if s.gcOptimizer.adaptiveGC {
		// Adaptive GC based on memory pressure and time
		if memoryMB > s.gcOptimizer.gcThresholdMB && timeSinceLastGC > time.Minute {
			shouldTriggerGC = true
		} else if memoryMB > s.gcOptimizer.gcThresholdMB*2 {
			// Force GC if memory usage is very high
			shouldTriggerGC = true
		}
	} else {
		// Simple threshold-based GC
		if memoryMB > s.gcOptimizer.gcThresholdMB {
			shouldTriggerGC = true
		}
	}

	if shouldTriggerGC {
		runtime.GC()
		s.gcOptimizer.lastGC = time.Now()
	}
}

// optimizeCaches performs cache optimizations
func (s *OptimizationService) optimizeCaches() {
	s.cacheManager.mu.Lock()
	defer s.cacheManager.mu.Unlock()

	now := time.Now()
	if now.Sub(s.cacheManager.lastCleanup) < s.cacheManager.cleanupInterval {
		return
	}

	// Clean plugin execution cache
	s.concurrentExec.ClearExpiredCache()

	s.cacheManager.lastCleanup = now
}

// optimizePlugins performs plugin-related optimizations
func (s *OptimizationService) optimizePlugins() {
	// Adjust plugin concurrency based on system load
	metrics := s.monitor.GetMetrics()

	currentConcurrency := s.concurrentExec.GetCacheStats()["max_concurrent"].(int)
	targetConcurrency := runtime.NumCPU() * 2

	// Reduce concurrency if memory usage is high
	if metrics.MemoryUsage > 400*1024*1024 { // 400MB
		targetConcurrency = runtime.NumCPU()
	}

	// Increase concurrency if system is underutilized
	if metrics.GoroutineCount < 20 && metrics.MemoryUsage < 200*1024*1024 { // 200MB
		targetConcurrency = runtime.NumCPU() * 3
	}

	if targetConcurrency != currentConcurrency {
		s.concurrentExec.SetMaxConcurrent(targetConcurrency)
	}
}

// GetOptimizationStats returns optimization statistics
func (s *OptimizationService) GetOptimizationStats() map[string]interface{} {
	return map[string]interface{}{
		"performance": s.monitor.GetHealthStatus(),
		"memory":      s.monitor.GetMemoryStats(),
		"plugins":     s.concurrentExec.GetCacheStats(),
		"gc_optimizer": map[string]interface{}{
			"last_gc":          s.gcOptimizer.lastGC,
			"threshold_mb":     s.gcOptimizer.gcThresholdMB,
			"adaptive_enabled": s.gcOptimizer.adaptiveGC,
		},
		"memory_manager": map[string]interface{}{
			"max_heap_mb":       s.memoryManager.maxHeapMB,
			"target_gc_percent": s.memoryManager.targetGCPercent,
			"last_optimization": s.memoryManager.lastOptimization,
		},
		"cache_manager": map[string]interface{}{
			"cleanup_interval": s.cacheManager.cleanupInterval,
			"last_cleanup":     s.cacheManager.lastCleanup,
			"cache_count":      len(s.cacheManager.caches),
		},
	}
}

// ForceOptimization triggers immediate optimization
func (s *OptimizationService) ForceOptimization() {
	s.performOptimizations()
}

// GetConcurrentExecutor returns the concurrent executor for use by other services
func (s *OptimizationService) GetConcurrentExecutor() *plugin.ConcurrentExecutor {
	return s.concurrentExec
}

// GetPerformanceMonitor returns the performance monitor
func (s *OptimizationService) GetPerformanceMonitor() *monitoring.PerformanceMonitor {
	return s.monitor
}

// RegisterCache registers a cache for management
func (s *OptimizationService) RegisterCache(name string, cache interface{}) {
	s.cacheManager.mu.Lock()
	defer s.cacheManager.mu.Unlock()
	s.cacheManager.caches[name] = cache
}

// UnregisterCache removes a cache from management
func (s *OptimizationService) UnregisterCache(name string) {
	s.cacheManager.mu.Lock()
	defer s.cacheManager.mu.Unlock()
	delete(s.cacheManager.caches, name)
}

// TuneForPresentation optimizes settings for presentation mode
func (s *OptimizationService) TuneForPresentation() {
	s.gcOptimizer.mu.Lock()
	s.memoryManager.mu.Lock()
	defer s.gcOptimizer.mu.Unlock()
	defer s.memoryManager.mu.Unlock()

	// More conservative settings during presentation
	s.gcOptimizer.gcThresholdMB = 150
	s.memoryManager.targetGCPercent = 75
	s.concurrentExec.SetMaxConcurrent(runtime.NumCPU())
}

// TuneForDevelopment optimizes settings for development mode
func (s *OptimizationService) TuneForDevelopment() {
	s.gcOptimizer.mu.Lock()
	s.memoryManager.mu.Lock()
	defer s.gcOptimizer.mu.Unlock()
	defer s.memoryManager.mu.Unlock()

	// More aggressive settings during development
	s.gcOptimizer.gcThresholdMB = 75
	s.memoryManager.targetGCPercent = 100
	s.concurrentExec.SetMaxConcurrent(runtime.NumCPU() * 3)
}
