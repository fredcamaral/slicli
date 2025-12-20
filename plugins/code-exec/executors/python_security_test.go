package executors

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// TestDangerousPythonCodeBlocking verifies that dangerous Python code is BLOCKED, not just warned about.
// This is a critical security test - if these tests pass, the sandbox is working correctly.
func TestDangerousPythonCodeBlocking(t *testing.T) {
	executor := &PythonExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false // Ensure we're not in trusted mode

	tests := []struct {
		name            string
		code            string
		shouldBlock     bool
		expectedPattern string // Pattern we expect to be detected
	}{
		// System command execution
		{
			name:            "os.system()",
			code:            `os.system("ls -la")`,
			shouldBlock:     true,
			expectedPattern: "os.system()",
		},
		{
			name:            "os.popen()",
			code:            `os.popen("cat /etc/passwd")`,
			shouldBlock:     true,
			expectedPattern: "os.popen()",
		},
		{
			name:            "os.spawn",
			code:            `os.spawnl(os.P_WAIT, "/bin/sh")`,
			shouldBlock:     true,
			expectedPattern: "os.spawn*()",
		},
		{
			name:            "subprocess module",
			code:            `import subprocess; subprocess.run(["ls"])`,
			shouldBlock:     true,
			expectedPattern: "subprocess module",
		},
		{
			name:            "subprocess call",
			code:            `from subprocess import call; call(["rm", "-rf", "/"])`,
			shouldBlock:     true,
			expectedPattern: "subprocess module",
		},

		// Dynamic code execution
		{
			name:            "__import__()",
			code:            `__import__("os").system("whoami")`,
			shouldBlock:     true,
			expectedPattern: "__import__()",
		},
		{
			name:            "exec()",
			code:            `exec("import os; os.system('whoami')")`,
			shouldBlock:     true,
			expectedPattern: "exec()",
		},
		{
			name:            "eval()",
			code:            `eval("__import__('os').system('whoami')")`,
			shouldBlock:     true,
			expectedPattern: "eval()",
		},
		{
			name:            "compile()",
			code:            `code = compile("print('hello')", "<string>", "exec")`,
			shouldBlock:     true,
			expectedPattern: "compile()",
		},

		// File system danger
		{
			name:            "shutil.rmtree()",
			code:            `import shutil; shutil.rmtree("/tmp/test")`,
			shouldBlock:     true,
			expectedPattern: "shutil.rmtree()",
		},
		{
			name:            "os.remove()",
			code:            `os.remove("/etc/passwd")`,
			shouldBlock:     true,
			expectedPattern: "os.remove()",
		},
		{
			name:            "os.unlink()",
			code:            `os.unlink("/tmp/file")`,
			shouldBlock:     true,
			expectedPattern: "os.unlink()",
		},
		{
			name:            "os.rmdir()",
			code:            `os.rmdir("/tmp/dir")`,
			shouldBlock:     true,
			expectedPattern: "os.rmdir()",
		},
		{
			name:            "os.chmod()",
			code:            `os.chmod("/etc/passwd", 0o777)`,
			shouldBlock:     true,
			expectedPattern: "os.chmod()",
		},

		// Network
		{
			name:            "socket module",
			code:            `import socket; s = socket.socket()`,
			shouldBlock:     true,
			expectedPattern: "socket module",
		},
		{
			name:            "urllib module",
			code:            `import urllib.request; urllib.request.urlopen("http://evil.com")`,
			shouldBlock:     true,
			expectedPattern: "urllib module",
		},
		{
			name:            "requests module",
			code:            `import requests; requests.get("http://evil.com")`,
			shouldBlock:     true,
			expectedPattern: "requests module",
		},
		{
			name:            "http.client module",
			code:            `import http.client`,
			shouldBlock:     true,
			expectedPattern: "http.client module",
		},

		// Dangerous paths
		{
			name:            "/etc/ access",
			code:            `open("/etc/passwd", "r")`,
			shouldBlock:     true,
			expectedPattern: "/etc/ access",
		},
		{
			name:            "home directory access",
			code:            `open("~/.ssh/id_rsa", "r")`,
			shouldBlock:     true,
			expectedPattern: "home directory access",
		},
		{
			name:            "/dev/ access",
			code:            `open("/dev/sda", "rb")`,
			shouldBlock:     true,
			expectedPattern: "/dev/ access",
		},
		{
			name:            "/proc/ access",
			code:            `open("/proc/1/environ", "r")`,
			shouldBlock:     true,
			expectedPattern: "/proc/ access",
		},

		// Dangerous modules
		{
			name:            "ctypes module",
			code:            `import ctypes; ctypes.CDLL("libc.so.6")`,
			shouldBlock:     true,
			expectedPattern: "ctypes module",
		},
		{
			name:            "pickle module",
			code:            `import pickle; pickle.loads(data)`,
			shouldBlock:     true,
			expectedPattern: "pickle module",
		},
		{
			name:            "multiprocessing module",
			code:            `import multiprocessing`,
			shouldBlock:     true,
			expectedPattern: "multiprocessing module",
		},

		// Safe code that should NOT be blocked
		{
			name:        "print statement",
			code:        `print("Hello, World!")`,
			shouldBlock: false,
		},
		{
			name:        "math operations",
			code:        `x = 1 + 2; print(x)`,
			shouldBlock: false,
		},
		{
			name:        "list operations",
			code:        `mylist = [1, 2, 3]; print(len(mylist))`,
			shouldBlock: false,
		},
		{
			name:        "string operations",
			code:        `s = "hello"; print(s.upper())`,
			shouldBlock: false,
		},
		{
			name:        "dictionary operations",
			code:        `d = {"key": "value"}; print(d["key"])`,
			shouldBlock: false,
		},
		{
			name:        "for loop",
			code:        `for i in range(5): print(i)`,
			shouldBlock: false,
		},
		{
			name:        "function definition",
			code:        `def greet(name): return f"Hello, {name}"`,
			shouldBlock: false,
		},
		{
			name:        "class definition",
			code:        `class MyClass: pass`,
			shouldBlock: false,
		},
		{
			name:        "math module usage",
			code:        `import math; print(math.sqrt(16))`,
			shouldBlock: false,
		},
		{
			name:        "json module usage",
			code:        `import json; print(json.dumps({"key": "value"}))`,
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cmd, cleanup, err := executor.Prepare(ctx, tt.code, config)

			if tt.shouldBlock {
				// Code should be BLOCKED with an error
				if err == nil {
					if cleanup != nil {
						cleanup()
					}
					t.Errorf("SECURITY FAILURE: dangerous Python code '%s' was NOT blocked", tt.code)
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
				// Code should NOT be blocked
				if err != nil {
					t.Errorf("Safe Python code '%s' was incorrectly blocked: %v", tt.code, err)
					return
				}

				if cleanup != nil {
					cleanup()
				}

				if cmd == nil {
					t.Errorf("Safe Python code '%s' returned nil cmd without error", tt.code)
				}
			}
		})
	}
}

// TestPythonTrustedModeBypassesSecurity verifies that TrustedMode allows dangerous code.
func TestPythonTrustedModeBypassesSecurity(t *testing.T) {
	executor := &PythonExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = true // Enable trusted mode

	dangerousCodes := []string{
		`os.system("whoami")`,
		`import subprocess`,
		`eval("1+1")`,
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
				t.Errorf("TrustedMode should allow dangerous code, but got error: %v", err)
				return
			}

			if cmd == nil {
				t.Error("Expected valid cmd in TrustedMode")
			}
		})
	}
}

// TestIsDangerousPythonCodeHelper tests the helper function
func TestIsDangerousPythonCodeHelper(t *testing.T) {
	tests := []struct {
		code        string
		isDangerous bool
	}{
		{`os.system("ls")`, true},
		{`import subprocess`, true},
		{`eval("1+1")`, true},
		{`print("hello")`, false},
		{`x = 1 + 2`, false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			isDangerous, patterns := IsDangerousPythonCode(tt.code)

			if isDangerous != tt.isDangerous {
				t.Errorf("IsDangerousPythonCode(%q) = %v, want %v (patterns: %v)",
					tt.code, isDangerous, tt.isDangerous, patterns)
			}
		})
	}
}

// TestMultipleDangerousPythonPatterns verifies all dangerous patterns are reported
func TestMultipleDangerousPythonPatterns(t *testing.T) {
	code := `import subprocess; eval("os.system('whoami')"); open("/etc/passwd")`

	isDangerous, patterns := IsDangerousPythonCode(code)

	if !isDangerous {
		t.Error("Expected code with multiple dangerous patterns to be detected")
	}

	// Should detect subprocess, eval, and /etc/ access
	if len(patterns) < 3 {
		t.Errorf("Expected at least 3 dangerous patterns, got %d: %v", len(patterns), patterns)
	}

	t.Logf("Detected dangerous patterns: %v", patterns)
}

// TestPythonCaseInsensitivity verifies pattern matching behavior
func TestPythonCaseInsensitivity(t *testing.T) {
	executor := &PythonExecutor{}

	tests := []struct {
		code        string
		shouldBlock bool
	}{
		{`OS.SYSTEM("ls")`, true},
		{`SUBPROCESS.run(["ls"])`, true},
		{`EVAL("1+1")`, true},
		{`print("hello")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := executor.validatePythonCode(tt.code)

			if tt.shouldBlock && err == nil {
				t.Errorf("Expected '%s' to be blocked (case-insensitive)", tt.code)
			}
			if !tt.shouldBlock && err != nil {
				t.Errorf("Expected '%s' to be allowed, got: %v", tt.code, err)
			}
		})
	}
}

// BenchmarkValidatePythonCode benchmarks the security validation
func BenchmarkValidatePythonCode(b *testing.B) {
	executor := &PythonExecutor{}
	code := `print("Hello, World!")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validatePythonCode(code)
	}
}

// BenchmarkValidateDangerousPythonCode benchmarks validation of dangerous code
func BenchmarkValidateDangerousPythonCode(b *testing.B) {
	executor := &PythonExecutor{}
	code := `import subprocess; eval("os.system('whoami')")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validatePythonCode(code)
	}
}
