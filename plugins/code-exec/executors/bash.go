package executors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// dangerousCommandPatterns defines regex patterns for dangerous shell commands
// These patterns are designed to catch various forms of dangerous commands
var dangerousCommandPatterns = []struct {
	pattern *regexp.Regexp
	name    string
}{
	// File system destruction commands
	{regexp.MustCompile(`(?i)\brm\s+(-[rfvI]*\s+)*`), "rm"},
	{regexp.MustCompile(`(?i)\brmdir\b`), "rmdir"},
	{regexp.MustCompile(`(?i)\bmv\s+`), "mv"},
	{regexp.MustCompile(`(?i)\bchmod\b`), "chmod"},
	{regexp.MustCompile(`(?i)\bchown\b`), "chown"},
	{regexp.MustCompile(`(?i)\bdd\s+`), "dd"},
	{regexp.MustCompile(`(?i)\bshred\b`), "shred"},
	{regexp.MustCompile(`(?i)\bmkfs\b`), "mkfs"},
	{regexp.MustCompile(`(?i)\bfdisk\b`), "fdisk"},

	// Privilege escalation
	{regexp.MustCompile(`(?i)\bsudo\b`), "sudo"},
	{regexp.MustCompile(`(?i)\bsu\s+`), "su"},
	{regexp.MustCompile(`(?i)\bpasswd\b`), "passwd"},
	{regexp.MustCompile(`(?i)\buseradd\b`), "useradd"},
	{regexp.MustCompile(`(?i)\buserdel\b`), "userdel"},

	// Network commands (potential data exfiltration)
	{regexp.MustCompile(`(?i)\bwget\b`), "wget"},
	{regexp.MustCompile(`(?i)\bcurl\b`), "curl"},
	{regexp.MustCompile(`(?i)\bssh\b`), "ssh"},
	{regexp.MustCompile(`(?i)\bscp\b`), "scp"},
	{regexp.MustCompile(`(?i)\brsync\b`), "rsync"},
	{regexp.MustCompile(`(?i)\bftp\b`), "ftp"},
	{regexp.MustCompile(`(?i)\bnc\s+`), "nc (netcat)"},
	{regexp.MustCompile(`(?i)\bnetcat\b`), "netcat"},
	{regexp.MustCompile(`(?i)\btelnet\b`), "telnet"},

	// System commands
	{regexp.MustCompile(`(?i)\bmount\b`), "mount"},
	{regexp.MustCompile(`(?i)\bumount\b`), "umount"},
	{regexp.MustCompile(`(?i)\breboot\b`), "reboot"},
	{regexp.MustCompile(`(?i)\bshutdown\b`), "shutdown"},
	{regexp.MustCompile(`(?i)\bhalt\b`), "halt"},
	{regexp.MustCompile(`(?i)\biptables\b`), "iptables"},

	// Process manipulation
	{regexp.MustCompile(`(?i)\bkill\s+`), "kill"},
	{regexp.MustCompile(`(?i)\bkillall\b`), "killall"},
	{regexp.MustCompile(`(?i)\bpkill\b`), "pkill"},

	// Scheduled tasks
	{regexp.MustCompile(`(?i)\bcrontab\b`), "crontab"},
	{regexp.MustCompile(`(?i)\bat\s+`), "at"},
	{regexp.MustCompile(`(?i)\bbatch\b`), "batch"},

	// Shell spawning and code execution
	{regexp.MustCompile(`(?i)\beval\s+`), "eval"},
	{regexp.MustCompile(`(?i)\bexec\s+`), "exec"},
	{regexp.MustCompile(`(?i)\bsource\s+`), "source"},
	{regexp.MustCompile(`(?i)\bbash\s+`), "bash"},
	{regexp.MustCompile(`(?i)\bsh\s+-c`), "sh -c"},
	{regexp.MustCompile(`(?i)\bzsh\s+`), "zsh"},

	// Command substitution (allows arbitrary command execution)
	// Note: $((expr)) is arithmetic expansion which is safe, so we exclude it
	// The pattern matches $( followed by anything except another (
	{regexp.MustCompile(`\$\([^(][^)]*\)`), "command substitution $()"},
	{regexp.MustCompile("`[^`]+`"), "command substitution backticks"},

	// Dangerous path access
	{regexp.MustCompile(`/dev/`), "/dev/ access"},
	{regexp.MustCompile(`/proc/`), "/proc/ access"},
	{regexp.MustCompile(`/sys/`), "/sys/ access"},
	{regexp.MustCompile(`/etc/passwd`), "/etc/passwd access"},
	{regexp.MustCompile(`/etc/shadow`), "/etc/shadow access"},

	// Environment variable manipulation that could be dangerous
	{regexp.MustCompile(`(?i)\bexport\s+PATH=`), "PATH manipulation"},
	{regexp.MustCompile(`(?i)\bexport\s+LD_`), "LD_* manipulation"},

	// Redirection to system files
	{regexp.MustCompile(`>\s*/etc/`), "write to /etc/"},
	{regexp.MustCompile(`>\s*/dev/`), "write to /dev/"},
	{regexp.MustCompile(`>\s*/proc/`), "write to /proc/"},
	{regexp.MustCompile(`>\s*/sys/`), "write to /sys/"},
}

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
	// Validate code for dangerous commands BEFORE any execution setup
	// This check is performed unless TrustedMode is enabled
	if !config.TrustedMode {
		if err := e.validateBashCode(code); err != nil {
			return nil, nil, err
		}
	}

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
// Note: Dangerous command validation is performed in Prepare() BEFORE this function is called.
// This function only adds runtime safety measures like resource limits and timeouts.
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

	// User script (already validated in Prepare() if not in TrustedMode)
	script.WriteString("# User script:\n")
	script.WriteString(code)
	script.WriteString("\n")

	return script.String()
}

// validateBashCode checks bash code for dangerous commands and returns an error if any are found.
// This function BLOCKS execution of dangerous commands rather than just warning about them.
// The check uses regex patterns to catch various forms of dangerous commands including:
// - File system destruction (rm, rmdir, mv, chmod, chown, dd, shred)
// - Privilege escalation (sudo, su, passwd)
// - Network operations (wget, curl, ssh, scp, nc, netcat)
// - System commands (mount, reboot, shutdown)
// - Command substitution ($() and backticks)
// - Access to sensitive paths (/dev/, /proc/, /sys/, /etc/passwd, /etc/shadow)
func (e *BashExecutor) validateBashCode(code string) error {
	var blockedCommands []string

	// Check against all dangerous command patterns
	for _, dp := range dangerousCommandPatterns {
		if dp.pattern.MatchString(code) {
			blockedCommands = append(blockedCommands, dp.name)
		}
	}

	// If any dangerous commands were found, return an error
	if len(blockedCommands) > 0 {
		return entities.NewBlockedCommandError(blockedCommands)
	}

	return nil
}

// IsDangerousCommand checks if a single command is dangerous.
// This is a helper function for external validation needs.
func IsDangerousCommand(code string) (bool, []string) {
	var blockedCommands []string

	for _, dp := range dangerousCommandPatterns {
		if dp.pattern.MatchString(code) {
			blockedCommands = append(blockedCommands, dp.name)
		}
	}

	return len(blockedCommands) > 0, blockedCommands
}
