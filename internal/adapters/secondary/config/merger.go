package config

import (
	"os"
	"strconv"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// ConfigMerger implements the ConfigMerger interface
type ConfigMerger struct{}

// NewConfigMerger creates a new configuration merger
func NewConfigMerger() *ConfigMerger {
	return &ConfigMerger{}
}

// Merge merges multiple configurations with later configs taking precedence
func (m *ConfigMerger) Merge(configs ...*entities.Config) *entities.Config {
	if len(configs) == 0 {
		return GetDefaultConfig()
	}

	// Start with first config as base
	result := deepCopy(configs[0])

	// Merge subsequent configs
	for i := 1; i < len(configs); i++ {
		if configs[i] != nil {
			m.mergeInto(result, configs[i])
		}
	}

	return result
}

// ApplyFlags applies CLI flag overrides to a configuration
func (m *ConfigMerger) ApplyFlags(config *entities.Config, flags map[string]interface{}) *entities.Config {
	result := deepCopy(config)

	// Apply CLI flag overrides
	if port, ok := flags["port"].(int); ok && port > 0 {
		result.Server.Port = port
	}

	if host, ok := flags["host"].(string); ok && host != "" {
		result.Server.Host = host
	}

	if theme, ok := flags["theme"].(string); ok && theme != "" {
		result.Theme.Name = theme
	}

	if noBrowser, ok := flags["no-browser"].(bool); ok {
		result.Browser.AutoOpen = !noBrowser
	}

	if customPath, ok := flags["theme-path"].(string); ok && customPath != "" {
		result.Theme.CustomPath = customPath
	}

	return result
}

// ApplyEnvVars applies environment variable overrides to a configuration
func (m *ConfigMerger) ApplyEnvVars(config *entities.Config) *entities.Config {
	result := deepCopy(config)

	// Server configuration from environment
	if host := os.Getenv("SLICLI_HOST"); host != "" {
		result.Server.Host = host
	}

	if portStr := os.Getenv("SLICLI_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			result.Server.Port = port
		}
	}

	// Theme configuration from environment
	if theme := os.Getenv("SLICLI_THEME"); theme != "" {
		result.Theme.Name = theme
	}

	if themePath := os.Getenv("SLICLI_THEME_PATH"); themePath != "" {
		result.Theme.CustomPath = themePath
	}

	// Browser configuration from environment
	if noBrowserStr := os.Getenv("SLICLI_NO_BROWSER"); noBrowserStr != "" {
		if noBrowser, err := strconv.ParseBool(noBrowserStr); err == nil {
			result.Browser.AutoOpen = !noBrowser
		}
	}

	if browser := os.Getenv("SLICLI_BROWSER"); browser != "" {
		result.Browser.Browser = browser
	}

	// Watcher configuration from environment
	if intervalStr := os.Getenv("SLICLI_WATCH_INTERVAL"); intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
			result.Watcher.IntervalMs = interval
		}
	}

	if debounceStr := os.Getenv("SLICLI_WATCH_DEBOUNCE"); debounceStr != "" {
		if debounce, err := strconv.Atoi(debounceStr); err == nil && debounce >= 0 {
			result.Watcher.DebounceMs = debounce
		}
	}

	// Plugins configuration from environment
	if enabledStr := os.Getenv("SLICLI_PLUGINS_ENABLED"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			result.Plugins.Enabled = enabled
		}
	}

	if pluginDir := os.Getenv("SLICLI_PLUGINS_DIR"); pluginDir != "" {
		result.Plugins.Directory = pluginDir
	}

	// Metadata from environment
	if author := os.Getenv("SLICLI_AUTHOR"); author != "" {
		result.Metadata.Author = author
	}

	if email := os.Getenv("SLICLI_EMAIL"); email != "" {
		result.Metadata.Email = email
	}

	if company := os.Getenv("SLICLI_COMPANY"); company != "" {
		result.Metadata.Company = company
	}

	return result
}

// mergeInto merges source configuration into target configuration
func (m *ConfigMerger) mergeInto(target, source *entities.Config) {
	// Server config
	if source.Server.Port != 0 {
		target.Server.Port = source.Server.Port
	}
	if source.Server.Host != "" {
		target.Server.Host = source.Server.Host
	}
	if source.Server.ReadTimeout != 0 {
		target.Server.ReadTimeout = source.Server.ReadTimeout
	}
	if source.Server.WriteTimeout != 0 {
		target.Server.WriteTimeout = source.Server.WriteTimeout
	}
	if source.Server.ShutdownTimeout != 0 {
		target.Server.ShutdownTimeout = source.Server.ShutdownTimeout
	}
	if source.Server.Environment != "" {
		target.Server.Environment = source.Server.Environment
	}
	if len(source.Server.CORSOrigins) > 0 {
		target.Server.CORSOrigins = make([]string, len(source.Server.CORSOrigins))
		copy(target.Server.CORSOrigins, source.Server.CORSOrigins)
	}

	// Theme config
	if source.Theme.Name != "" {
		target.Theme.Name = source.Theme.Name
	}
	if source.Theme.CustomPath != "" {
		target.Theme.CustomPath = source.Theme.CustomPath
	}

	// Browser config
	if source.Browser.Browser != "" {
		target.Browser.Browser = source.Browser.Browser
	}
	// KNOWN LIMITATION: Boolean Merge Issue
	//
	// In Go, we cannot distinguish between a boolean that was explicitly set to false
	// versus one that was not set at all (both have zero value = false).
	//
	// Current behavior: source.AutoOpen always overwrites target.AutoOpen.
	// This means if source config doesn't specify AutoOpen, it will set target to false.
	//
	// Workarounds for users:
	// 1. Always explicitly set AutoOpen in config files when using multiple configs
	// 2. Use environment variable SLICLI_NO_BROWSER=true/false which takes precedence
	// 3. Use CLI flag --no-browser which has highest precedence
	//
	// Future improvement: Consider using *bool (pointer) for optional boolean fields
	// in the config struct to properly distinguish unset from explicitly false.
	target.Browser.AutoOpen = source.Browser.AutoOpen

	// Watcher config
	if source.Watcher.IntervalMs != 0 {
		target.Watcher.IntervalMs = source.Watcher.IntervalMs
	}
	if source.Watcher.DebounceMs != 0 {
		target.Watcher.DebounceMs = source.Watcher.DebounceMs
	}
	if source.Watcher.MaxRetries != 0 {
		target.Watcher.MaxRetries = source.Watcher.MaxRetries
	}
	if source.Watcher.RetryDelayMs != 0 {
		target.Watcher.RetryDelayMs = source.Watcher.RetryDelayMs
	}

	// Plugins config
	// KNOWN LIMITATION: Same boolean merge issue as Browser.AutoOpen above.
	// See comments there for workarounds and future improvements.
	target.Plugins.Enabled = source.Plugins.Enabled
	if source.Plugins.Directory != "" {
		target.Plugins.Directory = source.Plugins.Directory
	}
	if source.Plugins.MarketplaceURL != "" {
		target.Plugins.MarketplaceURL = source.Plugins.MarketplaceURL
	}
	if len(source.Plugins.Whitelist) > 0 {
		target.Plugins.Whitelist = make([]string, len(source.Plugins.Whitelist))
		copy(target.Plugins.Whitelist, source.Plugins.Whitelist)
	}
	if len(source.Plugins.Blacklist) > 0 {
		target.Plugins.Blacklist = make([]string, len(source.Plugins.Blacklist))
		copy(target.Plugins.Blacklist, source.Plugins.Blacklist)
	}

	// Metadata config
	if source.Metadata.Author != "" {
		target.Metadata.Author = source.Metadata.Author
	}
	if source.Metadata.Email != "" {
		target.Metadata.Email = source.Metadata.Email
	}
	if source.Metadata.Company != "" {
		target.Metadata.Company = source.Metadata.Company
	}
	if len(source.Metadata.DefaultTags) > 0 {
		target.Metadata.DefaultTags = make([]string, len(source.Metadata.DefaultTags))
		copy(target.Metadata.DefaultTags, source.Metadata.DefaultTags)
	}
	if len(source.Metadata.Custom) > 0 {
		if target.Metadata.Custom == nil {
			target.Metadata.Custom = make(map[string]string)
		}
		for k, v := range source.Metadata.Custom {
			target.Metadata.Custom[k] = v
		}
	}
}

// deepCopy creates a deep copy of a configuration
func deepCopy(src *entities.Config) *entities.Config {
	if src == nil {
		return nil
	}

	// Manual copy to avoid reflection for performance
	dst := &entities.Config{
		Server: entities.ServerConfig{
			Host:            src.Server.Host,
			Port:            src.Server.Port,
			ReadTimeout:     src.Server.ReadTimeout,
			WriteTimeout:    src.Server.WriteTimeout,
			ShutdownTimeout: src.Server.ShutdownTimeout,
			Environment:     src.Server.Environment,
		},
		Theme: entities.ThemeConfig{
			Name:       src.Theme.Name,
			CustomPath: src.Theme.CustomPath,
		},
		Browser: entities.BrowserConfig{
			AutoOpen: src.Browser.AutoOpen,
			Browser:  src.Browser.Browser,
		},
		Watcher: entities.WatcherConfig{
			IntervalMs:   src.Watcher.IntervalMs,
			DebounceMs:   src.Watcher.DebounceMs,
			MaxRetries:   src.Watcher.MaxRetries,
			RetryDelayMs: src.Watcher.RetryDelayMs,
		},
		Plugins: entities.PluginsConfig{
			Enabled:        src.Plugins.Enabled,
			Directory:      src.Plugins.Directory,
			MarketplaceURL: src.Plugins.MarketplaceURL,
		},
		Metadata: entities.Metadata{
			Author:  src.Metadata.Author,
			Email:   src.Metadata.Email,
			Company: src.Metadata.Company,
		},
		Logging: entities.LoggingConfig{
			Level:      src.Logging.Level,
			Verbose:    src.Logging.Verbose,
			JSONFormat: src.Logging.JSONFormat,
			File:       src.Logging.File,
			MaxSize:    src.Logging.MaxSize,
			MaxAge:     src.Logging.MaxAge,
			MaxBackups: src.Logging.MaxBackups,
		},
	}

	// Copy CORSOrigins slice
	if src.Server.CORSOrigins != nil {
		dst.Server.CORSOrigins = make([]string, len(src.Server.CORSOrigins))
		copy(dst.Server.CORSOrigins, src.Server.CORSOrigins)
	}

	// Copy Plugins slices
	if src.Plugins.Whitelist != nil {
		dst.Plugins.Whitelist = make([]string, len(src.Plugins.Whitelist))
		copy(dst.Plugins.Whitelist, src.Plugins.Whitelist)
	}

	if src.Plugins.Blacklist != nil {
		dst.Plugins.Blacklist = make([]string, len(src.Plugins.Blacklist))
		copy(dst.Plugins.Blacklist, src.Plugins.Blacklist)
	}

	// Copy Metadata slices and maps
	if src.Metadata.DefaultTags != nil {
		dst.Metadata.DefaultTags = make([]string, len(src.Metadata.DefaultTags))
		copy(dst.Metadata.DefaultTags, src.Metadata.DefaultTags)
	}

	if src.Metadata.Custom != nil {
		dst.Metadata.Custom = make(map[string]string)
		for k, v := range src.Metadata.Custom {
			dst.Metadata.Custom[k] = v
		}
	}

	return dst
}

// Ensure ConfigMerger implements ports.ConfigMerger
var _ ports.ConfigMerger = (*ConfigMerger)(nil)
