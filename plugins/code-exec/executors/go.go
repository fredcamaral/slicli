package executors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

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
