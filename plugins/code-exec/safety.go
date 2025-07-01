package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// limitedWriter wraps an io.Writer to enforce size limits
type limitedWriter struct {
	w       io.Writer
	limit   int
	written int
	mu      sync.Mutex
}

// Write implements io.Writer with size limiting
func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	if lw.written >= lw.limit {
		return 0, errors.New("output size limit exceeded")
	}

	remaining := lw.limit - lw.written
	if len(p) > remaining {
		p = p[:remaining]
		err = errors.New("output truncated: size limit exceeded")
	}

	n, writeErr := lw.w.Write(p)
	lw.written += n

	if writeErr != nil {
		return n, writeErr
	}

	return n, err
}

// Written returns the current number of bytes written
func (lw *limitedWriter) Written() int {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.written
}

// String returns the content as a string (requires the underlying writer to be a string builder)
func (lw *limitedWriter) String() string {
	if sb, ok := lw.w.(*strings.Builder); ok {
		return sb.String()
	}
	return ""
}

// IsTruncated returns true if output was truncated
func (lw *limitedWriter) IsTruncated() bool {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.written >= lw.limit
}

// newLimitedWriter creates a new limited writer with a string builder
func newLimitedWriter(limit int) *limitedWriter {
	return &limitedWriter{
		w:     &strings.Builder{},
		limit: limit,
	}
}

// setResourceLimits applies resource limits to the command
func setResourceLimits(cmd *exec.Cmd, config entities.ExecutionConfig) error {
	// Set process group for easier cleanup
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	// Platform-specific resource limits
	switch runtime.GOOS {
	case "linux", "darwin":
		return setUnixResourceLimits(cmd, config)
	case "windows":
		return setWindowsResourceLimits(cmd, config)
	default:
		// No specific limits for other platforms
		return nil
	}
}

// setUnixResourceLimits sets resource limits on Unix-like systems
func setUnixResourceLimits(cmd *exec.Cmd, config entities.ExecutionConfig) error {
	// Build ulimit command prefix
	var limitCmds []string

	// Only use virtual memory limit on Linux (not supported on macOS)
	if runtime.GOOS == "linux" {
		memoryLimitKB := config.MaxMemory / 1024
		limitCmds = append(limitCmds, fmt.Sprintf("ulimit -v %d", memoryLimitKB)) // Virtual memory
	}

	limitCmds = append(limitCmds, "ulimit -t 30") // CPU time (30 seconds max)

	// Only apply file size limits on Linux (Go builds need more space on macOS)
	if runtime.GOOS == "linux" {
		limitCmds = append(limitCmds, "ulimit -f 10240") // File size (10MB max for Go builds)
	}

	if !config.AllowFileWrite {
		limitCmds = append(limitCmds, "ulimit -f 0") // No file creation
	}

	// Combine limits with original command
	originalCmd := strings.Join(cmd.Args, " ")
	limitPrefix := strings.Join(limitCmds, " && ")
	fullCommand := fmt.Sprintf("%s && %s", limitPrefix, originalCmd)

	// Modify command to run through shell with limits
	cmd.Path = "/bin/sh"
	cmd.Args = []string{"/bin/sh", "-c", fullCommand}

	return nil
}

// setWindowsResourceLimits sets resource limits on Windows using job objects
func setWindowsResourceLimits(cmd *exec.Cmd, config entities.ExecutionConfig) error {
	// Windows uses job objects for resource limiting
	// The memory limiting is now handled by the WindowsJobObjectManager
	// in the plugin execution layer, so we just set up process group here

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Create new process group for better isolation
	setupProcessIsolation(cmd)

	// Note: Memory limiting is now handled at the plugin execution level
	// through Windows job objects in the MemoryLimitedExecutor
	return nil
}

// filterEnvironment filters environment variables for safety
func filterEnvironment(env []string) []string {
	// Allow only safe environment variables
	safeVars := []string{
		"PATH", "HOME", "USER", "LANG", "LC_ALL",
		"PYTHONDONTWRITEBYTECODE", "PYTHONUNBUFFERED", "PYTHONPATH",
		"NODE_ENV", "NODE_OPTIONS",
		"GOOS", "GOARCH", "CGO_ENABLED",
	}

	var filtered []string
	for _, variable := range env {
		for _, safe := range safeVars {
			if strings.HasPrefix(variable, safe+"=") {
				filtered = append(filtered, variable)
				break
			}
		}
	}

	return filtered
}

// setProcessGroup sets up process group for cleanup
func setProcessGroup(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
	return nil
}

// setupProcessIsolation sets up process isolation based on the operating system
func setupProcessIsolation(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		// Windows-specific process isolation is handled through job objects
		// at the MemoryLimitedExecutor level, so we don't need special handling here
		return
	} else {
		// Unix-like systems use process groups
		cmd.SysProcAttr.Setpgid = true
	}
}
