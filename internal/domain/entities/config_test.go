package entities

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{
				Host:            "localhost",
				Port:            3000,
				ReadTimeout:     30,
				WriteTimeout:    30,
				ShutdownTimeout: 5,
			},
			Theme: ThemeConfig{
				Name: "default",
			},
			Browser: BrowserConfig{
				AutoOpen: true,
				Browser:  "default",
			},
			Watcher: WatcherConfig{
				IntervalMs:   200,
				DebounceMs:   500,
				MaxRetries:   3,
				RetryDelayMs: 100,
			},
			Plugins: PluginsConfig{
				Enabled:   true,
				Directory: "",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid server config", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{
				Port: -1, // Invalid port
			},
			Theme: ThemeConfig{
				Name: "default",
			},
			Browser: BrowserConfig{},
			Watcher: WatcherConfig{
				IntervalMs: 200,
			},
			Plugins: PluginsConfig{},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server config")
	})

	t.Run("invalid theme config", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{
				Port: 3000,
			},
			Theme: ThemeConfig{
				Name: "", // Invalid empty name
			},
			Browser: BrowserConfig{},
			Watcher: WatcherConfig{
				IntervalMs: 200,
			},
			Plugins: PluginsConfig{},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "theme config")
	})
}

func TestServerConfig_Validate(t *testing.T) {
	t.Run("valid server config", func(t *testing.T) {
		config := ServerConfig{
			Host:            "localhost",
			Port:            3000,
			ReadTimeout:     30,
			WriteTimeout:    30,
			ShutdownTimeout: 5,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid port - negative", func(t *testing.T) {
		config := ServerConfig{
			Port: -1,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port must be between 0 and 65535")
	})

	t.Run("invalid port - too high", func(t *testing.T) {
		config := ServerConfig{
			Port: 70000,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port must be between 0 and 65535")
	})

	t.Run("valid port range", func(t *testing.T) {
		validPorts := []int{0, 1, 3000, 8080, 65535}
		for _, port := range validPorts {
			config := ServerConfig{Port: port}
			err := config.Validate()
			assert.NoError(t, err, "Port %d should be valid", port)
		}
	})

	t.Run("negative timeouts", func(t *testing.T) {
		tests := []struct {
			name   string
			config ServerConfig
		}{
			{
				name: "negative read timeout",
				config: ServerConfig{
					Port:        3000,
					ReadTimeout: -1,
				},
			},
			{
				name: "negative write timeout",
				config: ServerConfig{
					Port:         3000,
					WriteTimeout: -1,
				},
			},
			{
				name: "negative shutdown timeout",
				config: ServerConfig{
					Port:            3000,
					ShutdownTimeout: -1,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				assert.Error(t, err)
			})
		}
	})
}

func TestServerConfig_GetTimeouts(t *testing.T) {
	t.Run("custom timeouts", func(t *testing.T) {
		config := ServerConfig{
			ReadTimeout:     45,
			WriteTimeout:    60,
			ShutdownTimeout: 10,
		}

		assert.Equal(t, 45*time.Second, config.GetReadTimeout())
		assert.Equal(t, 60*time.Second, config.GetWriteTimeout())
		assert.Equal(t, 10*time.Second, config.GetShutdownTimeout())
	})

	t.Run("default timeouts", func(t *testing.T) {
		config := ServerConfig{
			ReadTimeout:     0,
			WriteTimeout:    0,
			ShutdownTimeout: 0,
		}

		assert.Equal(t, 30*time.Second, config.GetReadTimeout())
		assert.Equal(t, 30*time.Second, config.GetWriteTimeout())
		assert.Equal(t, 5*time.Second, config.GetShutdownTimeout())
	})

	t.Run("negative timeouts use defaults", func(t *testing.T) {
		config := ServerConfig{
			ReadTimeout:     -5,
			WriteTimeout:    -10,
			ShutdownTimeout: -2,
		}

		assert.Equal(t, 30*time.Second, config.GetReadTimeout())
		assert.Equal(t, 30*time.Second, config.GetWriteTimeout())
		assert.Equal(t, 5*time.Second, config.GetShutdownTimeout())
	})
}

func TestThemeConfig_Validate(t *testing.T) {
	t.Run("valid theme config", func(t *testing.T) {
		config := ThemeConfig{
			Name: "default",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty theme name", func(t *testing.T) {
		config := ThemeConfig{
			Name: "",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "theme name cannot be empty")
	})

	t.Run("invalid custom path - relative", func(t *testing.T) {
		config := ThemeConfig{
			Name:       "custom",
			CustomPath: "relative/path",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom theme path must be absolute")
	})

	t.Run("invalid custom path - non-existent", func(t *testing.T) {
		config := ThemeConfig{
			Name:       "custom",
			CustomPath: "/non/existent/path",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom theme path does not exist")
	})
}

func TestBrowserConfig_Validate(t *testing.T) {
	t.Run("valid browser config", func(t *testing.T) {
		config := BrowserConfig{
			AutoOpen: true,
			Browser:  "chrome",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty browser name", func(t *testing.T) {
		config := BrowserConfig{
			AutoOpen: false,
			Browser:  "",
		}

		err := config.Validate()
		assert.NoError(t, err) // Browser validation is minimal
	})
}

func TestWatcherConfig_Validate(t *testing.T) {
	t.Run("valid watcher config", func(t *testing.T) {
		config := WatcherConfig{
			IntervalMs:   200,
			DebounceMs:   500,
			MaxRetries:   3,
			RetryDelayMs: 100,
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid interval - too low", func(t *testing.T) {
		config := WatcherConfig{
			IntervalMs: 25, // Below minimum of 50ms
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "watcher interval must be at least 50ms")
	})

	t.Run("negative values", func(t *testing.T) {
		tests := []struct {
			name   string
			config WatcherConfig
		}{
			{
				name: "negative debounce",
				config: WatcherConfig{
					IntervalMs: 200,
					DebounceMs: -1,
				},
			},
			{
				name: "negative max retries",
				config: WatcherConfig{
					IntervalMs: 200,
					MaxRetries: -1,
				},
			},
			{
				name: "negative retry delay",
				config: WatcherConfig{
					IntervalMs:   200,
					RetryDelayMs: -1,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				assert.Error(t, err)
			})
		}
	})
}

func TestWatcherConfig_GetDurations(t *testing.T) {
	t.Run("custom durations", func(t *testing.T) {
		config := WatcherConfig{
			IntervalMs:   300,
			DebounceMs:   750,
			RetryDelayMs: 150,
		}

		assert.Equal(t, 300*time.Millisecond, config.GetInterval())
		assert.Equal(t, 750*time.Millisecond, config.GetDebounce())
		assert.Equal(t, 150*time.Millisecond, config.GetRetryDelay())
	})

	t.Run("default durations", func(t *testing.T) {
		config := WatcherConfig{
			IntervalMs:   0,
			DebounceMs:   0,
			RetryDelayMs: 0,
		}

		assert.Equal(t, 200*time.Millisecond, config.GetInterval())
		assert.Equal(t, 500*time.Millisecond, config.GetDebounce())
		assert.Equal(t, 100*time.Millisecond, config.GetRetryDelay())
	})

	t.Run("negative durations use defaults", func(t *testing.T) {
		config := WatcherConfig{
			IntervalMs:   -100,
			DebounceMs:   -200,
			RetryDelayMs: -50,
		}

		assert.Equal(t, 200*time.Millisecond, config.GetInterval())
		assert.Equal(t, 500*time.Millisecond, config.GetDebounce())
		assert.Equal(t, 100*time.Millisecond, config.GetRetryDelay())
	})
}

func TestPluginsConfig_Validate(t *testing.T) {
	t.Run("valid plugins config", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:   true,
			Directory: "/absolute/path/to/plugins",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty directory is valid", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:   true,
			Directory: "",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid directory - relative path", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:   true,
			Directory: "relative/path",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "plugin directory must be absolute path")
	})

	t.Run("valid marketplace URL - https", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:        true,
			MarketplaceURL: "https://marketplace.example.com",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("valid marketplace URL - http", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:        true,
			MarketplaceURL: "http://localhost:3000",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty marketplace URL is valid", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:        true,
			MarketplaceURL: "",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid marketplace URL - no protocol", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:        true,
			MarketplaceURL: "marketplace.example.com",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marketplace URL must start with http:// or https://")
	})

	t.Run("invalid marketplace URL - wrong protocol", func(t *testing.T) {
		config := PluginsConfig{
			Enabled:        true,
			MarketplaceURL: "ftp://marketplace.example.com",
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marketplace URL must start with http:// or https://")
	})
}

func TestPluginsConfig_GetMarketplaceURL(t *testing.T) {
	t.Run("returns configured URL", func(t *testing.T) {
		config := PluginsConfig{
			MarketplaceURL: "https://custom.marketplace.com",
		}

		url := config.GetMarketplaceURL()
		assert.Equal(t, "https://custom.marketplace.com", url)
	})

	t.Run("returns default when empty", func(t *testing.T) {
		config := PluginsConfig{
			MarketplaceURL: "",
		}

		url := config.GetMarketplaceURL()
		assert.Equal(t, "https://marketplace.slicli.dev", url)
	})

	t.Run("environment variable overrides config", func(t *testing.T) {
		// Set environment variable
		originalEnv := os.Getenv("SLICLI_MARKETPLACE_URL")
		require.NoError(t, os.Setenv("SLICLI_MARKETPLACE_URL", "https://env.marketplace.com"))
		defer func() {
			if originalEnv == "" {
				_ = os.Unsetenv("SLICLI_MARKETPLACE_URL")
			} else {
				_ = os.Setenv("SLICLI_MARKETPLACE_URL", originalEnv)
			}
		}()

		config := PluginsConfig{
			MarketplaceURL: "https://config.marketplace.com",
		}

		url := config.GetMarketplaceURL()
		assert.Equal(t, "https://env.marketplace.com", url)
	})
}
