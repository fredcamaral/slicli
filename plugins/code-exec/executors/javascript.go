package executors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

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
