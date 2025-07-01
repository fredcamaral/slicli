package executors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

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
