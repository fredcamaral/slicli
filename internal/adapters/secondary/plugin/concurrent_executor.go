package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// ConcurrentExecutor provides concurrent plugin execution with optimizations
type ConcurrentExecutor struct {
	maxConcurrent int
	semaphore     chan struct{}
	resultCache   *sync.Map // Cache for plugin results
	activeJobs    *sync.Map // Track active plugin executions
	mu            sync.RWMutex
}

// ExecutionJob represents a plugin execution job
type ExecutionJob struct {
	ID        string
	Plugin    entities.PluginInstance
	Input     pluginapi.PluginInput
	Result    chan ExecutionResult
	StartTime time.Time
	Timeout   time.Duration
}

// ExecutionResult contains the result of a plugin execution
type ExecutionResult struct {
	Output   pluginapi.PluginOutput
	Error    error
	Duration time.Duration
	Cached   bool
}

// BatchExecutionResult contains results from batch execution
type BatchExecutionResult struct {
	Results   map[string]ExecutionResult
	TotalTime time.Duration
	Success   int
	Failures  int
}

// cachedResult wraps a result with timestamp for cache expiration
type cachedResult struct {
	result    ExecutionResult
	timestamp time.Time
}

// NewConcurrentExecutor creates a new concurrent executor
func NewConcurrentExecutor(maxConcurrent int) *ConcurrentExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	return &ConcurrentExecutor{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		resultCache:   &sync.Map{},
		activeJobs:    &sync.Map{},
	}
}

// ExecuteConcurrent executes multiple plugins concurrently
func (e *ConcurrentExecutor) ExecuteConcurrent(ctx context.Context, jobs []ExecutionJob) *BatchExecutionResult {
	startTime := time.Now()
	results := make(map[string]ExecutionResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Execute all jobs concurrently
	for _, job := range jobs {
		wg.Add(1)
		go func(j ExecutionJob) {
			defer wg.Done()
			result := e.executeJob(ctx, j)

			mu.Lock()
			results[j.ID] = result
			mu.Unlock()
		}(job)
	}

	// Wait for all jobs to complete
	wg.Wait()

	// Calculate summary statistics
	var success, failures int
	for _, result := range results {
		if result.Error == nil {
			success++
		} else {
			failures++
		}
	}

	return &BatchExecutionResult{
		Results:   results,
		TotalTime: time.Since(startTime),
		Success:   success,
		Failures:  failures,
	}
}

// executeJob executes a single plugin job with caching and concurrency control
func (e *ConcurrentExecutor) executeJob(ctx context.Context, job ExecutionJob) ExecutionResult {
	// Check cache first
	cacheKey := e.generateCacheKey(job.Plugin.Metadata.Name, job.Input)
	if cached, found := e.resultCache.Load(cacheKey); found {
		if cachedItem, ok := cached.(cachedResult); ok {
			// Check if cache is still valid (5 minutes TTL)
			if time.Since(cachedItem.timestamp) < 5*time.Minute {
				result := cachedItem.result
				result.Cached = true
				return result
			}
			// Remove expired cache entry
			e.resultCache.Delete(cacheKey)
		}
	}

	// Acquire semaphore for concurrency control
	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-ctx.Done():
		return ExecutionResult{
			Error: ctx.Err(),
		}
	}

	// Track active job
	e.activeJobs.Store(job.ID, job)
	defer e.activeJobs.Delete(job.ID)

	// Execute with timeout
	executeCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	startTime := time.Now()
	output, err := e.executePlugin(executeCtx, job.Plugin, job.Input)
	duration := time.Since(startTime)

	result := ExecutionResult{
		Output:   output,
		Error:    err,
		Duration: duration,
		Cached:   false,
	}

	// Cache successful results
	if err == nil {
		cached := cachedResult{
			result:    result,
			timestamp: time.Now(),
		}
		e.resultCache.Store(cacheKey, cached)
	}

	return result
}

// executePlugin performs the actual plugin execution
func (e *ConcurrentExecutor) executePlugin(ctx context.Context, plugin entities.PluginInstance, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	// Create execution context with proper error handling
	done := make(chan struct{})
	var output pluginapi.PluginOutput
	var err error

	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("plugin panic: %v", r)
			}
		}()

		// Execute the plugin
		output, err = plugin.Instance.Execute(ctx, input)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		return output, err
	case <-ctx.Done():
		return pluginapi.PluginOutput{}, ctx.Err()
	}
}

// ExecuteWithPriority executes plugins with priority ordering
func (e *ConcurrentExecutor) ExecuteWithPriority(ctx context.Context, prioritizedJobs [][]ExecutionJob) *BatchExecutionResult {
	startTime := time.Now()
	allResults := make(map[string]ExecutionResult)
	var totalSuccess, totalFailures int

	// Execute each priority level sequentially, but jobs within each level concurrently
	for _, jobGroup := range prioritizedJobs {
		if len(jobGroup) == 0 {
			continue
		}

		groupResult := e.ExecuteConcurrent(ctx, jobGroup)

		// Merge results
		for id, result := range groupResult.Results {
			allResults[id] = result
		}

		totalSuccess += groupResult.Success
		totalFailures += groupResult.Failures
	}

	return &BatchExecutionResult{
		Results:   allResults,
		TotalTime: time.Since(startTime),
		Success:   totalSuccess,
		Failures:  totalFailures,
	}
}

// GetActiveJobs returns currently executing jobs
func (e *ConcurrentExecutor) GetActiveJobs() []ExecutionJob {
	var jobs []ExecutionJob

	e.activeJobs.Range(func(key, value interface{}) bool {
		if job, ok := value.(ExecutionJob); ok {
			jobs = append(jobs, job)
		}
		return true
	})

	return jobs
}

// GetCacheStats returns cache statistics
func (e *ConcurrentExecutor) GetCacheStats() map[string]interface{} {
	var cacheSize int
	e.resultCache.Range(func(key, value interface{}) bool {
		cacheSize++
		return true
	})

	return map[string]interface{}{
		"cache_size":     cacheSize,
		"max_concurrent": e.maxConcurrent,
		"active_jobs":    len(e.GetActiveJobs()),
	}
}

// ClearCache clears the result cache
func (e *ConcurrentExecutor) ClearCache() {
	e.resultCache = &sync.Map{}
}

// ClearExpiredCache removes expired cache entries
func (e *ConcurrentExecutor) ClearExpiredCache() {
	now := time.Now()

	e.resultCache.Range(func(key, value interface{}) bool {
		if cached, ok := value.(cachedResult); ok {
			// If result is older than 5 minutes, remove it
			if now.Sub(cached.timestamp) > 5*time.Minute {
				e.resultCache.Delete(key)
			}
		}
		return true
	})
}

// generateCacheKey generates a cache key for plugin execution
func (e *ConcurrentExecutor) generateCacheKey(pluginName string, input pluginapi.PluginInput) string {
	// Simple hash of plugin name and input content
	return fmt.Sprintf("%s:%s:%s", pluginName, input.Language, input.Content)
}

// OptimizeForContent analyzes content and suggests optimal execution strategy
func (e *ConcurrentExecutor) OptimizeForContent(plugins []entities.PluginInstance, content string) [][]ExecutionJob {
	var priorityGroups [][]ExecutionJob

	// High priority: Essential plugins (syntax highlighting, code execution)
	var highPriority []ExecutionJob
	// Medium priority: Enhancement plugins (mermaid diagrams)
	var mediumPriority []ExecutionJob
	// Low priority: Optional plugins
	var lowPriority []ExecutionJob

	for _, plugin := range plugins {
		job := ExecutionJob{
			ID:        plugin.Metadata.Name,
			Plugin:    plugin,
			Input:     pluginapi.PluginInput{Content: content},
			Result:    make(chan ExecutionResult, 1),
			StartTime: time.Now(),
			Timeout:   5 * time.Second,
		}

		// Categorize based on plugin type or content analysis
		switch plugin.Metadata.Name {
		case "syntax-highlight", "code-exec":
			highPriority = append(highPriority, job)
		case "mermaid":
			mediumPriority = append(mediumPriority, job)
		default:
			lowPriority = append(lowPriority, job)
		}
	}

	// Only add non-empty groups
	if len(highPriority) > 0 {
		priorityGroups = append(priorityGroups, highPriority)
	}
	if len(mediumPriority) > 0 {
		priorityGroups = append(priorityGroups, mediumPriority)
	}
	if len(lowPriority) > 0 {
		priorityGroups = append(priorityGroups, lowPriority)
	}

	return priorityGroups
}

// SetMaxConcurrent updates the maximum concurrent execution limit
func (e *ConcurrentExecutor) SetMaxConcurrent(max int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if max <= 0 {
		max = 10
	}

	e.maxConcurrent = max
	e.semaphore = make(chan struct{}, max)
}
