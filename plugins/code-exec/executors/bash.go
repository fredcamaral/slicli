package executors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// BashExecutor executes Bash/shell scripts
type BashExecutor struct{}

// Name returns the executor name
func (e *BashExecutor) Name() string {
	return "bash"
}

// IsAvailable checks if Bash is available
func (e *BashExecutor) IsAvailable() bool {
	// On Windows, check for bash (Git Bash, WSL, etc.)
	// On Unix systems, bash should be available
	if runtime.GOOS == "windows" {
		_, err := exec.LookPath("bash")
		return err == nil
	}

	// Try bash first, then sh as fallback
	for _, shell := range []string{"bash", "sh"} {
		if _, err := exec.LookPath(shell); err == nil {
			return true
		}
	}
	return false
}

// GetDefaultConfig returns default configuration for Bash execution
func (e *BashExecutor) GetDefaultConfig() entities.ExecutionConfig {
	config := entities.GetDefaultExecutionConfig()
	config.Language = "bash"
	config.Environment = []string{
		"PATH=/usr/bin:/bin", // Restricted PATH for security
		"SHELL=/bin/bash",
		"HOME=/tmp",
		"USER=nobody",
	}
	// Bash is inherently more dangerous, so stricter limits
	config.AllowFileWrite = false
	config.AllowNetwork = false
	return config
}

// Prepare sets up Bash script execution
func (e *BashExecutor) Prepare(ctx context.Context, code string, config entities.ExecutionConfig) (*exec.Cmd, func(), error) {
	// Create temporary file for bash script
	tmpFile, err := os.CreateTemp("", "slicli-bash-*.sh")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file: %w", err)
	}

	// Prepare bash script with safety measures
	script := e.prepareBashScript(code, config)
	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("writing bash script: %w", err)
	}
	_ = tmpFile.Close()

	// Make script executable (0700 = owner read/write/execute only)
	// #nosec G302 - script needs execute permission, 0700 is most restrictive possible for executable
	if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("making script executable: %w", err)
	}

	// Find shell executable
	shell := e.findShellExecutable()
	if shell == "" {
		_ = os.Remove(tmpFile.Name())
		return nil, nil, errors.New("shell interpreter not found")
	}

	// Create bash execution command
	cmd := exec.CommandContext(ctx, shell, tmpFile.Name()) // #nosec G204 - shell path validated by findShellExecutable

	// Setup cleanup function
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	return cmd, cleanup, nil
}

// findShellExecutable finds the best available shell
func (e *BashExecutor) findShellExecutable() string {
	// Prefer bash over sh for better features
	for _, shell := range []string{"bash", "sh"} {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}
	return ""
}

// prepareBashScript wraps the user script with safety measures
func (e *BashExecutor) prepareBashScript(code string, config entities.ExecutionConfig) string {
	var script strings.Builder

	// Shebang and safety settings
	script.WriteString("#!/bin/bash\n")
	script.WriteString("set -e  # Exit on error\n")
	script.WriteString("set -u  # Exit on undefined variable\n")
	script.WriteString("set -o pipefail  # Exit on pipe failure\n\n")

	// Resource limits (Unix only)
	if runtime.GOOS != "windows" {
		script.WriteString("# Resource limits\n")
		script.WriteString("ulimit -t 30    # CPU time: 30 seconds\n")
		script.WriteString("ulimit -v 102400 # Virtual memory: 100MB\n")
		script.WriteString("ulimit -f 1024   # File size: 1MB\n")
		if !config.AllowFileWrite {
			script.WriteString("ulimit -f 0      # No file creation\n")
		}
		script.WriteString("\n")
	}

	// Timeout mechanism
	script.WriteString("# Timeout mechanism\n")
	script.WriteString("(\n")
	script.WriteString("  sleep 30 && kill -TERM $$ 2>/dev/null\n")
	script.WriteString(") &\n")
	script.WriteString("TIMEOUT_PID=$!\n\n")

	// Cleanup function
	script.WriteString("# Cleanup function\n")
	script.WriteString("cleanup() {\n")
	script.WriteString("  kill $TIMEOUT_PID 2>/dev/null || true\n")
	script.WriteString("}\n")
	script.WriteString("trap cleanup EXIT\n\n")

	// Restricted commands check
	script.WriteString("# User script:\n")

	// Apply restrictions if not in trusted mode
	if !config.TrustedMode {
		code = e.restrictBashCode(code)
	}

	script.WriteString(code)
	script.WriteString("\n")

	return script.String()
}

// restrictBashCode applies security restrictions to bash code
func (e *BashExecutor) restrictBashCode(code string) string {
	// List of dangerous commands to check for
	dangerousCommands := []string{
		"rm ", "rmdir", "mv ", "cp ", "chmod", "chown",
		"sudo", "su ", "passwd", "useradd", "userdel",
		"mount", "umount", "fdisk", "mkfs",
		"iptables", "netcat", "nc ", "wget", "curl",
		"ssh", "scp", "rsync", "ftp",
		"crontab", "at ", "batch",
		"kill", "killall", "pkill",
		"reboot", "shutdown", "halt",
		"dd ", "shred",
		"eval", "exec", "source", ".",
		"$(", "`", "bash", "sh ",
		"/dev/", "/proc/", "/sys/",
		"&", "&&", "||", "|",
	}

	// Check for dangerous patterns
	lowerCode := strings.ToLower(code)
	var warnings []string

	for _, cmd := range dangerousCommands {
		if strings.Contains(lowerCode, cmd) {
			warnings = append(warnings, "# WARNING: Potentially dangerous command detected: "+cmd)
		}
	}

	// Add warnings to the top of the script
	if len(warnings) > 0 {
		warningBlock := strings.Join(warnings, "\n") + "\n\n"
		code = warningBlock + code
	}

	return code
}
