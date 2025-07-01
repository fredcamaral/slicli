package plugin

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/ports"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// SandboxExecutor executes plugins in a sandboxed environment with timeout and panic recovery.
type SandboxExecutor struct {
	defaultTimeout time.Duration
	maxConcurrent  int
	semaphore      chan struct{}
	mu             sync.Mutex
	executing      map[string]time.Time
}

// NewSandboxExecutor creates a new sandbox executor.
func NewSandboxExecutor(defaultTimeout time.Duration, maxConcurrent int) *SandboxExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default concurrent executions
	}
	return &SandboxExecutor{
		defaultTimeout: defaultTimeout,
		maxConcurrent:  maxConcurrent,
		semaphore:      make(chan struct{}, maxConcurrent),
		executing:      make(map[string]time.Time),
	}
}

// Execute runs a plugin with the default timeout.
func (e *SandboxExecutor) Execute(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	return e.ExecuteWithTimeout(ctx, p, input, e.defaultTimeout)
}

// ExecuteWithTimeout runs a plugin with a specific timeout.
func (e *SandboxExecutor) ExecuteWithTimeout(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// Acquire semaphore
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return pluginapi.PluginOutput{}, ctx.Err()
	}

	// Track execution
	e.mu.Lock()
	e.executing[p.Name()] = time.Now()
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.executing, p.Name())
		e.mu.Unlock()
	}()

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Result channel
	type result struct {
		output pluginapi.PluginOutput
		err    error
	}
	resultChan := make(chan result, 1)

	// Execute in goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace
				stack := debug.Stack()
				err := fmt.Errorf("plugin %s panicked: %v\nStack trace:\n%s", p.Name(), r, stack)
				resultChan <- result{err: err}
			}
		}()

		// Execute the plugin
		output, err := p.Execute(execCtx, input)
		resultChan <- result{output: output, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		if res.err != nil {
			// Don't wrap context cancellation errors
			if errors.Is(res.err, context.Canceled) {
				return pluginapi.PluginOutput{}, res.err
			}
			return pluginapi.PluginOutput{}, &pluginapi.PluginError{
				Plugin:    p.Name(),
				Operation: "execute",
				Err:       res.err,
			}
		}
		return res.output, nil

	case <-execCtx.Done():
		// Check if it was cancellation or timeout
		if errors.Is(execCtx.Err(), context.Canceled) {
			return pluginapi.PluginOutput{}, execCtx.Err()
		}
		// Timeout occurred
		return pluginapi.PluginOutput{}, &pluginapi.PluginError{
			Plugin:    p.Name(),
			Operation: "execute",
			Err:       fmt.Errorf("execution timeout after %v", timeout),
		}
	}
}

// ExecuteAsync runs a plugin asynchronously and returns a channel for the result.
func (e *SandboxExecutor) ExecuteAsync(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput) <-chan ports.PluginResult {
	resultChan := make(chan ports.PluginResult, 1)

	go func() {
		defer close(resultChan)

		output, err := e.Execute(ctx, p, input)
		resultChan <- ports.PluginResult{
			Output: output,
			Error:  err,
		}
	}()

	return resultChan
}

// GetExecutingPlugins returns the names of currently executing plugins.
func (e *SandboxExecutor) GetExecutingPlugins() map[string]time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make(map[string]time.Duration)
	now := time.Now()
	for name, startTime := range e.executing {
		result[name] = now.Sub(startTime)
	}
	return result
}

// MemoryLimitedExecutor wraps a plugin to enforce memory limits using OS-specific mechanisms.
type MemoryLimitedExecutor struct {
	*SandboxExecutor
	memoryLimit   int64
	memoryLimiter *MemoryLimiter

	// Memory enforcement policy
	enableLogging     bool    // Log memory usage warnings
	killOnExceed      bool    // Kill execution if memory limit is exceeded
	warningThreshold  float64 // Threshold for warnings (0.0-1.0)
	criticalThreshold float64 // Threshold for critical alerts (0.0-1.0)
}

// NewMemoryLimitedExecutor creates a new memory-limited executor.
func NewMemoryLimitedExecutor(defaultTimeout time.Duration, maxConcurrent int, memoryLimit int64) (*MemoryLimitedExecutor, error) {
	limiter, err := NewMemoryLimiter()
	if err != nil {
		return nil, fmt.Errorf("creating memory limiter: %w", err)
	}

	return &MemoryLimitedExecutor{
		SandboxExecutor: NewSandboxExecutor(defaultTimeout, maxConcurrent),
		memoryLimit:     memoryLimit,
		memoryLimiter:   limiter,

		// Default enforcement policies
		enableLogging:     true,  // Enable memory usage logging
		killOnExceed:      false, // Don't kill by default, let OS handle it
		warningThreshold:  0.8,   // Warn at 80% usage
		criticalThreshold: 0.9,   // Critical at 90% usage
	}, nil
}

// ExecuteWithMemoryLimit executes a plugin with memory limits using OS-specific mechanisms.
func (e *MemoryLimitedExecutor) ExecuteWithMemoryLimit(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, timeout time.Duration) (pluginapi.PluginOutput, error) {
	if e.memoryLimiter == nil {
		// Fallback to regular execution if memory limiter is not available
		return e.ExecuteWithTimeout(ctx, p, input, timeout)
	}

	// Use the memory limiter for execution
	return e.memoryLimiter.ExecuteWithMemoryLimit(ctx, p, input, e.memoryLimit, timeout)
}

// Cleanup cleans up any resources used by the memory-limited executor.
func (e *MemoryLimitedExecutor) Cleanup() error {
	if e.memoryLimiter != nil {
		return e.memoryLimiter.Cleanup()
	}
	return nil
}

// GetMemoryUsage returns current memory usage for active plugin executions.
func (e *MemoryLimitedExecutor) GetMemoryUsage() map[string]int64 {
	if e.memoryLimiter != nil {
		return e.memoryLimiter.GetMemoryUsage()
	}
	return make(map[string]int64)
}

// IsMemoryLimitingSupported returns whether memory limiting is supported on the current platform.
func (e *MemoryLimitedExecutor) IsMemoryLimitingSupported() bool {
	return IsMemoryLimitingAvailable()
}

// SetMemoryEnforcementPolicy configures memory monitoring and enforcement behavior
func (e *MemoryLimitedExecutor) SetMemoryEnforcementPolicy(enableLogging, killOnExceed bool, warningThreshold, criticalThreshold float64) {
	// Update local settings
	e.enableLogging = enableLogging
	e.killOnExceed = killOnExceed
	e.warningThreshold = warningThreshold
	e.criticalThreshold = criticalThreshold

	// Update the underlying memory limiter settings
	if e.memoryLimiter != nil {
		monitoringInterval := 100 * time.Millisecond // Default monitoring interval
		e.memoryLimiter.SetEnforcementPolicy(enableLogging, killOnExceed, warningThreshold, criticalThreshold, monitoringInterval)
	}
}

// GetMemoryEnforcementPolicy returns current memory enforcement policy settings
func (e *MemoryLimitedExecutor) GetMemoryEnforcementPolicy() (enableLogging, killOnExceed bool, warningThreshold, criticalThreshold float64) {
	// Get from underlying memory limiter if available
	if e.memoryLimiter != nil {
		enableEnforcement, killOnExceed, warningThreshold, criticalThreshold, _ := e.memoryLimiter.GetEnforcementPolicy()
		return enableEnforcement, killOnExceed, warningThreshold, criticalThreshold
	}

	// Fallback to local settings
	return e.enableLogging, e.killOnExceed, e.warningThreshold, e.criticalThreshold
}

// SetMonitoringInterval configures how often memory usage is checked
func (e *MemoryLimitedExecutor) SetMonitoringInterval(interval time.Duration) {
	if e.memoryLimiter != nil {
		enableEnforcement, killOnExceed, warningThreshold, criticalThreshold, _ := e.memoryLimiter.GetEnforcementPolicy()
		e.memoryLimiter.SetEnforcementPolicy(enableEnforcement, killOnExceed, warningThreshold, criticalThreshold, interval)
	}
}

// GetActiveExecutions returns information about currently running executions
func (e *MemoryLimitedExecutor) GetActiveExecutions() map[string]time.Duration {
	if e.SandboxExecutor != nil {
		return e.GetExecutingPlugins()
	}
	return make(map[string]time.Duration)
}
