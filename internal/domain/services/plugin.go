package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/optimization"
	concurrentplugin "github.com/fredcamaral/slicli/internal/adapters/secondary/plugin"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// PluginService manages plugin operations.
type PluginService struct {
	loader             ports.PluginLoader
	executor           ports.PluginExecutor
	memoryExecutor     *concurrentplugin.MemoryLimitedExecutor
	registry           ports.PluginRegistry
	cache              ports.PluginCache
	matcher            ports.PluginMatcher
	config             PluginServiceConfig
	concurrentExec     *concurrentplugin.ConcurrentExecutor
	optimizationSvc    *optimization.OptimizationService
	logger             *slog.Logger
	shutdownMu         sync.Mutex
	shutdown           bool
	memoryMonitoring   map[string]int64 // Track memory usage per plugin
	memoryMonitoringMu sync.RWMutex
}

// PluginServiceConfig contains configuration for the plugin service.
type PluginServiceConfig struct {
	PluginDirs        []string
	DefaultTimeout    time.Duration
	MaxConcurrent     int
	CacheEnabled      bool
	CacheTTL          time.Duration
	AutoDiscover      bool
	DiscoverOnStart   bool
	MemoryLimit       int64 // Memory limit per plugin execution in bytes (0 = no limit)
	EnableMemoryLimit bool  // Enable memory limiting if supported by platform
}

// NewPluginService creates a new plugin service.
func NewPluginService(
	loader ports.PluginLoader,
	executor ports.PluginExecutor,
	registry ports.PluginRegistry,
	cache ports.PluginCache,
	matcher ports.PluginMatcher,
	config PluginServiceConfig,
	logger *slog.Logger,
) *PluginService {
	if logger == nil {
		logger = slog.Default()
	}
	// Set defaults
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 5 * time.Second
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.MemoryLimit <= 0 {
		config.MemoryLimit = 100 * 1024 * 1024 // Default 100MB per plugin
	}

	service := &PluginService{
		loader:           loader,
		executor:         executor,
		registry:         registry,
		cache:            cache,
		matcher:          matcher,
		config:           config,
		logger:           logger.With("service", "plugin"),
		memoryMonitoring: make(map[string]int64),
	}

	// Initialize memory-limited executor if enabled and supported
	if config.EnableMemoryLimit && concurrentplugin.IsMemoryLimitingAvailable() {
		if memExec, err := concurrentplugin.NewMemoryLimitedExecutor(
			config.DefaultTimeout,
			config.MaxConcurrent,
			config.MemoryLimit,
		); err == nil {
			service.memoryExecutor = memExec
			service.logger.Info("Memory limiting enabled", slog.Int64("limit_mb", config.MemoryLimit/(1024*1024)))
		} else {
			service.logger.Error("Failed to initialize memory-limited executor", slog.String("error", err.Error()))
		}
	} else if config.EnableMemoryLimit {
		service.logger.Warn("Memory limiting requested but not available on this platform")
	}

	// Initialize concurrent executor if not provided by optimization service
	service.concurrentExec = concurrentplugin.NewConcurrentExecutor(config.MaxConcurrent)

	return service
}

// SetOptimizationService sets the optimization service and uses its concurrent executor
func (s *PluginService) SetOptimizationService(optimizationSvc *optimization.OptimizationService) {
	s.optimizationSvc = optimizationSvc
	if optimizationSvc != nil {
		// Use the optimization service's concurrent executor for better performance
		s.concurrentExec = optimizationSvc.GetConcurrentExecutor()
	}
}

// Initialize initializes the plugin service.
func (s *PluginService) Initialize(ctx context.Context) error {
	if s.config.DiscoverOnStart && s.config.AutoDiscover {
		_, err := s.DiscoverPlugins(ctx)
		return err
	}
	return nil
}

// LoadPlugin loads a plugin from a path.
func (s *PluginService) LoadPlugin(ctx context.Context, path string) error {
	s.shutdownMu.Lock()
	if s.shutdown {
		s.shutdownMu.Unlock()
		return errors.New("plugin service is shutting down")
	}
	s.shutdownMu.Unlock()

	// Load the plugin
	p, err := s.loader.Load(ctx, path)
	if err != nil {
		return fmt.Errorf("loading plugin from %s: %w", path, err)
	}

	// Load manifest if available
	manifestPath := filepath.Join(filepath.Dir(path), "plugin.toml")
	manifest, err := s.loader.LoadManifest(ctx, manifestPath)
	var metadata entities.PluginMetadata
	if err != nil {
		// Create metadata from plugin interface
		metadata = entities.PluginMetadata{
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			Type:        entities.PluginTypeProcessor, // Default type
		}
	} else {
		metadata = manifest.Metadata
	}

	// Initialize the plugin
	config := make(map[string]interface{})
	if manifest != nil && manifest.DefaultConfig.Options != nil {
		config = manifest.DefaultConfig.Options
	}
	if err := p.Init(config); err != nil {
		return fmt.Errorf("initializing plugin %s: %w", p.Name(), err)
	}

	// Register the plugin
	if err := s.registry.Register(p.Name(), p, metadata); err != nil {
		// Try to cleanup
		_ = p.Cleanup()
		return fmt.Errorf("registering plugin %s: %w", p.Name(), err)
	}

	s.logger.Info("Plugin loaded successfully",
		slog.String("name", p.Name()),
		slog.String("version", p.Version()),
		slog.String("path", path),
	)
	return nil
}

// UnloadPlugin unloads a plugin by name.
func (s *PluginService) UnloadPlugin(ctx context.Context, name string) error {
	// Get the plugin
	p, exists := s.registry.Get(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Call cleanup
	if err := p.Cleanup(); err != nil {
		s.logger.Error("Error cleaning up plugin",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
	}

	// Remove from registry
	if err := s.registry.Remove(name); err != nil {
		return fmt.Errorf("removing plugin %s from registry: %w", name, err)
	}

	// Unload from loader
	if err := s.loader.Unload(ctx, name); err != nil {
		s.logger.Error("Error unloading plugin",
			slog.String("name", name),
			slog.String("error", err.Error()),
		)
	}

	// Clear all cache entries since we can't selectively clear by plugin
	// This is a limitation of the current cache interface design
	if s.cache != nil {
		s.cache.Clear()
		s.logger.Info("Cleared cache after unloading plugin",
			slog.String("name", name),
		)
	}

	s.logger.Info("Plugin unloaded successfully",
		slog.String("name", name),
	)
	return nil
}

// DiscoverPlugins discovers plugins in configured directories.
func (s *PluginService) DiscoverPlugins(ctx context.Context) ([]pluginapi.PluginInfo, error) {
	if len(s.config.PluginDirs) == 0 {
		return nil, nil
	}

	// Discover plugins
	plugins, err := s.loader.Discover(ctx, s.config.PluginDirs)
	if err != nil {
		return nil, fmt.Errorf("discovering plugins: %w", err)
	}

	// Load compatible plugins if auto-discover is enabled
	if s.config.AutoDiscover {
		for _, info := range plugins {
			if info.Compatible {
				if err := s.LoadPlugin(ctx, info.Path); err != nil {
					s.logger.Warn("Failed to auto-load discovered plugin",
						slog.String("name", info.Name),
						slog.String("path", info.Path),
						slog.String("error", err.Error()),
					)
				} else {
					s.logger.Debug("Auto-loaded discovered plugin",
						slog.String("name", info.Name),
						slog.String("path", info.Path),
					)
				}
			}
		}
	}

	return plugins, nil
}

// ExecutePlugin executes a plugin by name.
func (s *PluginService) ExecutePlugin(ctx context.Context, name string, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	// Check if shutting down
	s.shutdownMu.Lock()
	if s.shutdown {
		s.shutdownMu.Unlock()
		return pluginapi.PluginOutput{}, errors.New("plugin service is shutting down")
	}
	s.shutdownMu.Unlock()

	// Get the plugin
	p, exists := s.registry.Get(name)
	if !exists {
		return pluginapi.PluginOutput{}, fmt.Errorf("plugin %s not found", name)
	}

	// Check cache if enabled
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := s.generateCacheKey(name, input)
		if output, found := s.cache.Get(cacheKey); found {
			return *output, nil
		}
	}

	// Get timeout from config, with plugin-specific override
	timeout := s.config.DefaultTimeout

	// Get timeout from plugin metadata if available
	if metadata, exists := s.registry.GetMetadata(name); exists {
		// Check if plugin has custom config with timeout
		if loadedPlugin, err := s.registry.GetLoadedPlugin(name); err == nil {
			// First try to get timeout from plugin's manifest config
			if manifestTimeout := loadedPlugin.GetPluginTimeout(); manifestTimeout > 0 {
				timeout = manifestTimeout
			}
		}

		// Check for timeout in plugin metadata config
		if metadata.Config != nil {
			if timeoutStr, exists := metadata.Config["timeout"]; exists {
				if customTimeout, err := parseTimeout(timeoutStr); err == nil && customTimeout > 0 {
					timeout = customTimeout
				}
			}
		}
	}

	// Execute the plugin with memory limiting if available
	startTime := time.Now()
	var output pluginapi.PluginOutput
	var err error

	if s.memoryExecutor != nil {
		// Use memory-limited execution
		output, err = s.memoryExecutor.ExecuteWithMemoryLimit(ctx, p, input, timeout)

		// Track memory usage if successful
		if err == nil {
			s.updateMemoryUsage(name)
		}
	} else {
		// Fallback to regular execution
		output, err = s.executor.ExecuteWithTimeout(ctx, p, input, timeout)
	}
	duration := time.Since(startTime)

	// Update statistics
	success := err == nil
	bytesIn := int64(len(input.Content))
	bytesOut := int64(len(output.HTML))
	s.registry.UpdateStatistics(name, duration, success, bytesIn, bytesOut)

	if err != nil {
		// Check if it was a timeout or panic
		if strings.Contains(err.Error(), "timeout") {
			s.registry.IncrementTimeout(name)
		} else if strings.Contains(err.Error(), "panic") {
			s.registry.IncrementPanic(name)
		}
		return pluginapi.PluginOutput{}, err
	}

	// Cache the result if enabled
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := s.generateCacheKey(name, input)
		s.cache.Set(cacheKey, &output, s.config.CacheTTL)
	}

	return output, nil
}

// GetPlugin retrieves a plugin by name.
func (s *PluginService) GetPlugin(name string) (pluginapi.Plugin, error) {
	p, exists := s.registry.Get(name)
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return p, nil
}

// GetPluginInfo returns information about a plugin.
func (s *PluginService) GetPluginInfo(name string) (*entities.LoadedPlugin, error) {
	return s.registry.GetLoadedPlugin(name)
}

// ListPlugins returns information about all loaded plugins.
func (s *PluginService) ListPlugins() []entities.LoadedPlugin {
	return s.registry.ListLoadedPlugins()
}

// ProcessContent processes content using matching plugins.
func (s *PluginService) ProcessContent(ctx context.Context, content string, language string) ([]pluginapi.PluginOutput, error) {
	// Find matching plugins
	var pluginNames []string
	if s.matcher != nil {
		metadata := map[string]interface{}{
			"language": language,
		}
		pluginNames = s.matcher.Match(content, language, metadata)
	} else {
		// Fallback: try all processor plugins
		processors := s.registry.GetByType(entities.PluginTypeProcessor)
		for _, p := range processors {
			pluginNames = append(pluginNames, p.Name())
		}
	}

	// If we have concurrent executor and optimization service, use optimized execution
	if s.concurrentExec != nil && len(pluginNames) > 1 {
		return s.processContentConcurrently(ctx, content, language, pluginNames)
	}

	// Fallback to sequential execution
	return s.processContentSequentially(ctx, content, language, pluginNames)
}

// processContentConcurrently executes plugins concurrently with optimization
func (s *PluginService) processContentConcurrently(ctx context.Context, content string, language string, pluginNames []string) ([]pluginapi.PluginOutput, error) {
	// Get plugin instances for concurrent execution
	var pluginInstances []entities.PluginInstance
	for _, name := range pluginNames {
		plugin, exists := s.registry.Get(name)
		if !exists {
			continue
		}

		// Get plugin metadata
		loadedPlugin, err := s.registry.GetLoadedPlugin(name)
		if err != nil {
			continue
		}

		pluginInstance := entities.PluginInstance{
			Instance: plugin,
			Metadata: loadedPlugin.Metadata,
		}
		pluginInstances = append(pluginInstances, pluginInstance)
	}

	// Optimize execution strategy based on content
	prioritizedJobs := s.concurrentExec.OptimizeForContent(pluginInstances, content)

	// Execute with priority optimization
	batchResult := s.concurrentExec.ExecuteWithPriority(ctx, prioritizedJobs)

	// Convert results to expected format
	var outputs []pluginapi.PluginOutput
	var errors []error

	for pluginName, result := range batchResult.Results {
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("plugin %s: %w", pluginName, result.Error))
			continue
		}
		outputs = append(outputs, result.Output)

		// Record performance metrics if optimization service is available
		if s.optimizationSvc != nil {
			monitor := s.optimizationSvc.GetPerformanceMonitor()
			monitor.RecordPluginExecution(result.Duration)
		}
	}

	// Return any errors
	if len(errors) > 0 {
		// Return outputs we got and the first error
		return outputs, errors[0]
	}

	return outputs, nil
}

// processContentSequentially executes plugins one by one (fallback)
func (s *PluginService) processContentSequentially(ctx context.Context, content string, language string, pluginNames []string) ([]pluginapi.PluginOutput, error) {
	var outputs []pluginapi.PluginOutput
	var errors []error

	for _, name := range pluginNames {
		input := pluginapi.PluginInput{
			Content:  content,
			Language: language,
			Options:  make(map[string]interface{}),
			Metadata: make(map[string]interface{}),
		}

		output, err := s.ExecutePlugin(ctx, name, input)
		if err != nil {
			errors = append(errors, fmt.Errorf("plugin %s: %w", name, err))
			continue
		}

		outputs = append(outputs, output)
	}

	// Return any errors
	if len(errors) > 0 {
		// Return outputs we got and the first error
		return outputs, errors[0]
	}

	return outputs, nil
}

// Shutdown gracefully shuts down the plugin service.
func (s *PluginService) Shutdown(ctx context.Context) error {
	s.shutdownMu.Lock()
	s.shutdown = true
	s.shutdownMu.Unlock()

	// Unload all plugins
	var errors []error
	for name := range s.registry.GetAll() {
		if err := s.UnloadPlugin(ctx, name); err != nil {
			errors = append(errors, fmt.Errorf("unloading %s: %w", name, err))
		}
	}

	// Cleanup memory-limited executor
	if s.memoryExecutor != nil {
		if err := s.memoryExecutor.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("cleaning up memory executor: %w", err))
		}
	}

	// Clear cache
	if s.cache != nil {
		s.cache.Clear()
	}

	// Clear memory monitoring
	s.memoryMonitoringMu.Lock()
	s.memoryMonitoring = make(map[string]int64)
	s.memoryMonitoringMu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// updateMemoryUsage updates memory monitoring for a plugin
func (s *PluginService) updateMemoryUsage(pluginName string) {
	if s.memoryExecutor == nil {
		return
	}

	usage := s.memoryExecutor.GetMemoryUsage()
	s.memoryMonitoringMu.Lock()
	defer s.memoryMonitoringMu.Unlock()

	for name, mem := range usage {
		if strings.Contains(name, pluginName) {
			s.memoryMonitoring[pluginName] = mem
			break
		}
	}
}

// GetMemoryUsage returns current memory usage for all plugins
func (s *PluginService) GetMemoryUsage() map[string]int64 {
	s.memoryMonitoringMu.RLock()
	defer s.memoryMonitoringMu.RUnlock()

	// Copy the map to avoid races
	usage := make(map[string]int64)
	for name, mem := range s.memoryMonitoring {
		usage[name] = mem
	}
	return usage
}

// IsMemoryLimitingEnabled returns whether memory limiting is enabled and available
func (s *PluginService) IsMemoryLimitingEnabled() bool {
	return s.memoryExecutor != nil && s.memoryExecutor.IsMemoryLimitingSupported()
}

// GetMemoryLimitConfig returns memory limiting configuration
func (s *PluginService) GetMemoryLimitConfig() (enabled bool, limit int64) {
	return s.config.EnableMemoryLimit, s.config.MemoryLimit
}

// GetActiveExecutions returns information about currently running plugin executions
func (s *PluginService) GetActiveExecutions() map[string]time.Duration {
	if s.memoryExecutor != nil {
		return s.memoryExecutor.GetActiveExecutions()
	}

	// Fallback to sandbox executor
	if s.executor != nil {
		if sandboxExec, ok := s.executor.(*concurrentplugin.SandboxExecutor); ok {
			return sandboxExec.GetExecutingPlugins()
		}
	}

	return make(map[string]time.Duration)
}

// SetMemoryEnforcementPolicy configures memory monitoring behavior
func (s *PluginService) SetMemoryEnforcementPolicy(enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64) error {
	if s.memoryExecutor == nil {
		return errors.New("memory limiting not available")
	}

	s.memoryExecutor.SetMemoryEnforcementPolicy(enableEnforcement, killOnExceed, warningThreshold, criticalThreshold)
	s.logger.Info("Memory enforcement policy updated",
		slog.Bool("enforcement_enabled", enableEnforcement),
		slog.Bool("kill_on_exceed", killOnExceed),
		slog.Float64("warning_threshold_percent", warningThreshold*100),
		slog.Float64("critical_threshold_percent", criticalThreshold*100),
	)

	return nil
}

// GetMemoryEnforcementPolicy returns current memory enforcement settings
func (s *PluginService) GetMemoryEnforcementPolicy() (enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64, err error) {
	if s.memoryExecutor == nil {
		return false, false, 0, 0, errors.New("memory limiting not available")
	}

	enableEnforcement, killOnExceed, warningThreshold, criticalThreshold = s.memoryExecutor.GetMemoryEnforcementPolicy()
	return enableEnforcement, killOnExceed, warningThreshold, criticalThreshold, nil
}

// GetMemoryStatistics returns comprehensive memory usage statistics
func (s *PluginService) GetMemoryStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	// Get current memory usage
	if usage := s.GetMemoryUsage(); len(usage) > 0 {
		stats["current_usage"] = usage

		// Calculate total usage
		var total int64
		for _, mem := range usage {
			total += mem
		}
		stats["total_usage_bytes"] = total
		stats["total_usage_mb"] = total / (1024 * 1024)
	}

	// Get active executions
	if active := s.GetActiveExecutions(); len(active) > 0 {
		stats["active_executions"] = active
		stats["active_count"] = len(active)
	}

	// Get memory configuration
	enabled, limit := s.GetMemoryLimitConfig()
	stats["memory_limiting_enabled"] = enabled
	stats["memory_limit_bytes"] = limit
	stats["memory_limit_mb"] = limit / (1024 * 1024)

	// Get enforcement policy
	if enableEnforcement, killOnExceed, warningThreshold, criticalThreshold, err := s.GetMemoryEnforcementPolicy(); err == nil {
		stats["enforcement_enabled"] = enableEnforcement
		stats["kill_on_exceed"] = killOnExceed
		stats["warning_threshold"] = warningThreshold
		stats["critical_threshold"] = criticalThreshold
	}

	// Add system information
	stats["platform_supported"] = s.IsMemoryLimitingEnabled()

	return stats
}

// generateCacheKey generates a cache key for a plugin execution.
func (s *PluginService) generateCacheKey(pluginName string, input pluginapi.PluginInput) string {
	// Simple key generation - could be improved with hashing
	key := fmt.Sprintf("%s:%s:%s", pluginName, input.Language, input.Content)
	if len(key) > 100 {
		// Truncate long keys
		key = key[:100]
	}
	return key
}

// parseTimeout parses a timeout string from plugin configuration.
// Supports formats like: "30s", "5m", "1h", "5000ms", or plain seconds as string "30"
func parseTimeout(timeoutStr string) (time.Duration, error) {
	if timeoutStr == "" {
		return 0, errors.New("empty timeout string")
	}

	// First try to parse as Go duration (e.g., "30s", "5m", "1h")
	if duration, err := time.ParseDuration(timeoutStr); err == nil {
		return duration, nil
	}

	// Try to parse as plain number (assume seconds)
	if seconds, err := strconv.ParseFloat(timeoutStr, 64); err == nil {
		if seconds > 0 {
			return time.Duration(seconds * float64(time.Second)), nil
		}
		return 0, errors.New("timeout must be positive")
	}

	// Try to parse formats like "30000ms", "5000" (milliseconds)
	msRegex := regexp.MustCompile(`^(\d+(?:\.\d+)?)ms?$`)
	if matches := msRegex.FindStringSubmatch(timeoutStr); len(matches) > 1 {
		if ms, err := strconv.ParseFloat(matches[1], 64); err == nil && ms > 0 {
			return time.Duration(ms * float64(time.Millisecond)), nil
		}
	}

	return 0, fmt.Errorf("invalid timeout format: %s", timeoutStr)
}

// Note: Import the InMemoryRegistry type assertion fix
type InMemoryRegistry interface {
	ports.PluginRegistry
	IncrementTimeout(name string)
	IncrementPanic(name string)
	GetLoadedPlugin(name string) (*entities.LoadedPlugin, error)
	ListLoadedPlugins() []entities.LoadedPlugin
}
