package main

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/fredcamaral/slicli/plugins/code-exec/executors"
)

// CodeExecPlugin implements the code execution plugin
type CodeExecPlugin struct {
	mu        sync.RWMutex
	config    map[string]interface{}
	executors map[string]entities.Executor
}

// main is required for Go plugin system
func main() {
	// This function is required but not used in plugin mode
}

// NewPlugin creates a new code execution plugin instance
func NewPlugin() *CodeExecPlugin {
	plugin := &CodeExecPlugin{
		config:    make(map[string]interface{}),
		executors: make(map[string]entities.Executor),
	}

	// Register available executors
	plugin.registerExecutors()

	return plugin
}

// registerExecutors registers all available code executors
func (p *CodeExecPlugin) registerExecutors() {
	executorList := []entities.Executor{
		&executors.GoExecutor{},
		&executors.PythonExecutor{},
		&executors.JavaScriptExecutor{},
		&executors.BashExecutor{},
	}

	for _, executor := range executorList {
		if executor.IsAvailable() {
			p.executors[executor.Name()] = executor
		}
	}
}

// Name returns the plugin name
func (p *CodeExecPlugin) Name() string {
	return "code-exec"
}

// Version returns the plugin version
func (p *CodeExecPlugin) Version() string {
	return "1.0.0"
}

// Description returns the plugin description
func (p *CodeExecPlugin) Description() string {
	return "Execute code snippets safely during presentations"
}

// Init initializes the plugin with configuration
func (p *CodeExecPlugin) Init(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store configuration
	p.config = config

	// Validate critical safety settings
	if err := p.validateSafetyConfig(config); err != nil {
		return fmt.Errorf("safety configuration validation failed: %w", err)
	}

	return nil
}

// Execute processes the input and returns the output
func (p *CodeExecPlugin) Execute(ctx context.Context, input plugin.PluginInput) (plugin.PluginOutput, error) {
	// Extract execution configuration from input options
	config := p.extractConfig(input.Options)
	config.Language = input.Language

	// Check if execution is disabled
	p.mu.RLock()
	if disabled, ok := p.config["execution_disabled"].(bool); ok && disabled {
		p.mu.RUnlock()
		return plugin.PluginOutput{
			HTML: `<div class="code-execution-disabled">
				<p><strong>Code execution is disabled</strong></p>
				<pre><code>` + input.Content + `</code></pre>
			</div>`,
			Metadata: map[string]interface{}{
				"status":   "disabled",
				"language": input.Language,
			},
		}, nil
	}
	p.mu.RUnlock()

	// Get executor for the specified language
	executor, exists := p.executors[config.Language]
	if !exists {
		return plugin.PluginOutput{
			HTML: fmt.Sprintf(`<div class="code-execution-error">
				<p><strong>Error:</strong> No executor available for language: %s</p>
				<pre><code>%s</code></pre>
			</div>`, config.Language, input.Content),
			Metadata: map[string]interface{}{
				"status":   "error",
				"language": config.Language,
				"error":    "no executor available for language: " + config.Language,
			},
		}, nil
	}

	// Execute code with safety measures
	result, err := p.executeCode(executor, input.Content, config)
	if err != nil {
		return plugin.PluginOutput{
			HTML: fmt.Sprintf(`<div class="code-execution-error">
				<p><strong>Execution failed:</strong> %v</p>
				<pre><code>%s</code></pre>
			</div>`, err, input.Content),
			Metadata: map[string]interface{}{
				"status":   "error",
				"language": config.Language,
				"error":    err.Error(),
			},
		}, nil
	}

	// Generate HTML output
	html := p.generateHTML(input.Content, result, config)

	return plugin.PluginOutput{
		HTML: html,
		Metadata: map[string]interface{}{
			"status":      result.Status,
			"language":    result.Language,
			"duration":    result.Duration.String(),
			"exit_code":   result.ExitCode,
			"truncated":   result.Truncated,
			"output_size": len(result.Output),
			"error_size":  len(result.ErrorOutput),
		},
	}, nil
}

// generateHTML generates the HTML output for the execution result
func (p *CodeExecPlugin) generateHTML(code string, result *entities.ExecutionResult, config entities.ExecutionConfig) string {
	var statusClass string
	var statusText string

	switch result.Status {
	case "success":
		statusClass = "success"
		statusText = "Success"
	case "error":
		statusClass = "error"
		statusText = "Error"
	case "timeout":
		statusClass = "timeout"
		statusText = "Timeout"
	default:
		statusClass = "unknown"
		statusText = "Unknown"
	}

	html := fmt.Sprintf(`<div class="code-execution-result">
	<div class="code-execution-header">
		<span class="language">%s</span>
		<span class="status status-%s">%s</span>
		<span class="duration">%s</span>
		<span class="exit-code">Exit: %d</span>
	</div>
	<div class="code-execution-source">
		<pre><code class="language-%s">%s</code></pre>
	</div>`,
		result.Language, statusClass, statusText,
		result.Duration.Truncate(time.Millisecond).String(),
		result.ExitCode, result.Language, code)

	// Add output if present
	if result.Output != "" {
		truncated := ""
		if result.Truncated {
			truncated = " (truncated)"
		}
		html += fmt.Sprintf(`
	<div class="code-execution-output">
		<h4>Output%s:</h4>
		<pre><code>%s</code></pre>
	</div>`, truncated, result.Output)
	}

	// Add error output if present
	if result.ErrorOutput != "" {
		html += fmt.Sprintf(`
	<div class="code-execution-error-output">
		<h4>Error Output:</h4>
		<pre><code>%s</code></pre>
	</div>`, result.ErrorOutput)
	}

	html += `</div>`

	return html
}

// Cleanup releases any resources held by the plugin
func (p *CodeExecPlugin) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear configuration
	p.config = make(map[string]interface{})

	// Clear executors
	p.executors = make(map[string]entities.Executor)

	return nil
}

// validateSafetyConfig validates safety-critical configuration
func (p *CodeExecPlugin) validateSafetyConfig(config map[string]interface{}) error {
	// Check if execution is disabled globally
	if disabled, ok := config["execution_disabled"].(bool); ok && disabled {
		return nil // No further validation needed if disabled
	}

	// Validate global memory limits
	if memLimit, ok := config["global_memory_limit"].(string); ok {
		if _, err := parseSize(memLimit); err != nil {
			return fmt.Errorf("invalid global_memory_limit: %w", err)
		}
	}

	// Validate global timeout
	if timeout, ok := config["global_timeout"].(string); ok {
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("invalid global_timeout: %w", err)
		}
	}

	// Validate language-specific configurations
	if langConfigs, ok := config["languages"].(map[string]interface{}); ok {
		for lang, langConfig := range langConfigs {
			if langConfigMap, ok := langConfig.(map[string]interface{}); ok {
				if err := p.validateLanguageConfig(lang, langConfigMap); err != nil {
					return fmt.Errorf("invalid config for language %s: %w", lang, err)
				}
			}
		}
	}

	return nil
}

// validateLanguageConfig validates language-specific configuration
func (p *CodeExecPlugin) validateLanguageConfig(language string, config map[string]interface{}) error {
	// Check if we have an executor for this language
	if _, exists := p.executors[language]; !exists {
		return fmt.Errorf("no executor available for language: %s", language)
	}

	// Validate timeout
	if timeout, ok := config["timeout"].(string); ok {
		if _, err := time.ParseDuration(timeout); err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Validate memory limit
	if memLimit, ok := config["memory_limit"].(string); ok {
		if _, err := parseSize(memLimit); err != nil {
			return fmt.Errorf("invalid memory_limit: %w", err)
		}
	}

	return nil
}

// executeCode executes code using the specified executor with safety measures
func (p *CodeExecPlugin) executeCode(executor entities.Executor, code string, config entities.ExecutionConfig) (*entities.ExecutionResult, error) {
	// Create execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Prepare execution command
	cmd, cleanup, err := executor.Prepare(ctx, code, config)
	if err != nil {
		return nil, fmt.Errorf("preparing execution: %w", err)
	}
	defer cleanup()

	// Create limited output writers
	outputWriter := newLimitedWriter(config.MaxOutputSize)
	errorWriter := newLimitedWriter(config.MaxOutputSize)

	// Set up command stdio
	cmd.Stdout = outputWriter
	cmd.Stderr = errorWriter

	// Set environment variables
	if len(config.Environment) > 0 {
		cmd.Env = filterEnvironment(config.Environment)
	}

	// Apply resource limits (Unix only)
	if err := setResourceLimits(cmd, config); err != nil {
		return nil, fmt.Errorf("setting resource limits: %w", err)
	}

	// Set process group for cleanup
	if err := setProcessGroup(cmd); err != nil {
		return nil, fmt.Errorf("setting process group: %w", err)
	}

	// Execute the command
	execErr := cmd.Run()
	duration := time.Since(startTime)

	// Determine exit code
	exitCode := 0
	status := "success"
	if execErr != nil {
		if exitError, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
		status = "error"
	}

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		status = "timeout"
		exitCode = 124
	}

	// Create execution result
	result := &entities.ExecutionResult{
		Output:      outputWriter.String(),
		ErrorOutput: errorWriter.String(),
		ExitCode:    exitCode,
		Duration:    duration,
		Language:    config.Language,
		Status:      status,
		Truncated:   outputWriter.IsTruncated() || errorWriter.IsTruncated(),
	}

	return result, nil
}

// GetSupportedLanguages returns list of supported programming languages
func (p *CodeExecPlugin) GetSupportedLanguages() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var languages []string
	for lang := range p.executors {
		languages = append(languages, lang)
	}
	return languages
}

// GetLanguageConfig returns configuration for a specific language
func (p *CodeExecPlugin) GetLanguageConfig(language string) (entities.ExecutionConfig, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	executor, exists := p.executors[language]
	if !exists {
		return entities.ExecutionConfig{}, fmt.Errorf("unsupported language: %s", language)
	}

	return executor.GetDefaultConfig(), nil
}

// Health checks plugin health and availability
func (p *CodeExecPlugin) Health() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health := map[string]interface{}{
		"status":              "healthy",
		"executors_available": len(p.executors),
		"supported_languages": p.GetSupportedLanguages(),
	}

	// Check executor health
	executorHealth := make(map[string]bool)
	for lang, executor := range p.executors {
		executorHealth[lang] = executor.IsAvailable()
	}
	health["executor_health"] = executorHealth

	// Check if execution is disabled
	if disabled, ok := p.config["execution_disabled"].(bool); ok && disabled {
		health["execution_disabled"] = true
	}

	return health
}

// Export required plugin interface for Go plugin system

// Plugin is the exported symbol that the plugin loader looks for
var Plugin plugin.Plugin = NewPlugin()
