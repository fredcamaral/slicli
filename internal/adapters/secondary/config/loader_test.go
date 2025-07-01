package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTOMLLoader_LoadGlobal(t *testing.T) {
	t.Run("creates config on first run", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		globalPath := filepath.Join(tmpDir, "config.toml")
		loader := &TOMLLoader{
			globalPath: globalPath,
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		config, err := loader.LoadGlobal(ctx)
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Check that file was created
		_, err = os.Stat(globalPath)
		assert.NoError(t, err)

		// Verify default values
		assert.Equal(t, "localhost", config.Server.Host)
		assert.Equal(t, 1000, config.Server.Port)
		assert.Equal(t, "default", config.Theme.Name)
		assert.True(t, config.Browser.AutoOpen)
		assert.Equal(t, 200, config.Watcher.IntervalMs)
	})

	t.Run("loads existing config", func(t *testing.T) {
		// Create temporary directory and config file
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		globalPath := filepath.Join(tmpDir, "config.toml")

		// Write test config
		configContent := `
[server]
host = "0.0.0.0"
port = 8080

[theme]
name = "professional"

[browser]
auto_open = false
browser = "firefox"

[watcher]
interval_ms = 200

[plugins]
enabled = true
`
		err = os.WriteFile(globalPath, []byte(configContent), 0644)
		require.NoError(t, err)

		loader := &TOMLLoader{
			globalPath: globalPath,
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		config, err := loader.LoadGlobal(ctx)
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Verify loaded values
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 8080, config.Server.Port)
		assert.Equal(t, "professional", config.Theme.Name)
		assert.False(t, config.Browser.AutoOpen)
		assert.Equal(t, "firefox", config.Browser.Browser)
	})

	t.Run("fails with invalid TOML", func(t *testing.T) {
		// Create temporary directory and invalid config file
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		globalPath := filepath.Join(tmpDir, "config.toml")

		// Write invalid TOML
		invalidContent := `
[server
host = "localhost"
`
		err = os.WriteFile(globalPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		loader := &TOMLLoader{
			globalPath: globalPath,
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		_, err = loader.LoadGlobal(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing TOML")
	})

	t.Run("fails with invalid config values", func(t *testing.T) {
		// Create temporary directory and config with invalid values
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		globalPath := filepath.Join(tmpDir, "config.toml")

		// Write config with invalid port
		configContent := `
[server]
port = -1

[theme]
name = "default"
`
		err = os.WriteFile(globalPath, []byte(configContent), 0644)
		require.NoError(t, err)

		loader := &TOMLLoader{
			globalPath: globalPath,
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		_, err = loader.LoadGlobal(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config")
	})
}

func TestTOMLLoader_LoadLocal(t *testing.T) {
	t.Run("loads existing local config", func(t *testing.T) {
		// Create temporary directory structure
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		localPath := filepath.Join(tmpDir, "slicli.toml")

		// Write test config
		configContent := `
[server]
port = 4000

[theme]
name = "custom"

[watcher]
interval_ms = 150

[plugins]
enabled = true
`
		err = os.WriteFile(localPath, []byte(configContent), 0644)
		require.NoError(t, err)

		loader := &TOMLLoader{
			globalPath: "unused",
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		config, err := loader.LoadLocal(ctx, tmpDir)
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Verify loaded values
		assert.Equal(t, 4000, config.Server.Port)
		assert.Equal(t, "custom", config.Theme.Name)
	})

	t.Run("returns nil for non-existent local config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		loader := &TOMLLoader{
			globalPath: "unused",
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		config, err := loader.LoadLocal(ctx, tmpDir)
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("fails with invalid local config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		localPath := filepath.Join(tmpDir, "slicli.toml")

		// Write invalid config
		configContent := `
[theme]
name = ""
`
		err = os.WriteFile(localPath, []byte(configContent), 0644)
		require.NoError(t, err)

		loader := &TOMLLoader{
			globalPath: "unused",
			localName:  "slicli.toml",
		}

		ctx := context.Background()
		_, err = loader.LoadLocal(ctx, tmpDir)
		assert.Error(t, err)
	})
}

func TestTOMLLoader_CreateDefaults(t *testing.T) {
	t.Run("creates default config file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		configPath := filepath.Join(tmpDir, "nested", "config.toml")
		loader := NewTOMLLoader()

		ctx := context.Background()
		err = loader.CreateDefaults(ctx, configPath)
		require.NoError(t, err)

		// Check that file was created
		_, err = os.Stat(configPath)
		assert.NoError(t, err)

		// Check that directory was created
		dir := filepath.Dir(configPath)
		_, err = os.Stat(dir)
		assert.NoError(t, err)

		// Verify file contents by loading it
		config, err := loader.loadConfig(configPath)
		require.NoError(t, err)
		assert.Equal(t, "localhost", config.Server.Host)
		assert.Equal(t, 1000, config.Server.Port)
	})

	t.Run("fails with permission error", func(t *testing.T) {
		// Try to create in root directory (should fail with permission error)
		configPath := "/root/config.toml"
		loader := NewTOMLLoader()

		ctx := context.Background()
		err := loader.CreateDefaults(ctx, configPath)
		assert.Error(t, err)
	})
}

func TestTOMLLoader_GetPaths(t *testing.T) {
	t.Run("returns correct global path", func(t *testing.T) {
		loader := NewTOMLLoader()
		globalPath := loader.GetGlobalPath()

		assert.Contains(t, globalPath, ".config")
		assert.Contains(t, globalPath, "slicli")
		assert.Contains(t, globalPath, "config.toml")
	})

	t.Run("returns correct local path", func(t *testing.T) {
		loader := NewTOMLLoader()
		localPath := loader.GetLocalPath("/some/project")

		expected := filepath.Join("/some/project", "slicli.toml")
		assert.Equal(t, expected, localPath)
	})
}

func TestNewTOMLLoader(t *testing.T) {
	t.Run("creates loader with default paths", func(t *testing.T) {
		loader := NewTOMLLoader()
		assert.NotNil(t, loader)

		globalPath := loader.GetGlobalPath()
		assert.NotEmpty(t, globalPath)
		assert.Contains(t, globalPath, "config.toml")
	})
}

func TestTOMLLoader_loadConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "slicli-test-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		configPath := filepath.Join(tmpDir, "test.toml")
		configContent := `
[server]
host = "example.com"
port = 9000

[theme]
name = "test-theme"

[watcher]
interval_ms = 150
debounce_ms = 300
`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		loader := NewTOMLLoader()
		config, err := loader.loadConfig(configPath)
		require.NoError(t, err)

		assert.Equal(t, "example.com", config.Server.Host)
		assert.Equal(t, 9000, config.Server.Port)
		assert.Equal(t, "test-theme", config.Theme.Name)
		assert.Equal(t, 150, config.Watcher.IntervalMs)
		assert.Equal(t, 300, config.Watcher.DebounceMs)
	})

	t.Run("fails with non-existent file", func(t *testing.T) {
		loader := NewTOMLLoader()
		_, err := loader.loadConfig("/non/existent/file.toml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reading config")
	})
}
