package entities

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Server   ServerConfig  `toml:"server"`
	Theme    ThemeConfig   `toml:"theme"`
	Browser  BrowserConfig `toml:"browser"`
	Watcher  WatcherConfig `toml:"watcher"`
	Plugins  PluginsConfig `toml:"plugins"`
	Metadata Metadata      `toml:"metadata"`
	Logging  LoggingConfig `toml:"logging"`
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.Theme.Validate(); err != nil {
		return fmt.Errorf("theme config: %w", err)
	}

	if err := c.Browser.Validate(); err != nil {
		return fmt.Errorf("browser config: %w", err)
	}

	if err := c.Watcher.Validate(); err != nil {
		return fmt.Errorf("watcher config: %w", err)
	}

	if err := c.Plugins.Validate(); err != nil {
		return fmt.Errorf("plugins config: %w", err)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	return nil
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host            string   `toml:"host"`
	Port            int      `toml:"port"`
	ReadTimeout     int      `toml:"read_timeout"`
	WriteTimeout    int      `toml:"write_timeout"`
	ShutdownTimeout int      `toml:"shutdown_timeout"`
	Environment     string   `toml:"environment"`
	CORSOrigins     []string `toml:"cors_origins"`
}

// Validate validates server configuration
func (s ServerConfig) Validate() error {
	if s.Port < 0 || s.Port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}

	if s.Host != "" {
		if ip := net.ParseIP(s.Host); ip == nil {
			if _, err := net.LookupHost(s.Host); err != nil {
				return fmt.Errorf("invalid host: %w", err)
			}
		}
	}

	if s.ReadTimeout < 0 {
		return errors.New("read timeout must be non-negative")
	}

	if s.WriteTimeout < 0 {
		return errors.New("write timeout must be non-negative")
	}

	if s.ShutdownTimeout < 0 {
		return errors.New("shutdown timeout must be non-negative")
	}

	// Validate CORS origins
	for _, origin := range s.CORSOrigins {
		if origin == "" {
			return errors.New("CORS origin cannot be empty")
		}
		// Allow wildcard origin for development
		if origin == "*" {
			continue
		}
		// Basic URL validation
		if len(origin) < 7 || (!strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://")) {
			return fmt.Errorf("invalid CORS origin format: %s (must start with http:// or https://)", origin)
		}
	}

	return nil
}

// GetReadTimeout returns the read timeout as a duration
func (s ServerConfig) GetReadTimeout() time.Duration {
	if s.ReadTimeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(s.ReadTimeout) * time.Second
}

// GetWriteTimeout returns the write timeout as a duration
func (s ServerConfig) GetWriteTimeout() time.Duration {
	if s.WriteTimeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(s.WriteTimeout) * time.Second
}

// GetShutdownTimeout returns the shutdown timeout as a duration
func (s ServerConfig) GetShutdownTimeout() time.Duration {
	if s.ShutdownTimeout <= 0 {
		return 5 * time.Second
	}
	return time.Duration(s.ShutdownTimeout) * time.Second
}

// GetCORSOrigins returns CORS origins with defaults if empty
func (s ServerConfig) GetCORSOrigins() []string {
	if len(s.CORSOrigins) == 0 {
		// Default to secure localhost origins for development
		return []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}
	}
	return s.CORSOrigins
}

// IsDevelopment returns true if the server is running in development mode
func (s ServerConfig) IsDevelopment() bool {
	return s.Environment == "development" || s.Environment == ""
}

// ThemeConfig contains theme configuration
type ThemeConfig struct {
	Name       string `toml:"name"`
	CustomPath string `toml:"custom_path"`
}

// Validate validates theme configuration
func (t ThemeConfig) Validate() error {
	if t.Name == "" {
		return errors.New("theme name cannot be empty")
	}

	if t.CustomPath != "" {
		if !filepath.IsAbs(t.CustomPath) {
			return errors.New("custom theme path must be absolute")
		}

		if _, err := os.Stat(t.CustomPath); os.IsNotExist(err) {
			return fmt.Errorf("custom theme path does not exist: %s", t.CustomPath)
		}
	}

	return nil
}

// BrowserConfig contains browser launch configuration
type BrowserConfig struct {
	AutoOpen bool   `toml:"auto_open"`
	Browser  string `toml:"browser"`
}

// Validate validates browser configuration
func (b BrowserConfig) Validate() error {
	// Browser name validation is minimal since it's platform-dependent
	return nil
}

// WatcherConfig contains file watcher configuration
type WatcherConfig struct {
	IntervalMs   int `toml:"interval_ms"`
	DebounceMs   int `toml:"debounce_ms"`
	MaxRetries   int `toml:"max_retries"`
	RetryDelayMs int `toml:"retry_delay_ms"`
}

// Validate validates watcher configuration
func (w WatcherConfig) Validate() error {
	if w.IntervalMs < 50 {
		return errors.New("watcher interval must be at least 50ms")
	}

	if w.DebounceMs < 0 {
		return errors.New("debounce time must be non-negative")
	}

	if w.MaxRetries < 0 {
		return errors.New("max retries must be non-negative")
	}

	if w.RetryDelayMs < 0 {
		return errors.New("retry delay must be non-negative")
	}

	return nil
}

// GetInterval returns the watcher interval as a duration
func (w WatcherConfig) GetInterval() time.Duration {
	if w.IntervalMs <= 0 {
		return 200 * time.Millisecond
	}
	return time.Duration(w.IntervalMs) * time.Millisecond
}

// GetDebounce returns the debounce time as a duration
func (w WatcherConfig) GetDebounce() time.Duration {
	if w.DebounceMs <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(w.DebounceMs) * time.Millisecond
}

// GetRetryDelay returns the retry delay as a duration
func (w WatcherConfig) GetRetryDelay() time.Duration {
	if w.RetryDelayMs <= 0 {
		return 100 * time.Millisecond
	}
	return time.Duration(w.RetryDelayMs) * time.Millisecond
}

// PluginsConfig contains plugin system configuration
type PluginsConfig struct {
	Enabled        bool     `toml:"enabled"`
	Directory      string   `toml:"directory"`
	Whitelist      []string `toml:"whitelist"`
	Blacklist      []string `toml:"blacklist"`
	MarketplaceURL string   `toml:"marketplace_url"`
}

// Validate validates plugins configuration
func (p PluginsConfig) Validate() error {
	if p.Directory != "" {
		if !filepath.IsAbs(p.Directory) {
			return errors.New("plugin directory must be absolute path")
		}
	}

	// Validate marketplace URL if provided
	if p.MarketplaceURL != "" {
		if len(p.MarketplaceURL) < 7 ||
			(!strings.HasPrefix(p.MarketplaceURL, "http://") &&
				!strings.HasPrefix(p.MarketplaceURL, "https://")) {
			return fmt.Errorf("marketplace URL must start with http:// or https://: %s", p.MarketplaceURL)
		}
	}

	return nil
}

// GetMarketplaceURL returns the marketplace URL with environment override
func (p PluginsConfig) GetMarketplaceURL() string {
	// Environment variable takes highest precedence
	if envURL := os.Getenv("SLICLI_MARKETPLACE_URL"); envURL != "" {
		return envURL
	}

	// Use configured URL if available
	if p.MarketplaceURL != "" {
		return p.MarketplaceURL
	}

	// Default fallback
	return "https://marketplace.slicli.dev"
}

// Metadata contains presentation metadata defaults
type Metadata struct {
	Author      string            `toml:"author"`
	Email       string            `toml:"email"`
	Company     string            `toml:"company"`
	DefaultTags []string          `toml:"default_tags"`
	Custom      map[string]string `toml:"custom"`
}

// LogLevel represents logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `toml:"level"`       // debug, info, warn, error
	Verbose    bool   `toml:"verbose"`     // Enable verbose logging
	JSONFormat bool   `toml:"json_format"` // Output logs in JSON format
	File       string `toml:"file"`        // Log to file (optional)
	MaxSize    int    `toml:"max_size"`    // Maximum log file size in MB
	MaxAge     int    `toml:"max_age"`     // Maximum age in days
	MaxBackups int    `toml:"max_backups"` // Maximum number of backup files
}

// Validate validates logging configuration
func (l LoggingConfig) Validate() error {
	// Validate log level
	switch LogLevel(l.Level) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		// Valid levels
	case "":
		// Empty is okay, will use default
	default:
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", l.Level)
	}

	// Validate file settings if file logging is enabled
	if l.File != "" {
		if !filepath.IsAbs(l.File) {
			return errors.New("log file path must be absolute")
		}

		// Check if parent directory exists
		dir := filepath.Dir(l.File)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("log file directory does not exist: %s", dir)
		}

		if l.MaxSize < 0 {
			return errors.New("max log file size must be non-negative")
		}

		if l.MaxAge < 0 {
			return errors.New("max log file age must be non-negative")
		}

		if l.MaxBackups < 0 {
			return errors.New("max log backups must be non-negative")
		}
	}

	return nil
}

// GetLevel returns the log level with default
func (l LoggingConfig) GetLevel() LogLevel {
	if l.Level == "" {
		return LogLevelInfo // Default level
	}
	return LogLevel(l.Level)
}

// GetMaxSize returns the max file size with default (100MB)
func (l LoggingConfig) GetMaxSize() int {
	if l.MaxSize <= 0 {
		return 100
	}
	return l.MaxSize
}

// GetMaxAge returns the max age with default (7 days)
func (l LoggingConfig) GetMaxAge() int {
	if l.MaxAge <= 0 {
		return 7
	}
	return l.MaxAge
}

// GetMaxBackups returns the max backups with default (5)
func (l LoggingConfig) GetMaxBackups() int {
	if l.MaxBackups <= 0 {
		return 5
	}
	return l.MaxBackups
}
