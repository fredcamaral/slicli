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

// dangerousGoPatterns defines regex patterns for dangerous Go code
// These patterns are designed to catch various forms of dangerous operations
var dangerousGoPatterns = []struct {
	pattern *regexp.Regexp
	name    string
}{
	// Dangerous imports
	{regexp.MustCompile(`"os/exec"`), "os/exec import"},
	{regexp.MustCompile(`"syscall"`), "syscall import"},
	{regexp.MustCompile(`"unsafe"`), "unsafe import"},
	{regexp.MustCompile(`"plugin"`), "plugin import"},
	{regexp.MustCompile(`"net/http"`), "net/http import"},
	{regexp.MustCompile(`"net"`), "net import"},
	{regexp.MustCompile(`"reflect"`), "reflect import"},
	{regexp.MustCompile(`"runtime/debug"`), "runtime/debug import"},

	// CGO
	{regexp.MustCompile(`import\s+"C"`), "CGO import"},
	{regexp.MustCompile(`import\s+[^"]*"C"`), "CGO import"},

	// File operations
	{regexp.MustCompile(`os\.Remove\s*\(`), "os.Remove()"},
	{regexp.MustCompile(`os\.RemoveAll\s*\(`), "os.RemoveAll()"},
	{regexp.MustCompile(`os\.Create\s*\(`), "os.Create()"},
	{regexp.MustCompile(`os\.OpenFile\s*\(`), "os.OpenFile()"},
	{regexp.MustCompile(`os\.Mkdir\s*\(`), "os.Mkdir()"},
	{regexp.MustCompile(`os\.MkdirAll\s*\(`), "os.MkdirAll()"},
	{regexp.MustCompile(`os\.Rename\s*\(`), "os.Rename()"},
	{regexp.MustCompile(`os\.Chmod\s*\(`), "os.Chmod()"},
	{regexp.MustCompile(`os\.Chown\s*\(`), "os.Chown()"},
	{regexp.MustCompile(`os\.WriteFile\s*\(`), "os.WriteFile()"},
	{regexp.MustCompile(`ioutil\.WriteFile\s*\(`), "ioutil.WriteFile()"},

	// Environment and process manipulation
	{regexp.MustCompile(`os\.Setenv\s*\(`), "os.Setenv()"},
	{regexp.MustCompile(`os\.Unsetenv\s*\(`), "os.Unsetenv()"},
	{regexp.MustCompile(`os\.Exit\s*\(`), "os.Exit()"},

	// Dangerous paths
	{regexp.MustCompile(`"/etc/`), "/etc/ access"},
	{regexp.MustCompile(`"~/`), "home directory access"},
	{regexp.MustCompile(`"/dev/`), "/dev/ access"},
	{regexp.MustCompile(`"/proc/`), "/proc/ access"},
	{regexp.MustCompile(`"/sys/`), "/sys/ access"},

	// Runtime manipulation
	{regexp.MustCompile(`runtime\.GOMAXPROCS\s*\(`), "runtime.GOMAXPROCS()"},
	{regexp.MustCompile(`runtime\.SetFinalizer\s*\(`), "runtime.SetFinalizer()"},
	{regexp.MustCompile(`runtime\.GC\s*\(`), "runtime.GC()"},
}

// GoExecutor executes Go code
type GoExecutor struct{}

// Name returns the executor name
func (e *GoExecutor) Name() string {
	return "go"
}

// IsAvailable checks if Go runtime is available
func (e *GoExecutor) IsAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// GetDefaultConfig returns default configuration for Go execution
func (e *GoExecutor) GetDefaultConfig() entities.ExecutionConfig {
	config := entities.GetDefaultExecutionConfig()
	config.Language = "go"
	config.Environment = []string{
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	}
	return config
}

// Prepare sets up Go code execution
func (e *GoExecutor) Prepare(ctx context.Context, code string, config entities.ExecutionConfig) (*exec.Cmd, func(), error) {
	// Validate code for dangerous patterns BEFORE any execution setup
	// This check is performed unless TrustedMode is enabled
	if !config.TrustedMode {
		if err := e.validateGoCode(code); err != nil {
			return nil, nil, err
		}
	}

	// Create temporary file for Go code
	tmpFile, err := os.CreateTemp("", "slicli-go-*.go")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file: %w", err)
	}

	// Determine if code is a complete program or snippet
	program := e.buildProgram(code)

	// Write Go program to file
	if _, err := tmpFile.WriteString(program); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, nil, fmt.Errorf("writing Go code: %w", err)
	}
	_ = tmpFile.Close()

	// Create Go run command
	cmd := exec.CommandContext(ctx, "go", "run", tmpFile.Name()) // #nosec G204 - go executable is hardcoded and file path is controlled

	// Setup cleanup function
	cleanup := func() {
		_ = os.Remove(tmpFile.Name())
	}

	return cmd, cleanup, nil
}

// buildProgram creates a complete Go program from code snippet
func (e *GoExecutor) buildProgram(code string) string {
	// Check if code already contains package declaration
	if strings.Contains(code, "package ") {
		return code
	}

	// Check if code contains main function
	if strings.Contains(code, "func main()") {
		return "package main\n\n" + e.addImports(code)
	}

	// Wrap code in main function with intelligent imports
	imports := e.detectImports(code)
	importBlock := ""
	if len(imports) > 0 {
		importBlock = fmt.Sprintf("import (\n\t%s\n)\n\n", strings.Join(imports, "\n\t"))
	}

	return fmt.Sprintf(`package main

%sfunc main() {
%s
}`, importBlock, e.indentCode(code))
}

// addImports adds common imports to Go code
func (e *GoExecutor) addImports(code string) string {
	imports := []string{
		`"fmt"`,
		`"math"`,
		`"strings"`,
		`"time"`,
		`"sort"`,
		`"strconv"`,
		`"os"`,
		`"io"`,
		`"bufio"`,
		`"bytes"`,
	}

	// Only add imports if not already present
	var neededImports []string
	for _, imp := range imports {
		if !strings.Contains(code, imp) {
			neededImports = append(neededImports, imp)
		}
	}

	if len(neededImports) == 0 {
		return code
	}

	importBlock := fmt.Sprintf("import (\n\t%s\n)\n\n", strings.Join(neededImports, "\n\t"))
	return importBlock + code
}

// detectImports analyzes code and returns only the imports that are actually used
func (e *GoExecutor) detectImports(code string) []string {
	var imports []string

	// Common imports and their usage patterns
	importPatterns := map[string][]string{
		`"fmt"`:     {"fmt.", "Println", "Printf", "Print", "Sprintf", "Errorf"},
		`"strings"`: {"strings."},
		`"math"`:    {"math."},
		`"time"`:    {"time.", "Time", "Duration"},
		`"sort"`:    {"sort."},
		`"strconv"`: {"strconv."},
		`"os"`:      {"os."},
		`"io"`:      {"io."},
		`"bufio"`:   {"bufio."},
		`"bytes"`:   {"bytes."},
	}

	// Check each import to see if it's used in the code
	for imp, patterns := range importPatterns {
		for _, pattern := range patterns {
			if strings.Contains(code, pattern) {
				imports = append(imports, imp)
				break
			}
		}
	}

	return imports
}

// indentCode adds proper indentation to code
func (e *GoExecutor) indentCode(code string) string {
	lines := strings.Split(code, "\n")
	var indentedLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indentedLines = append(indentedLines, "\t"+line)
		} else {
			indentedLines = append(indentedLines, line)
		}
	}

	return strings.Join(indentedLines, "\n")
}

// validateGoCode checks Go code for dangerous patterns and returns an error if any are found.
// This function BLOCKS execution of dangerous code rather than just warning about it.
func (e *GoExecutor) validateGoCode(code string) error {
	var blockedPatterns []string

	// Check against all dangerous patterns
	for _, dp := range dangerousGoPatterns {
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

// IsDangerousGoCode checks if Go code contains dangerous patterns.
// This is a helper function for external validation needs.
func IsDangerousGoCode(code string) (bool, []string) {
	var blockedPatterns []string

	for _, dp := range dangerousGoPatterns {
		if dp.pattern.MatchString(code) {
			blockedPatterns = append(blockedPatterns, dp.name)
		}
	}

	return len(blockedPatterns) > 0, blockedPatterns
}
