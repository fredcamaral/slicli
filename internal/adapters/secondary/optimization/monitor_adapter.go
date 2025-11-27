package optimization

import (
	"time"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/monitoring"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// PerformanceMonitorAdapter wraps PerformanceMonitor to implement ports.PerformanceMonitor.
type PerformanceMonitorAdapter struct {
	monitor *monitoring.PerformanceMonitor
}

// Ensure PerformanceMonitorAdapter implements ports.PerformanceMonitor.
var _ ports.PerformanceMonitor = (*PerformanceMonitorAdapter)(nil)

// WrapPerformanceMonitor wraps an existing PerformanceMonitor.
func WrapPerformanceMonitor(monitor *monitoring.PerformanceMonitor) *PerformanceMonitorAdapter {
	return &PerformanceMonitorAdapter{
		monitor: monitor,
	}
}

// RecordPluginExecution implements ports.PerformanceMonitor.
func (a *PerformanceMonitorAdapter) RecordPluginExecution(duration time.Duration) {
	a.monitor.RecordPluginExecution(duration)
}

// GetMetrics implements ports.PerformanceMonitor.
func (a *PerformanceMonitorAdapter) GetMetrics() interface{} {
	return a.monitor.GetMetrics()
}

// GetHealthStatus implements ports.PerformanceMonitor.
func (a *PerformanceMonitorAdapter) GetHealthStatus() map[string]interface{} {
	return a.monitor.GetHealthStatus()
}

// GetMemoryStats implements ports.PerformanceMonitor.
func (a *PerformanceMonitorAdapter) GetMemoryStats() map[string]interface{} {
	return a.monitor.GetMemoryStats()
}

// GetUnderlyingMonitor returns the wrapped PerformanceMonitor for backward compatibility.
func (a *PerformanceMonitorAdapter) GetUnderlyingMonitor() *monitoring.PerformanceMonitor {
	return a.monitor
}
