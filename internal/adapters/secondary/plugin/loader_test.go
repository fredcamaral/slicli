package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPlugin implements the plugin.Plugin interface for testing.
type MockPlugin struct {
	name        string
	version     string
	description string
	initCalled  bool
	initError   error
	execError   error
}

func (m *MockPlugin) Name() string        { return m.name }
func (m *MockPlugin) Version() string     { return m.version }
func (m *MockPlugin) Description() string { return m.description }

func (m *MockPlugin) Init(config map[string]interface{}) error {
	m.initCalled = true
	return m.initError
}

func (m *MockPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	if m.execError != nil {
		return pluginapi.PluginOutput{}, m.execError
	}
	return pluginapi.PluginOutput{
		HTML: "<div>" + input.Content + "</div>",
	}, nil
}

func (m *MockPlugin) Cleanup() error {
	return nil
}

func TestGoPluginLoader_Validate(t *testing.T) {
	loader := NewGoPluginLoader("1.0.0")

	tests := []struct {
		name    string
		plugin  pluginapi.Plugin
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid plugin",
			plugin: &MockPlugin{
				name:        "test-plugin",
				version:     "1.0.0",
				description: "Test plugin",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			plugin: &MockPlugin{
				name:        "",
				version:     "1.0.0",
				description: "Test plugin",
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "invalid name characters",
			plugin: &MockPlugin{
				name:        "test plugin", // Space not allowed
				version:     "1.0.0",
				description: "Test plugin",
			},
			wantErr: true,
			errMsg:  "invalid plugin name",
		},
		{
			name: "empty version",
			plugin: &MockPlugin{
				name:        "test-plugin",
				version:     "",
				description: "Test plugin",
			},
			wantErr: true,
			errMsg:  "version cannot be empty",
		},
		{
			name: "invalid version format",
			plugin: &MockPlugin{
				name:        "test-plugin",
				version:     "1.0", // Missing patch version
				description: "Test plugin",
			},
			wantErr: true,
			errMsg:  "invalid plugin version",
		},
		{
			name: "empty description",
			plugin: &MockPlugin{
				name:        "test-plugin",
				version:     "1.0.0",
				description: "",
			},
			wantErr: true,
			errMsg:  "description cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.Validate(tt.plugin)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGoPluginLoader_LoadManifest(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid manifest
	manifestPath := filepath.Join(tmpDir, "plugin.toml")
	manifestContent := `
[metadata]
name = "test-plugin"
version = "1.0.0"
description = "Test plugin"
author = "Test Author"
type = "processor"

[requirements]
min_slicli_version = "0.1.0"
max_slicli_version = "2.0.0"
os = ["linux", "darwin"]
arch = ["amd64"]

[capabilities]
input_formats = ["text", "markdown"]
output_formats = ["html"]
concurrent = true
`
	err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	loader := NewGoPluginLoader("1.0.0")
	ctx := context.Background()

	manifest, err := loader.LoadManifest(ctx, manifestPath)
	require.NoError(t, err)
	require.NotNil(t, manifest)

	assert.Equal(t, "test-plugin", manifest.Metadata.Name)
	assert.Equal(t, "1.0.0", manifest.Metadata.Version)
	assert.Equal(t, entities.PluginTypeProcessor, manifest.Metadata.Type)
	assert.True(t, manifest.Capabilities.Concurrent)
}

func TestGoPluginLoader_LoadManifest_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "plugin.toml")

	// Invalid TOML
	err := os.WriteFile(manifestPath, []byte("invalid toml content"), 0644)
	require.NoError(t, err)

	loader := NewGoPluginLoader("1.0.0")
	ctx := context.Background()

	_, err = loader.LoadManifest(ctx, manifestPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing manifest")
}

func TestGoPluginLoader_Discover(t *testing.T) {
	// Create test directories
	tmpDir := t.TempDir()
	pluginDir1 := filepath.Join(tmpDir, "plugins1")
	pluginDir2 := filepath.Join(tmpDir, "plugins2")
	require.NoError(t, os.MkdirAll(pluginDir1, 0755))
	require.NoError(t, os.MkdirAll(pluginDir2, 0755))

	// Create some .so files (they won't actually load, but we can test discovery)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir1, "plugin1.so"), []byte("fake"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir1, "plugin2.so"), []byte("fake"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir2, "plugin3.so"), []byte("fake"), 0644))

	// Create a non-.so file that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir1, "notaplugin.txt"), []byte("text"), 0644))

	// Create manifests for some plugins
	manifest1 := `
[metadata]
name = "plugin1"
version = "1.0.0"
description = "Plugin 1"
type = "processor"

[requirements]
min_slicli_version = "0.1.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir1, "plugin.toml"), []byte(manifest1), 0644))

	loader := NewGoPluginLoader("1.0.0")
	ctx := context.Background()

	// Note: Discovery will fail to actually load the plugins since they're not real .so files,
	// but it should still find them
	_, err := loader.Discover(ctx, []string{pluginDir1, pluginDir2, "/nonexistent"})
	assert.NoError(t, err)

	// We can't test the actual loading, but we can verify the directory walking works
	// In a real test with actual plugins, we'd check the discovered plugins
}

func TestIsValidPluginName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid alphanumeric", "plugin123", true},
		{"valid with hyphen", "my-plugin", true},
		{"valid with underscore", "my_plugin", true},
		{"valid mixed", "My_Plugin-123", true},
		{"empty", "", false},
		{"with space", "my plugin", false},
		{"with special char", "my@plugin", false},
		{"with dot", "my.plugin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidPluginName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid semver", "1.2.3", true},
		{"valid with v prefix", "v1.2.3", true},
		{"valid with pre-release", "1.2.3-alpha", true},
		{"valid with pre-release and build", "1.2.3-alpha+build123", true},
		{"missing patch", "1.2", false},
		{"missing minor", "1", false},
		{"non-numeric", "a.b.c", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidVersion(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
