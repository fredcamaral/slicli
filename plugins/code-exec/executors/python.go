package executors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// dangerousPythonPatterns defines regex patterns for dangerous Python code
// These patterns are designed to catch various forms of dangerous operations
var dangerousPythonPatterns = []struct {
	pattern *regexp.Regexp
	name    string
}{
	// System command execution
	{regexp.MustCompile(`(?i)\bos\.system\s*\(`), "os.system()"},
	{regexp.MustCompile(`(?i)\bos\.popen\s*\(`), "os.popen()"},
	{regexp.MustCompile(`(?i)\bos\.spawn`), "os.spawn*()"},
	{regexp.MustCompile(`(?i)\bsubprocess\b`), "subprocess module"},
	{regexp.MustCompile(`(?i)\bcommands\b`), "commands module"},

	// Dynamic code execution
	{regexp.MustCompile(`(?i)\b__import__\s*\(`), "__import__()"},
	{regexp.MustCompile(`(?i)\bexec\s*\(`), "exec()"},
	{regexp.MustCompile(`(?i)\beval\s*\(`), "eval()"},
	{regexp.MustCompile(`(?i)\bcompile\s*\(`), "compile()"},

	// File system danger
	{regexp.MustCompile(`(?i)\bshutil\.rmtree`), "shutil.rmtree()"},
	{regexp.MustCompile(`(?i)\bos\.remove`), "os.remove()"},
	{regexp.MustCompile(`(?i)\bos\.unlink`), "os.unlink()"},
	{regexp.MustCompile(`(?i)\bos\.rmdir`), "os.rmdir()"},
	{regexp.MustCompile(`(?i)\bos\.rename`), "os.rename()"},
	{regexp.MustCompile(`(?i)\bos\.chmod`), "os.chmod()"},
	{regexp.MustCompile(`(?i)\bos\.chown`), "os.chown()"},

	// Network
	{regexp.MustCompile(`(?i)\bsocket\b`), "socket module"},
	{regexp.MustCompile(`(?i)\burllib`), "urllib module"},
	{regexp.MustCompile(`(?i)\brequests\b`), "requests module"},
	{regexp.MustCompile(`(?i)\bhttplib`), "httplib module"},
	{regexp.MustCompile(`(?i)\bhttp\.client`), "http.client module"},
	{regexp.MustCompile(`(?i)\bftplib`), "ftplib module"},
	{regexp.MustCompile(`(?i)\bsmtplib`), "smtplib module"},
	{regexp.MustCompile(`(?i)\btelnetlib`), "telnetlib module"},

	// Dangerous paths
	{regexp.MustCompile(`['"]/etc/`), "/etc/ access"},
	{regexp.MustCompile(`['"]~/`), "home directory access"},
	{regexp.MustCompile(`['"]/dev/`), "/dev/ access"},
	{regexp.MustCompile(`['"]/proc/`), "/proc/ access"},
	{regexp.MustCompile(`['"]/sys/`), "/sys/ access"},

	// Dangerous modules
	{regexp.MustCompile(`(?i)\bctypes\b`), "ctypes module"},
	{regexp.MustCompile(`(?i)\bpickle\b`), "pickle module"},
	{regexp.MustCompile(`(?i)\bmultiprocessing\b`), "multiprocessing module"},
	{regexp.MustCompile(`(?i)\bpty\b`), "pty module"},
}

// PythonExecutor executes Python code
type PythonExecutor struct{}

// Name returns the executor name
func (e *PythonExecutor) Name() string {
	return "python"
}

// IsAvailable checks if Python runtime is available
func (e *PythonExecutor) IsAvailable() bool {
	// Try python3 first, then python
	for _, cmd := range []string{"python3", "python"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}
	return false
}

// GetDefaultConfig returns default configuration for Python execution
func (e *PythonExecutor) GetDefaultConfig() entities.ExecutionConfig {
	config := entities.GetDefaultExecutionConfig()
	config.Language = "python"
	config.Environment = []string{
		"PYTHONDONTWRITEBYTECODE=1", // Don't create .pyc files
		"PYTHONUNBUFFERED=1",        // Don't buffer output
		"PYTHONPATH=",               // Clear Python path for security
	}
	return config
}

// Prepare sets up Python code execution
func (e *PythonExecutor) Prepare(ctx context.Context, code string, config entities.ExecutionConfig) (*exec.Cmd, func(), error) {
	// Validate code for dangerous patterns BEFORE any execution setup
	// This check is performed unless TrustedMode is enabled
	if !config.TrustedMode {
		if err := e.validatePythonCode(code); err != nil {
			return nil, nil, err
		}
	}

	// Create temporary file for Python code
	tmpFile, err := os.CreateTemp("", "slicli-python-*.py")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file: %w", err)
	}

	// Write Python code to file
	pythonCode := e.preparePythonCode(code)
	if _, err := tmpFile.WriteString(pythonCode); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("writing Python code: %w", err)
	}
	_ = tmpFile.Close()

	// Find Python executable
	pythonCmd := e.findPythonExecutable()
	if pythonCmd == "" {
		_ = os.Remove(tmpFile.Name())
		return nil, nil, errors.New("python interpreter not found")
	}

	// Create Python execution command
	cmd := exec.CommandContext(ctx, pythonCmd, tmpFile.Name()) // #nosec G204 - pythonCmd path validated by findPythonExecutable

	// Setup cleanup function
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	return cmd, cleanup, nil
}

// findPythonExecutable finds the best available Python executable
func (e *PythonExecutor) findPythonExecutable() string {
	// Prefer python3 over python for better compatibility
	for _, cmd := range []string{"python3", "python"} {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}
	return ""
}

// preparePythonCode prepares Python code for execution
func (e *PythonExecutor) preparePythonCode(code string) string {
	// Add safety imports and restrictions
	safetyCode := `
import sys
import signal
import resource

# Set resource limits for safety
try:
    # Limit memory usage (100MB)
    resource.setrlimit(resource.RLIMIT_AS, (100 * 1024 * 1024, 100 * 1024 * 1024))
    # Limit CPU time (30 seconds)
    resource.setrlimit(resource.RLIMIT_CPU, (30, 30))
except (ImportError, OSError):
    # Resource limiting not available on this platform
    pass

# Timeout handler
def timeout_handler(signum, frame):
    print("Execution timed out", file=sys.stderr)
    sys.exit(124)

# Set up timeout signal (Unix only)
try:
    signal.signal(signal.SIGALRM, timeout_handler)
except (AttributeError, OSError):
    # Signal handling not available on this platform
    pass

# Common imports for convenience
import math
import random
import datetime
import json
import re
from collections import defaultdict, Counter, deque
from itertools import combinations, permutations, product

`

	// Add user code
	return safetyCode + "\n# User code:\n" + code
}

// validatePythonCode checks Python code for dangerous patterns and returns an error if any are found.
// This function BLOCKS execution of dangerous code rather than just warning about it.
func (e *PythonExecutor) validatePythonCode(code string) error {
	var blockedPatterns []string

	// Check against all dangerous patterns
	for _, dp := range dangerousPythonPatterns {
		if dp.pattern.MatchString(code) {
			blockedPatterns = append(blockedPatterns, dp.name)
		}
	}

	// If any dangerous patterns were found, return an error
	if len(blockedPatterns) > 0 {
		return entities.NewBlockedCommandError(blockedPatterns)
	}

	return nil
}

// IsDangerousPythonCode checks if Python code contains dangerous patterns.
// This is a helper function for external validation needs.
func IsDangerousPythonCode(code string) (bool, []string) {
	var blockedPatterns []string

	for _, dp := range dangerousPythonPatterns {
		if dp.pattern.MatchString(code) {
			blockedPatterns = append(blockedPatterns, dp.name)
		}
	}

	return len(blockedPatterns) > 0, blockedPatterns
}
