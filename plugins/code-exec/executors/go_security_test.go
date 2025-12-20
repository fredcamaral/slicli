package executors

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// TestDangerousGoCodeBlocking verifies that dangerous Go code is BLOCKED, not just warned about.
// This is a critical security test - if these tests pass, the sandbox is working correctly.
func TestDangerousGoCodeBlocking(t *testing.T) {
	executor := &GoExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false // Ensure we're not in trusted mode

	tests := []struct {
		name            string
		code            string
		shouldBlock     bool
		expectedPattern string // Pattern we expect to be detected
	}{
		// Dangerous imports
		{
			name: "os/exec import",
			code: `package main
import "os/exec"
func main() { exec.Command("ls").Run() }`,
			shouldBlock:     true,
			expectedPattern: "os/exec import",
		},
		{
			name: "syscall import",
			code: `package main
import "syscall"
func main() { syscall.Kill(1, syscall.SIGKILL) }`,
			shouldBlock:     true,
			expectedPattern: "syscall import",
		},
		{
			name: "unsafe import",
			code: `package main
import "unsafe"
func main() { var p unsafe.Pointer }`,
			shouldBlock:     true,
			expectedPattern: "unsafe import",
		},
		{
			name: "plugin import",
			code: `package main
import "plugin"
func main() { plugin.Open("evil.so") }`,
			shouldBlock:     true,
			expectedPattern: "plugin import",
		},
		{
			name: "net/http import",
			code: `package main
import "net/http"
func main() { http.Get("http://evil.com") }`,
			shouldBlock:     true,
			expectedPattern: "net/http import",
		},
		{
			name: "net import",
			code: `package main
import "net"
func main() { net.Dial("tcp", "evil.com:80") }`,
			shouldBlock:     true,
			expectedPattern: "net import",
		},
		{
			name: "reflect import",
			code: `package main
import "reflect"
func main() { reflect.TypeOf(1) }`,
			shouldBlock:     true,
			expectedPattern: "reflect import",
		},

		// File operations
		{
			name:            "os.Remove()",
			code:            `os.Remove("/etc/passwd")`,
			shouldBlock:     true,
			expectedPattern: "os.Remove()",
		},
		{
			name:            "os.RemoveAll()",
			code:            `os.RemoveAll("/tmp/test")`,
			shouldBlock:     true,
			expectedPattern: "os.RemoveAll()",
		},
		{
			name:            "os.Create()",
			code:            `os.Create("/tmp/file")`,
			shouldBlock:     true,
			expectedPattern: "os.Create()",
		},
		{
			name:            "os.OpenFile()",
			code:            `os.OpenFile("/tmp/file", os.O_CREATE, 0644)`,
			shouldBlock:     true,
			expectedPattern: "os.OpenFile()",
		},
		{
			name:            "os.Mkdir()",
			code:            `os.Mkdir("/tmp/dir", 0755)`,
			shouldBlock:     true,
			expectedPattern: "os.Mkdir()",
		},
		{
			name:            "os.MkdirAll()",
			code:            `os.MkdirAll("/tmp/dir/sub", 0755)`,
			shouldBlock:     true,
			expectedPattern: "os.MkdirAll()",
		},
		{
			name:            "os.Rename()",
			code:            `os.Rename("/tmp/old", "/tmp/new")`,
			shouldBlock:     true,
			expectedPattern: "os.Rename()",
		},
		{
			name:            "os.Chmod()",
			code:            `os.Chmod("/etc/passwd", 0777)`,
			shouldBlock:     true,
			expectedPattern: "os.Chmod()",
		},
		{
			name:            "os.Chown()",
			code:            `os.Chown("/etc/passwd", 0, 0)`,
			shouldBlock:     true,
			expectedPattern: "os.Chown()",
		},
		{
			name:            "os.WriteFile()",
			code:            `os.WriteFile("/tmp/file", []byte("data"), 0644)`,
			shouldBlock:     true,
			expectedPattern: "os.WriteFile()",
		},
		{
			name:            "ioutil.WriteFile()",
			code:            `ioutil.WriteFile("/tmp/file", []byte("data"), 0644)`,
			shouldBlock:     true,
			expectedPattern: "ioutil.WriteFile()",
		},

		// Environment and process manipulation
		{
			name:            "os.Setenv()",
			code:            `os.Setenv("PATH", "/evil")`,
			shouldBlock:     true,
			expectedPattern: "os.Setenv()",
		},
		{
			name:            "os.Unsetenv()",
			code:            `os.Unsetenv("PATH")`,
			shouldBlock:     true,
			expectedPattern: "os.Unsetenv()",
		},
		{
			name:            "os.Exit()",
			code:            `os.Exit(1)`,
			shouldBlock:     true,
			expectedPattern: "os.Exit()",
		},

		// Dangerous paths
		{
			name:            "/etc/ access",
			code:            `f, _ := os.Open("/etc/passwd")`,
			shouldBlock:     true,
			expectedPattern: "/etc/ access",
		},
		{
			name:            "home directory access",
			code:            `f, _ := os.Open("~/.ssh/id_rsa")`,
			shouldBlock:     true,
			expectedPattern: "home directory access",
		},
		{
			name:            "/dev/ access",
			code:            `f, _ := os.Open("/dev/sda")`,
			shouldBlock:     true,
			expectedPattern: "/dev/ access",
		},
		{
			name:            "/proc/ access",
			code:            `f, _ := os.Open("/proc/1/environ")`,
			shouldBlock:     true,
			expectedPattern: "/proc/ access",
		},
		{
			name:            "/sys/ access",
			code:            `f, _ := os.Open("/sys/power/state")`,
			shouldBlock:     true,
			expectedPattern: "/sys/ access",
		},

		// Runtime manipulation
		{
			name:            "runtime.GOMAXPROCS()",
			code:            `runtime.GOMAXPROCS(1)`,
			shouldBlock:     true,
			expectedPattern: "runtime.GOMAXPROCS()",
		},
		{
			name:            "runtime.GC()",
			code:            `runtime.GC()`,
			shouldBlock:     true,
			expectedPattern: "runtime.GC()",
		},

		// Safe code that should NOT be blocked
		{
			name:        "fmt.Println",
			code:        `fmt.Println("Hello, World!")`,
			shouldBlock: false,
		},
		{
			name:        "math operations",
			code:        `x := 1 + 2; fmt.Println(x)`,
			shouldBlock: false,
		},
		{
			name:        "slice operations",
			code:        `s := []int{1, 2, 3}; fmt.Println(len(s))`,
			shouldBlock: false,
		},
		{
			name:        "string operations",
			code:        `s := "hello"; fmt.Println(strings.ToUpper(s))`,
			shouldBlock: false,
		},
		{
			name:        "map operations",
			code:        `m := map[string]int{"a": 1}; fmt.Println(m["a"])`,
			shouldBlock: false,
		},
		{
			name:        "for loop",
			code:        `for i := 0; i < 5; i++ { fmt.Println(i) }`,
			shouldBlock: false,
		},
		{
			name:        "function definition",
			code:        `func greet(name string) string { return "Hello, " + name }`,
			shouldBlock: false,
		},
		{
			name:        "struct definition",
			code:        `type Person struct { Name string }`,
			shouldBlock: false,
		},
		{
			name:        "interface definition",
			code:        `type Greeter interface { Greet() string }`,
			shouldBlock: false,
		},
		{
			name:        "goroutine (safe)",
			code:        `go func() { fmt.Println("hello") }()`,
			shouldBlock: false,
		},
		{
			name:        "channel operations",
			code:        `ch := make(chan int); go func() { ch <- 1 }(); <-ch`,
			shouldBlock: false,
		},
		{
			name:        "time operations",
			code:        `time.Sleep(time.Second)`,
			shouldBlock: false,
		},
		{
			name:        "sort operations",
			code:        `sort.Ints([]int{3, 1, 2})`,
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
					t.Errorf("SECURITY FAILURE: dangerous Go code was NOT blocked: %s", tt.name)
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
					t.Errorf("Safe Go code '%s' was incorrectly blocked: %v", tt.name, err)
					return
				}

				if cleanup != nil {
					cleanup()
				}

				if cmd == nil {
					t.Errorf("Safe Go code '%s' returned nil cmd without error", tt.name)
				}
			}
		})
	}
}

// TestGoTrustedModeBypassesSecurity verifies that TrustedMode allows dangerous code.
func TestGoTrustedModeBypassesSecurity(t *testing.T) {
	executor := &GoExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = true // Enable trusted mode

	dangerousCodes := []string{
		`import "os/exec"`,
		`os.Remove("/tmp/test")`,
		`import "unsafe"`,
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

// TestIsDangerousGoCodeHelper tests the helper function
func TestIsDangerousGoCodeHelper(t *testing.T) {
	tests := []struct {
		code        string
		isDangerous bool
	}{
		{`import "os/exec"`, true},
		{`os.Remove("/tmp")`, true},
		{`import "unsafe"`, true},
		{`fmt.Println("hello")`, false},
		{`x := 1 + 2`, false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			isDangerous, patterns := IsDangerousGoCode(tt.code)

			if isDangerous != tt.isDangerous {
				t.Errorf("IsDangerousGoCode(%q) = %v, want %v (patterns: %v)",
					tt.code, isDangerous, tt.isDangerous, patterns)
			}
		})
	}
}

// TestMultipleDangerousGoPatterns verifies all dangerous patterns are reported
func TestMultipleDangerousGoPatterns(t *testing.T) {
	code := `import "os/exec"; import "unsafe"; os.Remove("/etc/passwd")`

	isDangerous, patterns := IsDangerousGoCode(code)

	if !isDangerous {
		t.Error("Expected code with multiple dangerous patterns to be detected")
	}

	// Should detect os/exec, unsafe, os.Remove, and /etc/ access
	if len(patterns) < 3 {
		t.Errorf("Expected at least 3 dangerous patterns, got %d: %v", len(patterns), patterns)
	}

	t.Logf("Detected dangerous patterns: %v", patterns)
}

// TestGoCGOBlocking tests that CGO imports are blocked
func TestGoCGOBlocking(t *testing.T) {
	executor := &GoExecutor{}
	config := executor.GetDefaultConfig()
	config.TrustedMode = false

	cgoCode := `package main
import "C"
func main() { }
`

	err := executor.validateGoCode(cgoCode)
	if err == nil {
		t.Error("CGO import should be blocked")
	}

	if !entities.IsBlockedCommandError(err) {
		t.Errorf("Expected BlockedCommandError for CGO, got: %T - %v", err, err)
	}
}

// BenchmarkValidateGoCode benchmarks the security validation
func BenchmarkValidateGoCode(b *testing.B) {
	executor := &GoExecutor{}
	code := `fmt.Println("Hello, World!")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateGoCode(code)
	}
}

// BenchmarkValidateDangerousGoCode benchmarks validation of dangerous code
func BenchmarkValidateDangerousGoCode(b *testing.B) {
	executor := &GoExecutor{}
	code := `import "os/exec"; import "unsafe"; os.Remove("/tmp")`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.validateGoCode(code)
	}
}
