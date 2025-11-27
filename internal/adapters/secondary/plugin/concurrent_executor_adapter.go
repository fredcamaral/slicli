package plugin

import (
	"context"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// ConcurrentExecutorAdapter wraps ConcurrentExecutor to implement ports.ConcurrentPluginExecutor.
type ConcurrentExecutorAdapter struct {
	executor *ConcurrentExecutor
}

// Ensure ConcurrentExecutorAdapter implements ports.ConcurrentPluginExecutor.
var _ ports.ConcurrentPluginExecutor = (*ConcurrentExecutorAdapter)(nil)

// NewConcurrentExecutorAdapter creates a new adapter wrapping a ConcurrentExecutor.
func NewConcurrentExecutorAdapter(maxConcurrent int) *ConcurrentExecutorAdapter {
	return &ConcurrentExecutorAdapter{
		executor: NewConcurrentExecutor(maxConcurrent),
	}
}

// WrapConcurrentExecutor wraps an existing ConcurrentExecutor.
func WrapConcurrentExecutor(exec *ConcurrentExecutor) *ConcurrentExecutorAdapter {
	return &ConcurrentExecutorAdapter{
		executor: exec,
	}
}

// ExecuteWithPriority implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) ExecuteWithPriority(ctx context.Context, prioritizedJobs [][]ports.PluginJob) ports.BatchResult {
	// Convert ports.PluginJob to ExecutionJob
	var convertedJobs [][]ExecutionJob
	for _, group := range prioritizedJobs {
		var jobGroup []ExecutionJob
		for _, pj := range group {
			jobGroup = append(jobGroup, ExecutionJob{
				ID:        pj.ID,
				Plugin:    pj.Plugin,
				Input:     pj.Input,
				Result:    make(chan ExecutionResult, 1),
				StartTime: pj.StartTime,
				Timeout:   pj.Timeout,
			})
		}
		convertedJobs = append(convertedJobs, jobGroup)
	}

	// Execute using the underlying executor
	result := a.executor.ExecuteWithPriority(ctx, convertedJobs)

	// Convert BatchExecutionResult to ports.BatchResult
	portResults := make(map[string]ports.JobResult)
	for id, r := range result.Results {
		portResults[id] = ports.JobResult{
			Output:   r.Output,
			Error:    r.Error,
			Duration: r.Duration,
			Cached:   r.Cached,
		}
	}

	return ports.BatchResult{
		Results:   portResults,
		TotalTime: result.TotalTime,
		Success:   result.Success,
		Failures:  result.Failures,
	}
}

// OptimizeForContent implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) OptimizeForContent(plugins []entities.PluginInstance, content string) [][]ports.PluginJob {
	jobGroups := a.executor.OptimizeForContent(plugins, content)

	// Convert ExecutionJob to ports.PluginJob
	var result [][]ports.PluginJob
	for _, group := range jobGroups {
		var portGroup []ports.PluginJob
		for _, j := range group {
			portGroup = append(portGroup, ports.PluginJob{
				ID:        j.ID,
				Plugin:    j.Plugin,
				Input:     j.Input,
				StartTime: j.StartTime,
				Timeout:   j.Timeout,
			})
		}
		result = append(result, portGroup)
	}

	return result
}

// GetActiveJobs implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) GetActiveJobs() []ports.PluginJob {
	jobs := a.executor.GetActiveJobs()

	var result []ports.PluginJob
	for _, j := range jobs {
		result = append(result, ports.PluginJob{
			ID:        j.ID,
			Plugin:    j.Plugin,
			Input:     j.Input,
			StartTime: j.StartTime,
			Timeout:   j.Timeout,
		})
	}

	return result
}

// GetCacheStats implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) GetCacheStats() map[string]interface{} {
	return a.executor.GetCacheStats()
}

// ClearCache implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) ClearCache() {
	a.executor.ClearCache()
}

// ClearExpiredCache implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) ClearExpiredCache() {
	a.executor.ClearExpiredCache()
}

// SetMaxConcurrent implements ports.ConcurrentPluginExecutor.
func (a *ConcurrentExecutorAdapter) SetMaxConcurrent(max int) {
	a.executor.SetMaxConcurrent(max)
}

// GetUnderlyingExecutor returns the wrapped ConcurrentExecutor for backward compatibility.
func (a *ConcurrentExecutorAdapter) GetUnderlyingExecutor() *ConcurrentExecutor {
	return a.executor
}

// MemoryLimitedExecutorAdapter wraps MemoryLimitedExecutor to implement ports.MemoryLimitedPluginExecutor.
type MemoryLimitedExecutorAdapter struct {
	executor *MemoryLimitedExecutor
}

// Ensure MemoryLimitedExecutorAdapter implements ports.MemoryLimitedPluginExecutor.
var _ ports.MemoryLimitedPluginExecutor = (*MemoryLimitedExecutorAdapter)(nil)

// NewMemoryLimitedExecutorAdapter creates a new adapter wrapping a MemoryLimitedExecutor.
func NewMemoryLimitedExecutorAdapter(defaultTimeout time.Duration, maxConcurrent int, memoryLimit int64) (*MemoryLimitedExecutorAdapter, error) {
	exec, err := NewMemoryLimitedExecutor(defaultTimeout, maxConcurrent, memoryLimit)
	if err != nil {
		return nil, err
	}
	return &MemoryLimitedExecutorAdapter{
		executor: exec,
	}, nil
}

// WrapMemoryLimitedExecutor wraps an existing MemoryLimitedExecutor.
func WrapMemoryLimitedExecutor(exec *MemoryLimitedExecutor) *MemoryLimitedExecutorAdapter {
	return &MemoryLimitedExecutorAdapter{
		executor: exec,
	}
}

// ExecuteWithMemoryLimit implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) ExecuteWithMemoryLimit(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, timeout time.Duration) (pluginapi.PluginOutput, error) {
	return a.executor.ExecuteWithMemoryLimit(ctx, p, input, timeout)
}

// GetMemoryUsage implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) GetMemoryUsage() map[string]int64 {
	return a.executor.GetMemoryUsage()
}

// IsMemoryLimitingSupported implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) IsMemoryLimitingSupported() bool {
	return a.executor.IsMemoryLimitingSupported()
}

// Cleanup implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) Cleanup() error {
	return a.executor.Cleanup()
}

// GetActiveExecutions implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) GetActiveExecutions() map[string]time.Duration {
	return a.executor.GetActiveExecutions()
}

// SetMemoryEnforcementPolicy implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) SetMemoryEnforcementPolicy(enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64) {
	a.executor.SetMemoryEnforcementPolicy(enableEnforcement, killOnExceed, warningThreshold, criticalThreshold)
}

// GetMemoryEnforcementPolicy implements ports.MemoryLimitedPluginExecutor.
func (a *MemoryLimitedExecutorAdapter) GetMemoryEnforcementPolicy() (enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64) {
	return a.executor.GetMemoryEnforcementPolicy()
}

// GetUnderlyingExecutor returns the wrapped MemoryLimitedExecutor for backward compatibility.
func (a *MemoryLimitedExecutorAdapter) GetUnderlyingExecutor() *MemoryLimitedExecutor {
	return a.executor
}
