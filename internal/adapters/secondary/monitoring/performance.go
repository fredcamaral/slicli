package monitoring

import (
	"context"
	"math"
	"runtime"
	"sync"
	"time"
)

// PerformanceMetrics holds various performance measurements
type PerformanceMetrics struct {
	// Timing metrics
	AppStartTime        time.Time
	LastOperationTime   time.Time
	PluginLoadDuration  time.Duration
	RenderDuration      time.Duration
	ServerStartDuration time.Duration

	// Memory metrics
	MemoryUsage    int64
	GoroutineCount int
	HeapSize       int64
	StackSize      int64
	GCCount        uint32

	// Operation counters
	SlideRenderCount     int64
	PluginExecutions     int64
	HTTPRequests         int64
	WebSocketConnections int64

	// Performance indicators
	AverageRenderTime  time.Duration
	PluginCacheHitRate float64
	MemoryGrowthRate   float64

	mu sync.RWMutex
}

// PerformanceMonitor provides performance monitoring capabilities
type PerformanceMonitor struct {
	metrics *PerformanceMetrics
	ticker  *time.Ticker
	stopCh  chan struct{}
	running bool
	mu      sync.RWMutex
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		metrics: &PerformanceMetrics{
			AppStartTime: time.Now(),
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start(ctx context.Context) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return
	}

	pm.running = true
	pm.ticker = time.NewTicker(30 * time.Second) // Collect metrics every 30 seconds

	go pm.collectMetrics(ctx)
}

// Stop stops performance monitoring
func (pm *PerformanceMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return
	}

	pm.running = false
	if pm.ticker != nil {
		pm.ticker.Stop()
	}
	close(pm.stopCh)
}

// collectMetrics runs the metric collection loop
func (pm *PerformanceMonitor) collectMetrics(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-pm.ticker.C:
			pm.updateMetrics()
		}
	}
}

// updateMetrics updates all performance metrics
func (pm *PerformanceMonitor) updateMetrics() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	// Update memory metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Safe conversion with overflow check
	pm.metrics.MemoryUsage = safeUint64ToInt64(memStats.Alloc)
	pm.metrics.HeapSize = safeUint64ToInt64(memStats.HeapAlloc)
	pm.metrics.StackSize = safeUint64ToInt64(memStats.StackInuse)
	pm.metrics.GoroutineCount = runtime.NumGoroutine()

	// Update GC stats
	pm.metrics.GCCount = memStats.NumGC

	pm.metrics.LastOperationTime = time.Now()
}

// RecordSlideRender records a slide rendering operation
func (pm *PerformanceMonitor) RecordSlideRender(duration time.Duration) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.SlideRenderCount++

	// Update average render time
	if pm.metrics.AverageRenderTime == 0 {
		pm.metrics.AverageRenderTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		pm.metrics.AverageRenderTime = time.Duration(
			float64(pm.metrics.AverageRenderTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
}

// RecordPluginExecution records a plugin execution
func (pm *PerformanceMonitor) RecordPluginExecution(duration time.Duration) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.PluginExecutions++
	pm.metrics.PluginLoadDuration = duration
}

// RecordHTTPRequest records an HTTP request
func (pm *PerformanceMonitor) RecordHTTPRequest() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.HTTPRequests++
}

// RecordWebSocketConnection records a WebSocket connection
func (pm *PerformanceMonitor) RecordWebSocketConnection() {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.WebSocketConnections++
}

// GetMetrics returns a copy of current metrics
func (pm *PerformanceMonitor) GetMetrics() PerformanceMetrics {
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()

	// Return a copy without the mutex to avoid data races
	return PerformanceMetrics{
		AppStartTime:         pm.metrics.AppStartTime,
		LastOperationTime:    pm.metrics.LastOperationTime,
		PluginLoadDuration:   pm.metrics.PluginLoadDuration,
		RenderDuration:       pm.metrics.RenderDuration,
		ServerStartDuration:  pm.metrics.ServerStartDuration,
		MemoryUsage:          pm.metrics.MemoryUsage,
		GoroutineCount:       pm.metrics.GoroutineCount,
		HeapSize:             pm.metrics.HeapSize,
		StackSize:            pm.metrics.StackSize,
		GCCount:              pm.metrics.GCCount,
		SlideRenderCount:     pm.metrics.SlideRenderCount,
		PluginExecutions:     pm.metrics.PluginExecutions,
		HTTPRequests:         pm.metrics.HTTPRequests,
		WebSocketConnections: pm.metrics.WebSocketConnections,
		AverageRenderTime:    pm.metrics.AverageRenderTime,
		PluginCacheHitRate:   pm.metrics.PluginCacheHitRate,
		MemoryGrowthRate:     pm.metrics.MemoryGrowthRate,
	}
}

// GetUptime returns application uptime
func (pm *PerformanceMonitor) GetUptime() time.Duration {
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()

	return time.Since(pm.metrics.AppStartTime)
}

// IsHealthy performs a basic health check
func (pm *PerformanceMonitor) IsHealthy() bool {
	metrics := pm.GetMetrics()

	// Health criteria
	maxMemoryMB := int64(500 * 1024 * 1024) // 500MB
	maxGoroutines := 1000

	return metrics.MemoryUsage < maxMemoryMB &&
		metrics.GoroutineCount < maxGoroutines
}

// GetHealthStatus returns detailed health information
func (pm *PerformanceMonitor) GetHealthStatus() map[string]interface{} {
	metrics := pm.GetMetrics()
	uptime := pm.GetUptime()

	return map[string]interface{}{
		"healthy":    pm.IsHealthy(),
		"uptime":     uptime.String(),
		"memory_mb":  metrics.MemoryUsage / (1024 * 1024),
		"heap_mb":    metrics.HeapSize / (1024 * 1024),
		"goroutines": metrics.GoroutineCount,
		"gc_cycles":  metrics.GCCount,
		"operations": map[string]interface{}{
			"slides_rendered":       metrics.SlideRenderCount,
			"plugins_executed":      metrics.PluginExecutions,
			"http_requests":         metrics.HTTPRequests,
			"websocket_connections": metrics.WebSocketConnections,
		},
		"performance": map[string]interface{}{
			"avg_render_time_ms":  metrics.AverageRenderTime.Milliseconds(),
			"plugin_load_time_ms": metrics.PluginLoadDuration.Milliseconds(),
		},
	}
}

// TriggerGC forces garbage collection (use sparingly)
func (pm *PerformanceMonitor) TriggerGC() {
	runtime.GC()
	pm.updateMetrics()
}

// GetMemoryStats returns detailed memory statistics
func (pm *PerformanceMonitor) GetMemoryStats() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"alloc_mb":       safeUint64ToInt64(memStats.Alloc) / (1024 * 1024),
		"total_alloc_mb": safeUint64ToInt64(memStats.TotalAlloc) / (1024 * 1024),
		"sys_mb":         safeUint64ToInt64(memStats.Sys) / (1024 * 1024),
		"heap_alloc_mb":  safeUint64ToInt64(memStats.HeapAlloc) / (1024 * 1024),
		"heap_sys_mb":    safeUint64ToInt64(memStats.HeapSys) / (1024 * 1024),
		"heap_objects":   safeUint64ToInt64(memStats.HeapObjects),
		"stack_inuse_mb": safeUint64ToInt64(memStats.StackInuse) / (1024 * 1024),
		"gc_cycles":      memStats.NumGC,
		"gc_pause_ns":    memStats.PauseNs[(memStats.NumGC+255)%256],
		"next_gc_mb":     safeUint64ToInt64(memStats.NextGC) / (1024 * 1024),
	}
}

// safeUint64ToInt64 safely converts uint64 to int64, capping at max int64 value
func safeUint64ToInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}
