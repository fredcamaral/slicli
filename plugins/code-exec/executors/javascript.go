package executors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// dangerousJSPatterns defines regex patterns for dangerous JavaScript/Node.js code
// These patterns are designed to catch various forms of dangerous operations
var dangerousJSPatterns = []struct {
	pattern *regexp.Regexp
	name    string
}{
	// Node.js dangerous modules
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]child_process`), "child_process module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]fs['"]\s*\)`), "fs module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]net['"]\s*\)`), "net module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]dgram['"]\s*\)`), "dgram module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]cluster['"]\s*\)`), "cluster module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]http['"]\s*\)`), "http module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]https['"]\s*\)`), "https module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]dns['"]\s*\)`), "dns module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]tls['"]\s*\)`), "tls module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]vm['"]\s*\)`), "vm module"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"]repl['"]\s*\)`), "repl module"},

	// ES Module imports of dangerous modules
	{regexp.MustCompile(`(?i)import\s+.*\s+from\s+['"]child_process`), "child_process import"},
	{regexp.MustCompile(`(?i)import\s+.*\s+from\s+['"]fs['"]\s*`), "fs import"},
	{regexp.MustCompile(`(?i)import\s+.*\s+from\s+['"]net['"]\s*`), "net import"},

	// Process access
	{regexp.MustCompile(`(?i)\bprocess\.env\b`), "process.env access"},
	{regexp.MustCompile(`(?i)\bprocess\.exit\b`), "process.exit()"},
	{regexp.MustCompile(`(?i)\bprocess\.kill\b`), "process.kill()"},
	{regexp.MustCompile(`(?i)\bprocess\.binding\b`), "process.binding()"},
	{regexp.MustCompile(`(?i)\bprocess\.dlopen\b`), "process.dlopen()"},
	{regexp.MustCompile(`(?i)\bprocess\.chdir\b`), "process.chdir()"},

	// Dynamic execution
	{regexp.MustCompile(`(?i)\beval\s*\(`), "eval()"},
	{regexp.MustCompile(`(?i)\bFunction\s*\(`), "Function constructor"},
	{regexp.MustCompile(`(?i)\bsetTimeout\s*\(\s*['"]`), "setTimeout with string"},
	{regexp.MustCompile(`(?i)\bsetInterval\s*\(\s*['"]`), "setInterval with string"},

	// Global manipulation
	{regexp.MustCompile(`(?i)\bglobal\s*\[`), "global object manipulation"},
	{regexp.MustCompile(`(?i)\bglobal\s*\.`), "global object access"},
	{regexp.MustCompile(`(?i)\bglobalThis\b`), "globalThis access"},

	// Dangerous paths
	{regexp.MustCompile(`['"]/etc/`), "/etc/ access"},
	{regexp.MustCompile(`['"]~/`), "home directory access"},
	{regexp.MustCompile(`['"]/dev/`), "/dev/ access"},
	{regexp.MustCompile(`['"]/proc/`), "/proc/ access"},
	{regexp.MustCompile(`['"]/sys/`), "/sys/ access"},

	// Native addons and binary execution
	{regexp.MustCompile(`(?i)\.node['"]\s*\)`), "native addon loading"},
	{regexp.MustCompile(`(?i)require\s*\(\s*['"].*\.node['"]\s*\)`), "native module require"},
}

// JavaScriptExecutor executes JavaScript code using Node.js
type JavaScriptExecutor struct{}

// Name returns the executor name
func (e *JavaScriptExecutor) Name() string {
	return "javascript"
}

// IsAvailable checks if Node.js runtime is available
func (e *JavaScriptExecutor) IsAvailable() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

// GetDefaultConfig returns default configuration for JavaScript execution
func (e *JavaScriptExecutor) GetDefaultConfig() entities.ExecutionConfig {
	config := entities.GetDefaultExecutionConfig()
	config.Language = "javascript"
	config.Environment = []string{
		"NODE_ENV=sandbox",
		"NODE_OPTIONS=--max-old-space-size=100", // Limit memory to 100MB
	}
	return config
}

// Prepare sets up JavaScript code execution
func (e *JavaScriptExecutor) Prepare(ctx context.Context, code string, config entities.ExecutionConfig) (*exec.Cmd, func(), error) {
	// Validate code for dangerous patterns BEFORE any execution setup
	// This check is performed unless TrustedMode is enabled
	if !config.TrustedMode {
		if err := e.validateJSCode(code); err != nil {
			return nil, nil, err
		}
	}

	// Create temporary file for JavaScript code
	tmpFile, err := os.CreateTemp("", "slicli-js-*.js")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file: %w", err)
	}

	// Prepare JavaScript code with safety measures
	jsCode := e.prepareJavaScriptCode(code)
	if _, err := tmpFile.WriteString(jsCode); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("writing JavaScript code: %w", err)
	}
	_ = tmpFile.Close()

	// Create Node.js execution command
	cmd := exec.CommandContext(ctx, "node", tmpFile.Name()) // #nosec G204 - node executable is hardcoded and file path is controlled

	// Setup cleanup function
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	return cmd, cleanup, nil
}

// prepareJavaScriptCode wraps JavaScript code with safety measures
func (e *JavaScriptExecutor) prepareJavaScriptCode(code string) string {
	safetyWrapper := `
// Safety and resource management
process.on('uncaughtException', (err) => {
    console.error('Uncaught Exception:', err.message);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('Unhandled Rejection at:', promise, 'reason:', reason);
    process.exit(1);
});

// Set up timeout (will be overridden by external timeout)
const TIMEOUT_MS = 5000;
const timeoutId = setTimeout(() => {
    console.error('Execution timed out');
    process.exit(124);
}, TIMEOUT_MS);

// Memory usage monitoring
let memoryCheckInterval;
if (typeof process.memoryUsage === 'function') {
    memoryCheckInterval = setInterval(() => {
        const usage = process.memoryUsage();
        // 100MB limit
        if (usage.heapUsed > 100 * 1024 * 1024) {
            console.error('Memory limit exceeded');
            process.exit(125);
        }
    }, 100);
}

// Common utilities available to user code
const Math = require('math') || global.Math;
const JSON = require('json') || global.JSON;

// Cleanup function
function cleanup() {
    clearTimeout(timeoutId);
    if (memoryCheckInterval) {
        clearInterval(memoryCheckInterval);
    }
}

// Wrap user code in try-catch
try {
`

	cleanupCode := `
} catch (error) {
    console.error('Runtime Error:', error.message);
    if (error.stack) {
        console.error(error.stack);
    }
    process.exit(1);
} finally {
    cleanup();
}
`

	return safetyWrapper + e.indentCode(code) + cleanupCode
}

// indentCode adds proper indentation to JavaScript code
func (e *JavaScriptExecutor) indentCode(code string) string {
	lines := strings.Split(code, "\n")
	var indentedLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indentedLines = append(indentedLines, "    "+line)
		} else {
			indentedLines = append(indentedLines, line)
		}
	}

	return strings.Join(indentedLines, "\n")
}

// validateJSCode checks JavaScript code for dangerous patterns and returns an error if any are found.
// This function BLOCKS execution of dangerous code rather than just warning about it.
func (e *JavaScriptExecutor) validateJSCode(code string) error {
	var blockedPatterns []string

	// Check against all dangerous patterns
	for _, dp := range dangerousJSPatterns {
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

// IsDangerousJSCode checks if JavaScript code contains dangerous patterns.
// This is a helper function for external validation needs.
func IsDangerousJSCode(code string) (bool, []string) {
	var blockedPatterns []string

	for _, dp := range dangerousJSPatterns {
		if dp.pattern.MatchString(code) {
			blockedPatterns = append(blockedPatterns, dp.name)
		}
	}

	return len(blockedPatterns) > 0, blockedPatterns
}
