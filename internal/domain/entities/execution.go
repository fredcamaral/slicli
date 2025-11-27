package entities

import (
	"context"
	"os/exec"
	"time"
)

// ExecutionConfig defines the configuration for code execution
type ExecutionConfig struct {
	// Language is the programming language to execute
	Language string `json:"language"`

	// Timeout is the maximum execution time (default: 5s)
	Timeout time.Duration `json:"timeout"`

	// MaxOutputSize is the maximum output size in bytes (default: 1MB)
	MaxOutputSize int `json:"max_output_size"`

	// MaxMemory is the maximum memory usage in bytes (default: 100MB)
	MaxMemory int64 `json:"max_memory"`

	// AllowNetwork determines if network access is allowed (default: false)
	AllowNetwork bool `json:"allow_network"`

	// AllowFileWrite determines if file writing is allowed (default: false)
	AllowFileWrite bool `json:"allow_file_write"`

	// Environment contains environment variables
	Environment []string `json:"environment"`

	// TrustedMode allows dangerous operations (default: false)
	TrustedMode bool `json:"trusted_mode"`
}

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	// Output is the standard output from execution
	Output string `json:"output"`

	// ErrorOutput is the standard error from execution
	ErrorOutput string `json:"error_output"`

	// ExitCode is the exit code of the executed process
	ExitCode int `json:"exit_code"`

	// Duration is how long the execution took
	Duration time.Duration `json:"duration"`

	// Language is the programming language that was executed
	Language string `json:"language"`

	// Status is the execution status (success, error, timeout)
	Status string `json:"status"`

	// Metadata contains additional execution information
	Metadata map[string]interface{} `json:"metadata"`

	// Truncated indicates if output was truncated due to size limits
	Truncated bool `json:"truncated"`
}

// Executor interface defines how to execute code for a specific language
type Executor interface {
	// Name returns the executor name (e.g., "go", "python", "javascript")
	Name() string

	// Prepare sets up the execution environment and returns the command to run
	Prepare(ctx context.Context, code string, config ExecutionConfig) (*exec.Cmd, func(), error)

	// IsAvailable checks if the required runtime is available
	IsAvailable() bool

	// GetDefaultConfig returns default configuration for this executor
	GetDefaultConfig() ExecutionConfig
}

// GetDefaultExecutionConfig returns the default execution configuration
func GetDefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		Timeout:        5 * time.Second,
		MaxOutputSize:  1024 * 1024,       // 1MB
		MaxMemory:      100 * 1024 * 1024, // 100MB
		AllowNetwork:   false,
		AllowFileWrite: false,
		Environment:    []string{},
		TrustedMode:    false,
	}
}

// WithLanguage sets the language for execution config
func (c ExecutionConfig) WithLanguage(lang string) ExecutionConfig {
	c.Language = lang
	return c
}

// WithTimeout sets the timeout for execution config
func (c ExecutionConfig) WithTimeout(timeout time.Duration) ExecutionConfig {
	c.Timeout = timeout
	return c
}

// WithTrustedMode enables or disables trusted mode
func (c ExecutionConfig) WithTrustedMode(trusted bool) ExecutionConfig {
	c.TrustedMode = trusted
	return c
}

// Validate checks if the execution config is valid
func (c ExecutionConfig) Validate() error {
	if c.Language == "" {
		return ErrInvalidLanguage
	}

	if c.Timeout <= 0 {
		return ErrInvalidTimeout
	}

	if c.MaxOutputSize <= 0 {
		return ErrInvalidOutputSize
	}

	if c.MaxMemory <= 0 {
		return ErrInvalidMemoryLimit
	}

	return nil
}

// ExecutionError represents errors that occur during code execution
type ExecutionError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e ExecutionError) Error() string {
	return e.Message
}

// Common execution errors
var (
	ErrInvalidLanguage    = ExecutionError{Type: "invalid_language", Message: "invalid or unsupported language"}
	ErrInvalidTimeout     = ExecutionError{Type: "invalid_timeout", Message: "timeout must be positive"}
	ErrInvalidOutputSize  = ExecutionError{Type: "invalid_output_size", Message: "output size limit must be positive"}
	ErrInvalidMemoryLimit = ExecutionError{Type: "invalid_memory_limit", Message: "memory limit must be positive"}
	ErrExecutionTimeout   = ExecutionError{Type: "execution_timeout", Message: "code execution timed out"}
	ErrOutputTooLarge     = ExecutionError{Type: "output_too_large", Message: "output exceeded size limit"}
	ErrMemoryExceeded     = ExecutionError{Type: "memory_exceeded", Message: "execution exceeded memory limit"}
	ErrLanguageNotFound   = ExecutionError{Type: "language_not_found", Message: "language runtime not found"}
	ErrUntrustedExecution = ExecutionError{Type: "untrusted_execution", Message: "code execution disabled in untrusted environment"}
)

// BlockedCommandError represents a security error when dangerous commands are detected
type BlockedCommandError struct {
	Command  string   // The specific command or pattern that was blocked
	Commands []string // All blocked commands found
	Message  string   // Human-readable error message
}

// Error implements the error interface
func (e *BlockedCommandError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "blocked dangerous command: " + e.Command
}

// NewBlockedCommandError creates a new BlockedCommandError with the detected commands
func NewBlockedCommandError(commands []string) *BlockedCommandError {
	if len(commands) == 0 {
		return nil
	}

	msg := "execution blocked: dangerous command(s) detected: "
	for i, cmd := range commands {
		if i > 0 {
			msg += ", "
		}
		msg += "'" + cmd + "'"
	}

	return &BlockedCommandError{
		Command:  commands[0],
		Commands: commands,
		Message:  msg,
	}
}

// IsBlockedCommandError checks if an error is a BlockedCommandError
func IsBlockedCommandError(err error) bool {
	_, ok := err.(*BlockedCommandError)
	return ok
}
