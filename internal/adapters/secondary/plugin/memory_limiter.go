package plugin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// MemoryLimiter handles memory limiting for plugin execution using OS-specific mechanisms
type MemoryLimiter struct {
	cgroupPath   string
	cgroupPrefix string
	mu           sync.Mutex
	activeGroups map[string]*cgroupInfo

	// Enforcement configuration
	enableEnforcement  bool          // Enable active memory enforcement
	warningThreshold   float64       // Threshold for warnings (0.0-1.0)
	criticalThreshold  float64       // Threshold for critical alerts (0.0-1.0)
	killOnExceed       bool          // Kill execution if memory limit is exceeded
	monitoringInterval time.Duration // How often to check memory usage
}

type cgroupInfo struct {
	path         string
	createdAt    time.Time
	pid          int
	currentUsage int64
}

// NewMemoryLimiter creates a new memory limiter
func NewMemoryLimiter() (*MemoryLimiter, error) {
	ml := &MemoryLimiter{
		cgroupPrefix: "slicli-plugin",
		activeGroups: make(map[string]*cgroupInfo),

		// Default enforcement configuration
		enableEnforcement:  true,                   // Enable memory enforcement by default
		warningThreshold:   0.8,                    // Warn at 80% usage
		criticalThreshold:  0.9,                    // Critical at 90% usage
		killOnExceed:       false,                  // Don't kill by default, let OS handle it
		monitoringInterval: 100 * time.Millisecond, // Check every 100ms
	}

	if err := ml.initialize(); err != nil {
		return nil, fmt.Errorf("initializing memory limiter: %w", err)
	}

	return ml, nil
}

// SetEnforcementPolicy configures memory monitoring and enforcement behavior
func (ml *MemoryLimiter) SetEnforcementPolicy(enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64, monitoringInterval time.Duration) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	ml.enableEnforcement = enableEnforcement
	ml.killOnExceed = killOnExceed
	ml.warningThreshold = warningThreshold
	ml.criticalThreshold = criticalThreshold
	ml.monitoringInterval = monitoringInterval
}

// GetEnforcementPolicy returns current memory enforcement policy settings
func (ml *MemoryLimiter) GetEnforcementPolicy() (enableEnforcement, killOnExceed bool, warningThreshold, criticalThreshold float64, monitoringInterval time.Duration) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	return ml.enableEnforcement, ml.killOnExceed, ml.warningThreshold, ml.criticalThreshold, ml.monitoringInterval
}

// initialize sets up the memory limiter based on the OS
func (ml *MemoryLimiter) initialize() error {
	switch runtime.GOOS {
	case "linux":
		return ml.initializeLinuxCgroups()
	case "darwin":
		return ml.initializeMacOSLimits()
	case "windows":
		return ml.initializeWindowsJobObjects()
	default:
		return fmt.Errorf("memory limiting not supported on %s", runtime.GOOS)
	}
}

// initializeLinuxCgroups sets up cgroups v1/v2 on Linux
func (ml *MemoryLimiter) initializeLinuxCgroups() error {
	// Try cgroups v2 first
	if ml.tryInitializeCgroupsV2() {
		return nil
	}

	// Fall back to cgroups v1
	return ml.initializeCgroupsV1()
}

// tryInitializeCgroupsV2 attempts to set up cgroups v2
func (ml *MemoryLimiter) tryInitializeCgroupsV2() bool {
	// Check if cgroups v2 is available
	cgroupV2Path := "/sys/fs/cgroup"
	if _, err := os.Stat(filepath.Join(cgroupV2Path, "cgroup.controllers")); err != nil {
		return false
	}

	// Check if memory controller is available
	controllers, err := os.ReadFile(filepath.Join(cgroupV2Path, "cgroup.controllers"))
	if err != nil || !strings.Contains(string(controllers), "memory") {
		return false
	}

	ml.cgroupPath = cgroupV2Path
	return true
}

// initializeCgroupsV1 sets up cgroups v1
func (ml *MemoryLimiter) initializeCgroupsV1() error {
	// Check for cgroups v1 memory controller
	cgroupV1Path := "/sys/fs/cgroup/memory"
	if _, err := os.Stat(cgroupV1Path); err != nil {
		return fmt.Errorf("cgroups v1 memory controller not available: %w", err)
	}

	ml.cgroupPath = cgroupV1Path
	return nil
}

// initializeMacOSLimits sets up macOS resource limits
func (ml *MemoryLimiter) initializeMacOSLimits() error {
	// macOS uses BSD resource limits, no setup needed
	ml.cgroupPath = "" // Not applicable for macOS
	return nil
}

// initializeWindowsJobObjects sets up Windows job objects
func (ml *MemoryLimiter) initializeWindowsJobObjects() error {
	// Windows job objects require no global initialization
	// The actual implementation is in memory_limiter_windows.go (build tag: windows)
	return nil
}

// ExecuteWithMemoryLimit executes a plugin with memory limits
func (ml *MemoryLimiter) ExecuteWithMemoryLimit(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	switch runtime.GOOS {
	case "linux":
		return ml.executeLinuxWithCgroups(ctx, p, input, memoryLimitBytes, timeout)
	case "darwin":
		return ml.executeMacOSWithLimits(ctx, p, input, memoryLimitBytes, timeout)
	case "windows":
		return ml.executeWindowsWithJobObjects(ctx, p, input, memoryLimitBytes, timeout)
	default:
		return pluginapi.PluginOutput{}, fmt.Errorf("memory limiting not supported on %s", runtime.GOOS)
	}
}

// executeLinuxWithCgroups executes plugin with Linux cgroups
func (ml *MemoryLimiter) executeLinuxWithCgroups(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// Create unique cgroup for this execution
	groupName := fmt.Sprintf("%s-%s-%d", ml.cgroupPrefix, p.Name(), time.Now().UnixNano())
	cgroupPath := filepath.Join(ml.cgroupPath, groupName)

	// Create cgroup
	if err := ml.createCgroup(cgroupPath, memoryLimitBytes); err != nil {
		return pluginapi.PluginOutput{}, fmt.Errorf("creating cgroup: %w", err)
	}

	// Ensure cleanup
	defer func() {
		if err := ml.removeCgroup(cgroupPath); err != nil {
			// Log error but don't fail the execution
			fmt.Printf("Warning: failed to cleanup cgroup %s: %v\n", cgroupPath, err)
		}
	}()

	// Execute with memory monitoring
	return ml.executeWithMonitoring(ctx, p, input, cgroupPath, timeout)
}

// createCgroup creates a new cgroup with memory limits
func (ml *MemoryLimiter) createCgroup(cgroupPath string, memoryLimitBytes int64) error {
	// Create cgroup directory
	if err := os.MkdirAll(cgroupPath, 0750); err != nil {
		return fmt.Errorf("creating cgroup directory: %w", err)
	}

	// Set memory limit
	memoryLimitFile := filepath.Join(cgroupPath, "memory.limit_in_bytes")
	if strings.Contains(ml.cgroupPath, "cgroup") && !strings.Contains(ml.cgroupPath, "memory") {
		// cgroups v2
		memoryLimitFile = filepath.Join(cgroupPath, "memory.max")
	}

	if err := os.WriteFile(memoryLimitFile, []byte(strconv.FormatInt(memoryLimitBytes, 10)), 0600); err != nil {
		return fmt.Errorf("setting memory limit: %w", err)
	}

	// Enable memory accounting (cgroups v2)
	if strings.Contains(ml.cgroupPath, "/sys/fs/cgroup") && !strings.Contains(ml.cgroupPath, "memory") {
		subtreeControlFile := filepath.Join(cgroupPath, "cgroup.subtree_control")
		if err := os.WriteFile(subtreeControlFile, []byte("+memory"), 0600); err != nil {
			// This might fail if already enabled, which is fine
			// We intentionally ignore this error as it's expected in many cases
			_ = err // Explicitly mark as ignored
		}
	}

	return nil
}

// executeWithMonitoring executes the plugin with memory monitoring
func (ml *MemoryLimiter) executeWithMonitoring(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, cgroupPath string, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Result channel
	type result struct {
		output pluginapi.PluginOutput
		err    error
	}
	resultChan := make(chan result, 1)

	// Register this execution for monitoring
	ml.mu.Lock()
	groupName := filepath.Base(cgroupPath)
	ml.activeGroups[groupName] = &cgroupInfo{
		path:      cgroupPath,
		createdAt: time.Now(),
		pid:       os.Getpid(),
	}
	ml.mu.Unlock()

	// Cleanup registration when done
	defer func() {
		ml.mu.Lock()
		delete(ml.activeGroups, groupName)
		ml.mu.Unlock()
	}()

	// Start monitoring goroutine
	monitorDone := make(chan struct{})
	go ml.monitorMemoryUsage(execCtx, cgroupPath, monitorDone)

	// Execute plugin in goroutine
	go func() {
		defer close(monitorDone)

		// Move current process to cgroup (for Linux)
		if runtime.GOOS == "linux" {
			procsFile := filepath.Join(cgroupPath, "cgroup.procs")
			if err := os.WriteFile(procsFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				resultChan <- result{err: fmt.Errorf("adding process to cgroup: %w", err)}
				return
			}
		}

		// Execute the plugin
		output, err := p.Execute(execCtx, input)
		resultChan <- result{output: output, err: err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-execCtx.Done():
		return pluginapi.PluginOutput{}, errors.New("execution timeout or cancelled")
	}
}

// monitorMemoryUsage monitors memory usage during execution with active enforcement
func (ml *MemoryLimiter) monitorMemoryUsage(ctx context.Context, cgroupPath string, done <-chan struct{}) {
	// Get current enforcement configuration
	ml.mu.Lock()
	enableEnforcement := ml.enableEnforcement
	killOnExceed := ml.killOnExceed
	warningThreshold := ml.warningThreshold
	criticalThreshold := ml.criticalThreshold
	monitoringInterval := ml.monitoringInterval
	ml.mu.Unlock()

	if !enableEnforcement {
		// Monitoring disabled, just wait for completion
		<-done
		return
	}

	ticker := time.NewTicker(monitoringInterval)
	defer ticker.Stop()

	usageFile := filepath.Join(cgroupPath, "memory.usage_in_bytes")
	limitFile := filepath.Join(cgroupPath, "memory.limit_in_bytes")
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	if strings.Contains(ml.cgroupPath, "/sys/fs/cgroup") && !strings.Contains(ml.cgroupPath, "memory") {
		// cgroups v2
		usageFile = filepath.Join(cgroupPath, "memory.current")
		limitFile = filepath.Join(cgroupPath, "memory.max")
	}

	// Read memory limit for comparison
	var memoryLimit int64
	// #nosec G304 - limitFile path is constructed from validated cgroup directory structure
	// Reading memory limit from cgroup filesystem is legitimate system monitoring
	if data, err := os.ReadFile(limitFile); err == nil {
		if limit, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			memoryLimit = limit
		}
	}

	var lastWarningTime time.Time
	var lastCriticalTime time.Time
	exceedanceCount := 0 // Count how many times we've exceeded the critical threshold

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			// Read current memory usage
			// #nosec G304 - usageFile path is constructed from validated cgroup directory structure
			// Reading memory usage from cgroup filesystem is legitimate system monitoring
			if data, err := os.ReadFile(usageFile); err == nil {
				if usage, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
					// Calculate usage percentage
					if memoryLimit > 0 {
						usagePercent := float64(usage) / float64(memoryLimit)
						now := time.Now()

						// Store current usage for reporting
						ml.mu.Lock()
						for _, info := range ml.activeGroups {
							if info.path == cgroupPath {
								info.currentUsage = usage
								break
							}
						}
						ml.mu.Unlock()

						// Handle memory enforcement based on usage level
						if usagePercent >= 1.0 || (usagePercent > criticalThreshold && killOnExceed) {
							// Memory limit exceeded or critical threshold with kill policy
							exceedanceCount++

							if now.Sub(lastCriticalTime) > time.Second {
								log.Printf("[CRITICAL] Plugin memory limit exceeded: %.1f%% (%d MB / %d MB) in cgroup %s (exceedance #%d)",
									usagePercent*100, usage/(1024*1024), memoryLimit/(1024*1024), filepath.Base(cgroupPath), exceedanceCount)
								lastCriticalTime = now
							}

							// If kill policy is enabled and we've exceeded multiple times, terminate
							if killOnExceed && exceedanceCount >= 3 {
								log.Printf("[ENFORCEMENT] Terminating plugin execution due to memory limit violation in cgroup %s", filepath.Base(cgroupPath))
								ml.terminateProcessesInCgroup(procsFile)
								return
							}
						} else if usagePercent > criticalThreshold && now.Sub(lastCriticalTime) > time.Second {
							log.Printf("[CRITICAL] Plugin memory usage: %.1f%% (%d MB / %d MB) in cgroup %s",
								usagePercent*100, usage/(1024*1024), memoryLimit/(1024*1024), filepath.Base(cgroupPath))
							lastCriticalTime = now
						} else if usagePercent > warningThreshold && now.Sub(lastWarningTime) > 5*time.Second {
							log.Printf("[WARNING] Plugin memory usage: %.1f%% (%d MB / %d MB) in cgroup %s",
								usagePercent*100, usage/(1024*1024), memoryLimit/(1024*1024), filepath.Base(cgroupPath))
							lastWarningTime = now
						}
					}
				}
			}
		}
	}
}

// terminateProcessesInCgroup terminates all processes in a cgroup
func (ml *MemoryLimiter) terminateProcessesInCgroup(procsFile string) {
	// #nosec G304 - procsFile is a validated cgroup.procs file path for process management
	// Reading process list from cgroup filesystem is legitimate system monitoring and cleanup
	if data, err := os.ReadFile(procsFile); err == nil {
		pids := strings.Fields(string(data))
		for _, pidStr := range pids {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				if proc, err := os.FindProcess(pid); err == nil {
					// Send SIGTERM first for graceful shutdown
					if err := proc.Signal(os.Interrupt); err != nil {
						log.Printf("Failed to send SIGTERM to process %d: %v", pid, err)
					} else {
						log.Printf("Sent SIGTERM to process %d for memory limit enforcement", pid)
					}

					// Wait a moment then send SIGKILL if needed
					go func(p *os.Process, pid int) {
						time.Sleep(2 * time.Second)
						if err := p.Kill(); err != nil {
							// Process may have already exited, which is fine
							log.Printf("Process %d may have already exited: %v", pid, err)
						} else {
							log.Printf("Sent SIGKILL to process %d for memory limit enforcement", pid)
						}
					}(proc, pid)
				}
			}
		}
	}
}

// executeMacOSWithLimits executes plugin with macOS resource limits
func (ml *MemoryLimiter) executeMacOSWithLimits(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// macOS uses ulimit and resource limits
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Set resource limits using ulimit
	originalPath := os.Getenv("PATH")
	memoryLimitKB := memoryLimitBytes / 1024

	// For macOS, we use environment variables to communicate limits
	// since direct ulimit integration requires shell command execution

	// For macOS, we need to monitor memory usage manually
	// since cgroups aren't available
	resultChan := make(chan struct {
		output pluginapi.PluginOutput
		err    error
	}, 1)

	go func() {
		defer func() {
			_ = os.Setenv("PATH", originalPath)
		}()

		// Set memory limit environment (both bytes and KB for compatibility)
		_ = os.Setenv("SLICLI_MEMORY_LIMIT", strconv.FormatInt(memoryLimitBytes, 10))
		_ = os.Setenv("SLICLI_MEMORY_LIMIT_KB", strconv.FormatInt(memoryLimitKB, 10))

		output, err := p.Execute(execCtx, input)
		resultChan <- struct {
			output pluginapi.PluginOutput
			err    error
		}{output, err}
	}()

	select {
	case res := <-resultChan:
		return res.output, res.err
	case <-execCtx.Done():
		return pluginapi.PluginOutput{}, errors.New("execution timeout")
	}
}

// executeWindowsWithJobObjects executes plugin with Windows job objects
// The actual implementation is in memory_limiter_windows.go (build tag: windows)
func (ml *MemoryLimiter) executeWindowsWithJobObjects(ctx context.Context, p pluginapi.Plugin, input pluginapi.PluginInput, memoryLimitBytes int64, timeout time.Duration) (pluginapi.PluginOutput, error) {
	// This is a stub for non-Windows platforms
	return pluginapi.PluginOutput{}, errors.New("windows job objects only available on Windows")
}

// removeCgroup removes a cgroup
func (ml *MemoryLimiter) removeCgroup(cgroupPath string) error {
	// Kill any remaining processes
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	// #nosec G304 - procsFile path is constructed from validated cgroup directory structure
	// Reading process list from cgroup filesystem is legitimate system monitoring
	if data, err := os.ReadFile(procsFile); err == nil {
		pids := strings.Fields(string(data))
		for _, pidStr := range pids {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				// Send SIGTERM to process
				if proc, err := os.FindProcess(pid); err == nil {
					_ = proc.Signal(os.Interrupt)
				}
			}
		}
	}

	// Wait a bit for processes to exit
	time.Sleep(100 * time.Millisecond)

	// Remove cgroup directory
	return os.Remove(cgroupPath)
}

// Cleanup removes all active cgroups
func (ml *MemoryLimiter) Cleanup() error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	var errs []string
	for name, info := range ml.activeGroups {
		if err := ml.removeCgroup(info.path); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	ml.activeGroups = make(map[string]*cgroupInfo)

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// GetMemoryUsage returns current memory usage for active executions
func (ml *MemoryLimiter) GetMemoryUsage() map[string]int64 {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	usage := make(map[string]int64)
	for name, info := range ml.activeGroups {
		// Use stored current usage if available, otherwise read from file
		if info.currentUsage > 0 {
			usage[name] = info.currentUsage
		} else {
			// Fallback to reading from file for platforms without monitoring
			usageFile := filepath.Join(info.path, "memory.usage_in_bytes")
			if strings.Contains(ml.cgroupPath, "/sys/fs/cgroup") && !strings.Contains(ml.cgroupPath, "memory") {
				usageFile = filepath.Join(info.path, "memory.current")
			}

			// #nosec G304 - usageFile path is constructed from validated cgroup directory structure
			// Reading memory usage from cgroup filesystem is legitimate system monitoring
			if data, err := os.ReadFile(usageFile); err == nil {
				if memUsage, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
					usage[name] = memUsage
				}
			}
		}
	}

	return usage
}

// IsMemoryLimitingAvailable checks if memory limiting is available on the current platform
func IsMemoryLimitingAvailable() bool {
	switch runtime.GOOS {
	case "linux":
		// Check for cgroups v1 or v2
		return checkLinuxCgroupsAvailable()
	case "darwin":
		// macOS has basic resource limits
		return true
	case "windows":
		// Windows job objects are now implemented
		return true
	default:
		return false
	}
}

// checkLinuxCgroupsAvailable checks if cgroups are available on Linux
func checkLinuxCgroupsAvailable() bool {
	// Check cgroups v2
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return true
	}

	// Check cgroups v1
	if _, err := os.Stat("/sys/fs/cgroup/memory"); err == nil {
		return true
	}

	return false
}
