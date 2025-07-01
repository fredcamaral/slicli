package main

import (
	"context"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/pkg/plugin"
)

func TestNewPlugin(t *testing.T) {
	p := NewPlugin()

	if p == nil {
		t.Fatal("NewPlugin() returned nil")
	}

	if p.config == nil {
		t.Error("Plugin config not initialized")
	}

	if p.executors == nil {
		t.Error("Plugin executors not initialized")
	}
}

func TestGetInfo(t *testing.T) {
	p := NewPlugin()

	if p.Name() != "code-exec" {
		t.Errorf("Expected plugin name 'code-exec', got '%s'", p.Name())
	}

	if p.Version() == "" {
		t.Error("Plugin version is empty")
	}

	if p.Description() == "" {
		t.Error("Plugin description is empty")
	}
}

func TestInit(t *testing.T) {
	p := NewPlugin()

	tests := []struct {
		name      string
		config    map[string]interface{}
		expectErr bool
	}{
		{
			name:      "empty config",
			config:    map[string]interface{}{},
			expectErr: false,
		},
		{
			name: "valid config",
			config: map[string]interface{}{
				"global_timeout":      "30s",
				"global_memory_limit": "100MB",
				"execution_disabled":  false,
			},
			expectErr: false,
		},
		{
			name: "invalid timeout",
			config: map[string]interface{}{
				"global_timeout": "invalid",
			},
			expectErr: true,
		},
		{
			name: "invalid memory limit",
			config: map[string]interface{}{
				"global_memory_limit": "invalid",
			},
			expectErr: true,
		},
		{
			name: "execution disabled",
			config: map[string]interface{}{
				"execution_disabled": true,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Init(tt.config)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	p := NewPlugin()
	languages := p.GetSupportedLanguages()

	if len(languages) == 0 {
		t.Error("No supported languages found")
	}

	// Check for expected languages (if their runtimes are available)
	expectedLanguages := []string{"go"}
	for _, lang := range expectedLanguages {
		found := false
		for _, supported := range languages {
			if supported == lang {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Language %s not supported (runtime may not be available)", lang)
		}
	}
}

func TestGetLanguageConfig(t *testing.T) {
	p := NewPlugin()

	// Test with Go (should always be available in Go environment)
	config, err := p.GetLanguageConfig("go")
	if err != nil {
		t.Errorf("Error getting Go config: %v", err)
	}

	if config.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", config.Language)
	}

	// Test with invalid language
	_, err = p.GetLanguageConfig("invalid-language")
	if err == nil {
		t.Error("Expected error for invalid language")
	}
}

func TestHealth(t *testing.T) {
	p := NewPlugin()
	health := p.Health()

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", health["status"])
	}

	if health["executors_available"] == nil {
		t.Error("executors_available not present in health check")
	}

	if health["supported_languages"] == nil {
		t.Error("supported_languages not present in health check")
	}
}

func TestProcessWithDisabledExecution(t *testing.T) {
	p := NewPlugin()

	// Configure with execution disabled
	config := map[string]interface{}{
		"execution_disabled": true,
	}
	err := p.Init(config)
	if err != nil {
		t.Fatalf("Error configuring plugin: %v", err)
	}

	// Try to execute code
	input := plugin.PluginInput{
		Content:  `fmt.Println("Hello, World!")`,
		Language: "go",
		Options:  map[string]interface{}{},
	}

	result, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Metadata["status"] != "disabled" {
		t.Errorf("Expected status 'disabled', got '%v'", result.Metadata["status"])
	}
}

func TestProcessWithInvalidLanguage(t *testing.T) {
	p := NewPlugin()

	input := plugin.PluginInput{
		Content:  `print("Hello")`,
		Language: "invalid-language",
		Options:  map[string]interface{}{},
	}

	result, err := p.Execute(context.Background(), input)
	// Should not error, but should indicate error in metadata
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Metadata["status"] != "error" {
		t.Error("Expected error status for invalid language")
	}
}

func TestProcessGoCode(t *testing.T) {
	p := NewPlugin()

	// Skip if Go is not available
	if !isLanguageSupported(p, "go") {
		t.Skip("Go runtime not available")
	}

	tests := []struct {
		name     string
		code     string
		options  map[string]interface{}
		expectOk bool
	}{
		{
			name: "simple print",
			code: `fmt.Println("Hello, World!")`,
			options: map[string]interface{}{
				"language":         "go",
				"allow_file_write": true, // Go compilation needs file writes
			},
			expectOk: true,
		},
		{
			name: "syntax error",
			code: `fmt.Println("Hello, World!"`,
			options: map[string]interface{}{
				"language": "go",
			},
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := plugin.PluginInput{
				Content:  tt.code,
				Language: tt.options["language"].(string),
				Options:  tt.options,
			}

			result, err := p.Execute(context.Background(), input)
			if err != nil {
				t.Errorf("Execute error: %v", err)
				return
			}

			if tt.expectOk {
				if result.Metadata["status"] != "success" {
					t.Errorf("Expected success, got status: %v", result.Metadata["status"])
				}
			} else {
				if result.Metadata["status"] == "success" {
					t.Error("Expected failure but got success")
				}
			}
		})
	}
}

func TestConfigExtraction(t *testing.T) {
	p := NewPlugin()

	options := map[string]interface{}{
		"language":         "python",
		"timeout":          "15s",
		"max_output":       "5KB",
		"max_memory":       "50MB",
		"trusted":          true,
		"allow_network":    false,
		"allow_file_write": false,
		"environment": map[string]interface{}{
			"TEST_VAR": "test_value",
		},
	}

	config := p.extractConfig(options)

	if config.Language != "python" {
		t.Errorf("Expected language 'python', got '%s'", config.Language)
	}

	if config.Timeout != 15*time.Second {
		t.Errorf("Expected timeout 15s, got %v", config.Timeout)
	}

	if config.MaxOutputSize != 5*1024 {
		t.Errorf("Expected max output 5KB, got %d", config.MaxOutputSize)
	}

	if config.MaxMemory != 50*1024*1024 {
		t.Errorf("Expected max memory 50MB, got %d", config.MaxMemory)
	}

	if !config.TrustedMode {
		t.Error("Expected trusted mode to be true")
	}
}

func TestCleanup(t *testing.T) {
	p := NewPlugin()

	// Configure the plugin
	config := map[string]interface{}{
		"test": "value",
	}
	err := p.Init(config)
	if err != nil {
		t.Fatalf("Error configuring plugin: %v", err)
	}

	// Cleanup
	err = p.Cleanup()
	if err != nil {
		t.Errorf("Cleanup error: %v", err)
	}

	// Verify cleanup
	if len(p.config) != 0 {
		t.Error("Config not cleared after cleanup")
	}

	if len(p.executors) != 0 {
		t.Error("Executors not cleared after cleanup")
	}
}

// Helper functions

func isLanguageSupported(plugin *CodeExecPlugin, language string) bool {
	languages := plugin.GetSupportedLanguages()
	for _, lang := range languages {
		if lang == language {
			return true
		}
	}
	return false
}

// Benchmark tests

func BenchmarkNewPlugin(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewPlugin()
	}
}

func BenchmarkGetSupportedLanguages(b *testing.B) {
	p := NewPlugin()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p.GetSupportedLanguages()
	}
}

func BenchmarkHealth(b *testing.B) {
	p := NewPlugin()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p.Health()
	}
}
