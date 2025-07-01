package config

import (
	"os"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestConfigMerger_Merge(t *testing.T) {
	merger := NewConfigMerger()

	t.Run("merge with no configs returns defaults", func(t *testing.T) {
		result := merger.Merge()
		assert.NotNil(t, result)
		assert.Equal(t, "localhost", result.Server.Host)
		assert.Equal(t, 1000, result.Server.Port)
		assert.Equal(t, "default", result.Theme.Name)
	})

	t.Run("merge single config", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "example.com",
				Port: 8080,
			},
			Theme: entities.ThemeConfig{
				Name: "custom",
			},
		}

		result := merger.Merge(config)
		assert.Equal(t, "example.com", result.Server.Host)
		assert.Equal(t, 8080, result.Server.Port)
		assert.Equal(t, "custom", result.Theme.Name)
	})

	t.Run("merge multiple configs with precedence", func(t *testing.T) {
		base := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
			Theme: entities.ThemeConfig{
				Name: "default",
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true,
				Browser:  "default",
			},
		}

		override := &entities.Config{
			Server: entities.ServerConfig{
				Host: "0.0.0.0", // Override host
				// Port not specified, should keep base value
			},
			Theme: entities.ThemeConfig{
				Name: "professional", // Override theme
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true, // Explicitly set to preserve base value
				Browser:  "",   // Keep base browser
			},
		}

		result := merger.Merge(base, override)
		assert.Equal(t, "0.0.0.0", result.Server.Host)
		assert.Equal(t, 1000, result.Server.Port) // From base
		assert.Equal(t, "professional", result.Theme.Name)
		assert.True(t, result.Browser.AutoOpen)            // From base
		assert.Equal(t, "default", result.Browser.Browser) // From base
	})

	t.Run("merge handles nil configs", func(t *testing.T) {
		base := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
		}

		result := merger.Merge(base, nil)
		assert.Equal(t, "localhost", result.Server.Host)
		assert.Equal(t, 1000, result.Server.Port)
	})

	t.Run("merge preserves slices and maps", func(t *testing.T) {
		base := &entities.Config{
			Plugins: entities.PluginsConfig{
				Whitelist: []string{"plugin1", "plugin2"},
			},
			Metadata: entities.Metadata{
				DefaultTags: []string{"tag1", "tag2"},
				Custom: map[string]string{
					"key1": "value1",
				},
			},
		}

		override := &entities.Config{
			Plugins: entities.PluginsConfig{
				Blacklist: []string{"badplugin"},
			},
			Metadata: entities.Metadata{
				Custom: map[string]string{
					"key2": "value2",
				},
			},
		}

		result := merger.Merge(base, override)
		assert.Equal(t, []string{"plugin1", "plugin2"}, result.Plugins.Whitelist)
		assert.Equal(t, []string{"badplugin"}, result.Plugins.Blacklist)
		assert.Contains(t, result.Metadata.Custom, "key1")
		assert.Contains(t, result.Metadata.Custom, "key2")
		assert.Equal(t, "value1", result.Metadata.Custom["key1"])
		assert.Equal(t, "value2", result.Metadata.Custom["key2"])
	})
}

func TestConfigMerger_ApplyFlags(t *testing.T) {
	merger := NewConfigMerger()

	t.Run("apply CLI flag overrides", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
			Theme: entities.ThemeConfig{
				Name: "default",
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true,
			},
		}

		flags := map[string]interface{}{
			"port":       8080,
			"host":       "0.0.0.0",
			"theme":      "professional",
			"no-browser": true,
			"theme-path": "/custom/theme/path",
		}

		result := merger.ApplyFlags(config, flags)
		assert.Equal(t, "0.0.0.0", result.Server.Host)
		assert.Equal(t, 8080, result.Server.Port)
		assert.Equal(t, "professional", result.Theme.Name)
		assert.Equal(t, "/custom/theme/path", result.Theme.CustomPath)
		assert.False(t, result.Browser.AutoOpen) // no-browser = true means AutoOpen = false
	})

	t.Run("ignore invalid flag values", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
		}

		flags := map[string]interface{}{
			"port":  0,  // Should be ignored
			"host":  "", // Should be ignored
			"theme": "", // Should be ignored
		}

		result := merger.ApplyFlags(config, flags)
		assert.Equal(t, "localhost", result.Server.Host) // Unchanged
		assert.Equal(t, 1000, result.Server.Port)        // Unchanged
	})

	t.Run("handle missing flags", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
		}

		flags := map[string]interface{}{
			"other-flag": "value",
		}

		result := merger.ApplyFlags(config, flags)
		assert.Equal(t, "localhost", result.Server.Host) // Unchanged
		assert.Equal(t, 1000, result.Server.Port)        // Unchanged
	})

	t.Run("handle wrong type flags", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Port: 1000,
			},
		}

		flags := map[string]interface{}{
			"port": "not-a-number", // Wrong type
		}

		result := merger.ApplyFlags(config, flags)
		assert.Equal(t, 1000, result.Server.Port) // Unchanged
	})
}

func TestConfigMerger_ApplyEnvVars(t *testing.T) {
	merger := NewConfigMerger()

	t.Run("apply environment variable overrides", func(t *testing.T) {
		// Set environment variables
		_ = os.Setenv("SLICLI_HOST", "env-host")
		_ = os.Setenv("SLICLI_PORT", "9000")
		_ = os.Setenv("SLICLI_THEME", "env-theme")
		_ = os.Setenv("SLICLI_NO_BROWSER", "true")
		_ = os.Setenv("SLICLI_WATCH_INTERVAL", "300")
		_ = os.Setenv("SLICLI_AUTHOR", "Test Author")
		defer func() {
			_ = os.Unsetenv("SLICLI_HOST")
			_ = os.Unsetenv("SLICLI_PORT")
			_ = os.Unsetenv("SLICLI_THEME")
			_ = os.Unsetenv("SLICLI_NO_BROWSER")
			_ = os.Unsetenv("SLICLI_WATCH_INTERVAL")
			_ = os.Unsetenv("SLICLI_AUTHOR")
		}()

		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
			Theme: entities.ThemeConfig{
				Name: "default",
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true,
			},
			Watcher: entities.WatcherConfig{
				IntervalMs: 200,
			},
			Metadata: entities.Metadata{
				Author: "Original Author",
			},
		}

		result := merger.ApplyEnvVars(config)
		assert.Equal(t, "env-host", result.Server.Host)
		assert.Equal(t, 9000, result.Server.Port)
		assert.Equal(t, "env-theme", result.Theme.Name)
		assert.False(t, result.Browser.AutoOpen)
		assert.Equal(t, 300, result.Watcher.IntervalMs)
		assert.Equal(t, "Test Author", result.Metadata.Author)
	})

	t.Run("ignore invalid environment values", func(t *testing.T) {
		// Set invalid environment variables
		_ = os.Setenv("SLICLI_PORT", "not-a-number")
		_ = os.Setenv("SLICLI_NO_BROWSER", "not-a-bool")
		_ = os.Setenv("SLICLI_WATCH_INTERVAL", "negative")
		defer func() {
			_ = os.Unsetenv("SLICLI_PORT")
			_ = os.Unsetenv("SLICLI_NO_BROWSER")
			_ = os.Unsetenv("SLICLI_WATCH_INTERVAL")
		}()

		config := &entities.Config{
			Server: entities.ServerConfig{
				Port: 1000,
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true,
			},
			Watcher: entities.WatcherConfig{
				IntervalMs: 200,
			},
		}

		result := merger.ApplyEnvVars(config)
		assert.Equal(t, 1000, result.Server.Port)       // Unchanged
		assert.True(t, result.Browser.AutoOpen)         // Unchanged
		assert.Equal(t, 200, result.Watcher.IntervalMs) // Unchanged
	})

	t.Run("no environment variables set", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
		}

		result := merger.ApplyEnvVars(config)
		assert.Equal(t, "localhost", result.Server.Host) // Unchanged
		assert.Equal(t, 1000, result.Server.Port)        // Unchanged
	})
}

func TestDeepCopy(t *testing.T) {
	t.Run("deep copy preserves all fields", func(t *testing.T) {
		original := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 1000,
			},
			Theme: entities.ThemeConfig{
				Name: "default",
			},
			Plugins: entities.PluginsConfig{
				Whitelist: []string{"plugin1", "plugin2"},
			},
			Metadata: entities.Metadata{
				Custom: map[string]string{
					"key": "value",
				},
			},
		}

		copy := deepCopy(original)
		assert.Equal(t, original.Server.Host, copy.Server.Host)
		assert.Equal(t, original.Server.Port, copy.Server.Port)
		assert.Equal(t, original.Theme.Name, copy.Theme.Name)
		assert.Equal(t, original.Plugins.Whitelist, copy.Plugins.Whitelist)
		assert.Equal(t, original.Metadata.Custom, copy.Metadata.Custom)
	})

	t.Run("deep copy creates independent slices", func(t *testing.T) {
		original := &entities.Config{
			Plugins: entities.PluginsConfig{
				Whitelist: []string{"plugin1"},
			},
		}

		copy := deepCopy(original)

		// Modify original slice
		original.Plugins.Whitelist[0] = "modified"

		// Copy should be unchanged
		assert.Equal(t, "plugin1", copy.Plugins.Whitelist[0])
	})

	t.Run("deep copy creates independent maps", func(t *testing.T) {
		original := &entities.Config{
			Metadata: entities.Metadata{
				Custom: map[string]string{
					"key": "value",
				},
			},
		}

		copy := deepCopy(original)

		// Modify original map
		original.Metadata.Custom["key"] = "modified"

		// Copy should be unchanged
		assert.Equal(t, "value", copy.Metadata.Custom["key"])
	})

	t.Run("deep copy handles nil config", func(t *testing.T) {
		copy := deepCopy(nil)
		assert.Nil(t, copy)
	})

	t.Run("deep copy handles nil slices and maps", func(t *testing.T) {
		original := &entities.Config{
			Plugins: entities.PluginsConfig{
				Whitelist: nil,
			},
			Metadata: entities.Metadata{
				Custom: nil,
			},
		}

		copy := deepCopy(original)
		assert.Nil(t, copy.Plugins.Whitelist)
		assert.Nil(t, copy.Metadata.Custom)
	})
}
