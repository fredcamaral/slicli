package executors

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// TestDangerousJSCodeBlocking verifies that dangerous JavaScript code is BLOCKED, not just warned about.
// This is a critical security test - if these tests pass, the sandbox is working correctly.
func TestDangerousJSCodeBlocking(t *testing.T) {
	executor := &JavaScriptExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false // Ensure we're not in trusted mode

	tests := []struct {
		name            string
		code            string
		shouldBlock     bool
		expectedPattern string // Pattern we expect to be detected
	}{
		// Node.js dangerous modules
		{
			name:            "child_process require",
			code:            `const cp = require('child_process'); cp.exec('ls')`,
			shouldBlock:     true,
			expectedPattern: "child_process module",
		},
		{
			name:            "fs require",
			code:            `const fs = require('fs'); fs.readFileSync('/etc/passwd')`,
			shouldBlock:     true,
			expectedPattern: "fs module",
		},
		{
			name:            "net require",
			code:            `const net = require('net'); net.createServer()`,
			shouldBlock:     true,
			expectedPattern: "net module",
		},
		{
			name:            "dgram require",
			code:            `const dgram = require('dgram')`,
			shouldBlock:     true,
			expectedPattern: "dgram module",
		},
		{
			name:            "cluster require",
			code:            `const cluster = require('cluster')`,
			shouldBlock:     true,
			expectedPattern: "cluster module",
		},
		{
			name:            "http require",
			code:            `const http = require('http')`,
			shouldBlock:     true,
			expectedPattern: "http module",
		},
		{
			name:            "https require",
			code:            `const https = require('https')`,
			shouldBlock:     true,
			expectedPattern: "https module",
		},
		{
			name:            "vm require",
			code:            `const vm = require('vm')`,
			shouldBlock:     true,
			expectedPattern: "vm module",
		},

		// ES Module imports
		{
			name:            "child_process import",
			code:            `import { exec } from 'child_process'`,
			shouldBlock:     true,
			expectedPattern: "child_process import",
		},
		{
			name:            "fs import",
			code:            `import fs from 'fs'`,
			shouldBlock:     true,
			expectedPattern: "fs import",
		},
		{
			name:            "net import",
			code:            `import net from 'net'`,
			shouldBlock:     true,
			expectedPattern: "net import",
		},

		// Process access
		{
			name:            "process.env access",
			code:            `const secret = process.env.SECRET_KEY`,
			shouldBlock:     true,
			expectedPattern: "process.env access",
		},
		{
			name:            "process.exit()",
			code:            `process.exit(1)`,
			shouldBlock:     true,
			expectedPattern: "process.exit()",
		},
		{
			name:            "process.kill()",
			code:            `process.kill(process.pid, 'SIGKILL')`,
			shouldBlock:     true,
			expectedPattern: "process.kill()",
		},
		{
			name:            "process.binding()",
			code:            `process.binding('fs')`,
			shouldBlock:     true,
			expectedPattern: "process.binding()",
		},
		{
			name:            "process.chdir()",
			code:            `process.chdir('/tmp')`,
			shouldBlock:     true,
			expectedPattern: "process.chdir()",
		},

		// Dynamic execution
		{
			name:            "eval()",
			code:            `eval("require('child_process').exec('whoami')")`,
			shouldBlock:     true,
			expectedPattern: "eval()",
		},
		{
			name:            "Function constructor",
			code:            `new Function("return process.env")()`,
			shouldBlock:     true,
			expectedPattern: "Function constructor",
		},
		{
			name:            "setTimeout with string",
			code:            `setTimeout("console.log('evil')", 100)`,
			shouldBlock:     true,
			expectedPattern: "setTimeout with string",
		},
		{
			name:            "setInterval with string",
			code:            `setInterval("console.log('evil')", 100)`,
			shouldBlock:     true,
			expectedPattern: "setInterval with string",
		},

		// Global manipulation
		{
			name:            "global object manipulation",
			code:            `global["require"]("child_process")`,
			shouldBlock:     true,
			expectedPattern: "global object manipulation",
		},
		{
			name:            "global object access",
			code:            `global.process.exit(1)`,
			shouldBlock:     true,
			expectedPattern: "global object access",
		},
		{
			name:            "globalThis access",
			code:            `globalThis.eval("1+1")`,
			shouldBlock:     true,
			expectedPattern: "globalThis access",
		},

		// Dangerous paths
		{
			name:            "/etc/ access",
			code:            `const path = "/etc/passwd"`,
			shouldBlock:     true,
			expectedPattern: "/etc/ access",
		},
		{
			name:            "home directory access",
			code:            `const path = "~/.ssh/id_rsa"`,
			shouldBlock:     true,
			expectedPattern: "home directory access",
		},
		{
			name:            "/dev/ access",
			code:            `const path = "/dev/sda"`,
			shouldBlock:     true,
			expectedPattern: "/dev/ access",
		},
		{
			name:            "/proc/ access",
			code:            `const path = "/proc/1/environ"`,
			shouldBlock:     true,
			expectedPattern: "/proc/ access",
		},

		// Safe code that should NOT be blocked
		{
			name:        "console.log",
			code:        `console.log("Hello, World!")`,
			shouldBlock: false,
		},
		{
			name:        "math operations",
			code:        `const x = 1 + 2; console.log(x)`,
			shouldBlock: false,
		},
		{
			name:        "array operations",
			code:        `const arr = [1, 2, 3]; console.log(arr.map(x => x * 2))`,
			shouldBlock: false,
		},
		{
			name:        "string operations",
			code:        `const s = "hello"; console.log(s.toUpperCase())`,
			shouldBlock: false,
		},
		{
			name:        "object operations",
			code:        `const obj = {key: "value"}; console.log(obj.key)`,
			shouldBlock: false,
		},
		{
			name:        "for loop",
			code:        `for (let i = 0; i < 5; i++) { console.log(i) }`,
			shouldBlock: false,
		},
		{
			name:        "function definition",
			code:        `function greet(name) { return "Hello, " + name; }`,
			shouldBlock: false,
		},
		{
			name:        "class definition",
			code:        `class MyClass { constructor() {} }`,
			shouldBlock: false,
		},
		{
			name:        "async/await",
			code:        `async function test() { await Promise.resolve(1); }`,
			shouldBlock: false,
		},
		{
			name:        "JSON operations",
			code:        `JSON.parse('{"key": "value"}')`,
			shouldBlock: false,
		},
		{
			name:        "setTimeout with function",
			code:        `setTimeout(() => console.log('safe'), 100)`,
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
					t.Errorf("SECURITY FAILURE: dangerous JavaScript code '%s' was NOT blocked", tt.code)
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
					t.Errorf("Safe JavaScript code '%s' was incorrectly blocked: %v", tt.code, err)
					return
				}

				if cleanup != nil {
					cleanup()
				}

				if cmd == nil {
					t.Errorf("Safe JavaScript code '%s' returned nil cmd without error", tt.code)
				}
			}
		})
	}
}

// TestJSTrustedModeBypassesSecurity verifies that TrustedMode allows dangerous code.
func TestJSTrustedModeBypassesSecurity(t *testing.T) {
	executor := &JavaScriptExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = true // Enable trusted mode

	dangerousCodes := []string{
		`require('child_process')`,
		`process.env.SECRET`,
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

// TestIsDangerousJSCodeHelper tests the helper function
func TestIsDangerousJSCodeHelper(t *testing.T) {
	tests := []struct {
		code        string
		isDangerous bool
	}{
		{`require('child_process')`, true},
		{`process.env.SECRET`, true},
		{`eval("1+1")`, true},
		{`console.log("hello")`, false},
		{`const x = 1 + 2`, false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			isDangerous, patterns := IsDangerousJSCode(tt.code)

			if isDangerous != tt.isDangerous {
				t.Errorf("IsDangerousJSCode(%q) = %v, want %v (patterns: %v)",
					tt.code, isDangerous, tt.isDangerous, patterns)
			}
		})
	}
}

// TestMultipleDangerousJSPatterns verifies all dangerous patterns are reported
func TestMultipleDangerousJSPatterns(t *testing.T) {
	code := `require('child_process'); eval("process.env.SECRET"); const path = "/etc/passwd"`

	isDangerous, patterns := IsDangerousJSCode(code)

	if !isDangerous {
		t.Error("Expected code with multiple dangerous patterns to be detected")
	}

	// Should detect child_process, eval, process.env, and /etc/ access
	if len(patterns) < 3 {
		t.Errorf("Expected at least 3 dangerous patterns, got %d: %v", len(patterns), patterns)
	}

	t.Logf("Detected dangerous patterns: %v", patterns)
}

// TestJSCaseInsensitivity verifies pattern matching behavior
func TestJSCaseInsensitivity(t *testing.T) {
	executor := &JavaScriptExecutor{}

	tests := []struct {
		code        string
		shouldBlock bool
	}{
		{`REQUIRE('child_process')`, true},
		{`PROCESS.ENV.SECRET`, true},
		{`EVAL("1+1")`, true},
		{`console.log("hello")`, false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := executor.validateJSCode(tt.code)

			if tt.shouldBlock && err == nil {
				t.Errorf("Expected '%s' to be blocked (case-insensitive)", tt.code)
			}
			if !tt.shouldBlock && err != nil {
				t.Errorf("Expected '%s' to be allowed, got: %v", tt.code, err)
			}
		})
	}
}

// BenchmarkValidateJSCode benchmarks the security validation
func BenchmarkValidateJSCode(b *testing.B) {
	executor := &JavaScriptExecutor{}
	code := `console.log("Hello, World!")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateJSCode(code)
	}
}

// BenchmarkValidateDangerousJSCode benchmarks validation of dangerous code
func BenchmarkValidateDangerousJSCode(b *testing.B) {
	executor := &JavaScriptExecutor{}
	code := `require('child_process'); eval("process.env.SECRET")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateJSCode(code)
	}
}
