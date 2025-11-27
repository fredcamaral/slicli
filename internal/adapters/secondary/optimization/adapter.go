package optimization

import (
	"context"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/plugin"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// OptimizationServiceAdapter wraps OptimizationService to implement ports.OptimizationService.
type OptimizationServiceAdapter struct {
	service             *OptimizationService
	concurrentExecAdapt *plugin.ConcurrentExecutorAdapter
	monitorAdapter      *PerformanceMonitorAdapter
}

// Ensure OptimizationServiceAdapter implements ports.OptimizationService.
var _ ports.OptimizationService = (*OptimizationServiceAdapter)(nil)

// NewOptimizationServiceAdapter creates a new adapter wrapping an OptimizationService.
func NewOptimizationServiceAdapter(config OptimizationConfig) *OptimizationServiceAdapter {
	svc := NewOptimizationService(config)
	return &OptimizationServiceAdapter{
		service:             svc,
		concurrentExecAdapt: plugin.WrapConcurrentExecutor(svc.concurrentExec),
		monitorAdapter:      WrapPerformanceMonitor(svc.monitor),
	}
}

// WrapOptimizationService wraps an existing OptimizationService.
func WrapOptimizationService(svc *OptimizationService) *OptimizationServiceAdapter {
	return &OptimizationServiceAdapter{
		service:             svc,
		concurrentExecAdapt: plugin.WrapConcurrentExecutor(svc.concurrentExec),
		monitorAdapter:      WrapPerformanceMonitor(svc.monitor),
	}
}

// Start implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) Start(ctx context.Context) error {
	return a.service.Start(ctx)
}

// Stop implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) Stop() {
	a.service.Stop()
}

// GetConcurrentExecutor implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) GetConcurrentExecutor() ports.ConcurrentPluginExecutor {
	return a.concurrentExecAdapt
}

// GetPerformanceMonitor implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) GetPerformanceMonitor() ports.PerformanceMonitor {
	return a.monitorAdapter
}

// ForceOptimization implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) ForceOptimization() {
	a.service.ForceOptimization()
}

// GetOptimizationStats implements ports.OptimizationService.
func (a *OptimizationServiceAdapter) GetOptimizationStats() map[string]interface{} {
	return a.service.GetOptimizationStats()
}

// GetUnderlyingService returns the wrapped OptimizationService for backward compatibility.
func (a *OptimizationServiceAdapter) GetUnderlyingService() *OptimizationService {
	return a.service
}
