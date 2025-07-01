package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// GetDefaultConfig returns the default configuration with environment overrides
func GetDefaultConfig() *entities.Config {
	config := &entities.Config{
		Server: entities.ServerConfig{
			Host:            getEnvOrDefault("SLICLI_HOST", "localhost"),
			Port:            getEnvIntOrDefault("SLICLI_PORT", 1000),
			ReadTimeout:     getEnvIntOrDefault("SLICLI_READ_TIMEOUT", 30),
			WriteTimeout:    getEnvIntOrDefault("SLICLI_WRITE_TIMEOUT", 30),
			ShutdownTimeout: getEnvIntOrDefault("SLICLI_SHUTDOWN_TIMEOUT", 5),
			CORSOrigins: getEnvSliceOrDefault("SLICLI_CORS_ORIGINS", []string{
				"http://localhost:3000",
				"http://127.0.0.1:3000",
				"http://localhost:8080",
				"http://127.0.0.1:8080",
			}),
		},
		Theme: entities.ThemeConfig{
			Name:       "default",
			CustomPath: "",
		},
		Browser: entities.BrowserConfig{
			AutoOpen: true,
			Browser:  "default",
		},
		Watcher: entities.WatcherConfig{
			IntervalMs:   200,
			DebounceMs:   500,
			MaxRetries:   3,
			RetryDelayMs: 100,
		},
		Plugins: entities.PluginsConfig{
			Enabled:        true,
			Directory:      "",
			Whitelist:      []string{},
			Blacklist:      []string{},
			MarketplaceURL: "https://marketplace.slicli.dev",
		},
		Metadata: entities.Metadata{
			Author:      "",
			Email:       "",
			Company:     "",
			DefaultTags: []string{},
			Custom:      make(map[string]string),
		},
		Logging: entities.LoggingConfig{
			Level:      getEnvOrDefault("SLICLI_LOG_LEVEL", "info"),
			Verbose:    getEnvBoolOrDefault("SLICLI_LOG_VERBOSE", false),
			JSONFormat: getEnvBoolOrDefault("SLICLI_LOG_JSON", false),
			File:       getEnvOrDefault("SLICLI_LOG_FILE", ""),
			MaxSize:    getEnvIntOrDefault("SLICLI_LOG_MAX_SIZE", 100),
			MaxAge:     getEnvIntOrDefault("SLICLI_LOG_MAX_AGE", 7),
			MaxBackups: getEnvIntOrDefault("SLICLI_LOG_MAX_BACKUPS", 5),
		},
	}

	// Apply additional environment-based overrides
	applyEnvironmentOverrides(config)

	return config
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns environment variable as int or default
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault returns environment variable as bool or default
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvSliceOrDefault returns environment variable as slice or default
func getEnvSliceOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim whitespace
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// applyEnvironmentOverrides applies additional environment-based configuration
func applyEnvironmentOverrides(config *entities.Config) {
	// Override plugin settings
	if enabled := os.Getenv("SLICLI_PLUGINS_ENABLED"); enabled != "" {
		if boolValue, err := strconv.ParseBool(enabled); err == nil {
			config.Plugins.Enabled = boolValue
		}
	}

	if dir := os.Getenv("SLICLI_PLUGINS_DIR"); dir != "" {
		config.Plugins.Directory = dir
	}

	// Override theme settings
	if theme := os.Getenv("SLICLI_THEME"); theme != "" {
		config.Theme.Name = theme
	}

	if customPath := os.Getenv("SLICLI_THEME_PATH"); customPath != "" {
		config.Theme.CustomPath = customPath
	}

	// Override browser settings
	if autoOpen := os.Getenv("SLICLI_BROWSER_AUTO_OPEN"); autoOpen != "" {
		if boolValue, err := strconv.ParseBool(autoOpen); err == nil {
			config.Browser.AutoOpen = boolValue
		}
	}

	if browser := os.Getenv("SLICLI_BROWSER"); browser != "" {
		config.Browser.Browser = browser
	}
}
