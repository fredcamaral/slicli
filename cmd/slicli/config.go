package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/config"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/spf13/cobra"
)

// ConfigSource represents the source of a configuration value
type ConfigSource string

const (
	SourceDefault ConfigSource = "default"
	SourceGlobal  ConfigSource = "global"
	SourceLocal   ConfigSource = "local"
	SourceEnv     ConfigSource = "env"
	SourceFlag    ConfigSource = "flag"
)

// ConfigValueInfo holds a configuration value with its source
type ConfigValueInfo struct {
	Value  interface{}  `json:"value"`
	Source ConfigSource `json:"source"`
	Path   string       `json:"path,omitempty"`
}

// TrackedConfig holds configuration with source tracking
type TrackedConfig struct {
	Server   TrackedServerConfig   `json:"server"`
	Theme    TrackedThemeConfig    `json:"theme"`
	Browser  TrackedBrowserConfig  `json:"browser"`
	Watcher  TrackedWatcherConfig  `json:"watcher"`
	Plugins  TrackedPluginsConfig  `json:"plugins"`
	Metadata TrackedMetadataConfig `json:"metadata"`
	Logging  TrackedLoggingConfig  `json:"logging"`
	Sources  []LoadedSource        `json:"sources_loaded"`
}

// LoadedSource represents a config file that was loaded
type LoadedSource struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// TrackedServerConfig holds server config with sources
type TrackedServerConfig struct {
	Host            ConfigValueInfo `json:"host"`
	Port            ConfigValueInfo `json:"port"`
	ReadTimeout     ConfigValueInfo `json:"read_timeout"`
	WriteTimeout    ConfigValueInfo `json:"write_timeout"`
	ShutdownTimeout ConfigValueInfo `json:"shutdown_timeout"`
	Environment     ConfigValueInfo `json:"environment"`
	CORSOrigins     ConfigValueInfo `json:"cors_origins"`
}

// TrackedThemeConfig holds theme config with sources
type TrackedThemeConfig struct {
	Name       ConfigValueInfo `json:"name"`
	CustomPath ConfigValueInfo `json:"custom_path"`
}

// TrackedBrowserConfig holds browser config with sources
type TrackedBrowserConfig struct {
	AutoOpen ConfigValueInfo `json:"auto_open"`
	Browser  ConfigValueInfo `json:"browser"`
}

// TrackedWatcherConfig holds watcher config with sources
type TrackedWatcherConfig struct {
	IntervalMs   ConfigValueInfo `json:"interval_ms"`
	DebounceMs   ConfigValueInfo `json:"debounce_ms"`
	MaxRetries   ConfigValueInfo `json:"max_retries"`
	RetryDelayMs ConfigValueInfo `json:"retry_delay_ms"`
}

// TrackedPluginsConfig holds plugins config with sources
type TrackedPluginsConfig struct {
	Enabled        ConfigValueInfo `json:"enabled"`
	Directory      ConfigValueInfo `json:"directory"`
	Whitelist      ConfigValueInfo `json:"whitelist"`
	Blacklist      ConfigValueInfo `json:"blacklist"`
	MarketplaceURL ConfigValueInfo `json:"marketplace_url"`
}

// TrackedMetadataConfig holds metadata config with sources
type TrackedMetadataConfig struct {
	Author      ConfigValueInfo `json:"author"`
	Email       ConfigValueInfo `json:"email"`
	Company     ConfigValueInfo `json:"company"`
	DefaultTags ConfigValueInfo `json:"default_tags"`
}

// TrackedLoggingConfig holds logging config with sources
type TrackedLoggingConfig struct {
	Level      ConfigValueInfo `json:"level"`
	Verbose    ConfigValueInfo `json:"verbose"`
	JSONFormat ConfigValueInfo `json:"json_format"`
	File       ConfigValueInfo `json:"file"`
	MaxSize    ConfigValueInfo `json:"max_size"`
	MaxAge     ConfigValueInfo `json:"max_age"`
	MaxBackups ConfigValueInfo `json:"max_backups"`
}

// Command flags
var (
	configJSON     bool
	configPath     string
	configGlobal   bool
	configTemplate string
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and manage configuration",
	Long: `The config command allows you to inspect and manage slicli configuration.

Configuration is loaded from multiple sources with precedence:
  CLI flags > environment variables > local config > global config > defaults

Use subcommands to show, initialize, or validate configuration.`,
}

// configShowCmd shows the active configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the active configuration",
	Long: `Display all configuration values and their sources.

The output shows each configuration value along with where it came from:
  - default: Built-in default value
  - global: From ~/.config/slicli/config.toml
  - local: From ./slicli.toml in the presentation directory
  - env: From environment variables`,
	RunE: runConfigShow,
}

// configInitCmd creates a new configuration file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new configuration file",
	Long: `Create a new slicli configuration file.

By default, creates slicli.toml in the current directory.
Use --global to create in ~/.config/slicli/config.toml instead.

Templates:
  - minimal: Essential options only (default)
  - full: All available options with comments`,
	RunE: runConfigInit,
}

// configValidateCmd validates configuration files
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	Long: `Validate configuration files for syntax and value errors.

Checks:
  - TOML syntax is valid
  - Values are within acceptable ranges
  - Paths exist (if specified)
  - Required fields are present`,
	RunE: runConfigValidate,
}

func init() {
	// Add config subcommands
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)

	// Flags for show command
	configShowCmd.Flags().BoolVar(&configJSON, "json", false, "Output in JSON format")
	configShowCmd.Flags().StringVar(&configPath, "path", "", "Show config for a specific presentation directory")

	// Flags for init command
	configInitCmd.Flags().BoolVar(&configGlobal, "global", false, "Create global config in ~/.config/slicli/")
	configInitCmd.Flags().StringVar(&configTemplate, "template", "minimal", "Template to use: minimal, full")

	// Add to root command
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Determine the directory to use for local config
	localDir := "."
	if configPath != "" {
		info, err := os.Stat(configPath)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
		if info.IsDir() {
			localDir = configPath
		} else {
			localDir = filepath.Dir(configPath)
		}
	}

	// Load configuration with source tracking
	tracked, err := loadTrackedConfig(localDir)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	if configJSON {
		return printConfigJSON(tracked)
	}

	return printConfigFormatted(tracked)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	var targetPath string

	if configGlobal {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		targetPath = filepath.Join(homeDir, ".config", "slicli", "config.toml")
	} else {
		targetPath = "slicli.toml"
	}

	// Check if file already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("configuration file already exists: %s", targetPath)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Generate config content based on template
	var content string
	switch configTemplate {
	case "full":
		content = getFullConfigTemplate()
	case "minimal":
		content = getMinimalConfigTemplate()
	default:
		return fmt.Errorf("unknown template: %s (use 'minimal' or 'full')", configTemplate)
	}

	// Write the file
	if err := os.WriteFile(targetPath, []byte(content), 0640); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Created configuration file: %s\n", targetPath)
	if configGlobal {
		fmt.Println("This global config will apply to all presentations.")
	} else {
		fmt.Println("This local config will apply to presentations in this directory.")
	}

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	loader := config.NewTOMLLoader()
	ctx := context.Background()

	var hasErrors bool
	var validatedFiles int

	// Validate global config
	globalPath := loader.GetGlobalPath()
	if _, err := os.Stat(globalPath); err == nil {
		validatedFiles++
		fmt.Printf("Validating global config: %s\n", globalPath)
		if err := validateConfigFile(ctx, globalPath); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			hasErrors = true
		} else {
			fmt.Println("  OK")
		}
	}

	// Validate local config
	localPath := loader.GetLocalPath(".")
	if _, err := os.Stat(localPath); err == nil {
		validatedFiles++
		fmt.Printf("Validating local config: %s\n", localPath)
		if err := validateConfigFile(ctx, localPath); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			hasErrors = true
		} else {
			fmt.Println("  OK")
		}
	}

	// Also validate path-specific config if provided
	if configPath != "" {
		specificPath := filepath.Join(configPath, "slicli.toml")
		if _, err := os.Stat(specificPath); err == nil && specificPath != localPath {
			validatedFiles++
			fmt.Printf("Validating config: %s\n", specificPath)
			if err := validateConfigFile(ctx, specificPath); err != nil {
				fmt.Printf("  ERROR: %v\n", err)
				hasErrors = true
			} else {
				fmt.Println("  OK")
			}
		}
	}

	if validatedFiles == 0 {
		fmt.Println("No configuration files found to validate.")
		fmt.Println("\nExpected locations:")
		fmt.Printf("  Global: %s\n", globalPath)
		fmt.Printf("  Local:  %s\n", localPath)
		return nil
	}

	fmt.Printf("\nValidated %d file(s)\n", validatedFiles)

	if hasErrors {
		return fmt.Errorf("validation failed with errors")
	}

	return nil
}

// validateConfigFile validates a single configuration file
func validateConfigFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path) // #nosec G304 - path from trusted source
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var cfg entities.Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("TOML syntax error: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	return nil
}

// loadTrackedConfig loads configuration while tracking sources
func loadTrackedConfig(localDir string) (*TrackedConfig, error) {
	loader := config.NewTOMLLoader()
	ctx := context.Background()

	// Initialize with defaults
	defaults := config.GetDefaultConfig()
	tracked := initTrackedFromConfig(defaults, SourceDefault, "")

	// Track loaded sources
	var sources []LoadedSource

	// Load global config
	globalPath := loader.GetGlobalPath()
	if _, err := os.Stat(globalPath); err == nil {
		globalConfig, err := loader.LoadGlobal(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading global config: %w", err)
		}
		if globalConfig != nil {
			sources = append(sources, LoadedSource{Path: globalPath, Type: "global"})
			updateTrackedFromConfig(tracked, globalConfig, SourceGlobal, globalPath)
		}
	}

	// Load local config
	localPath := loader.GetLocalPath(localDir)
	if _, err := os.Stat(localPath); err == nil {
		localConfig, err := loader.LoadLocal(ctx, localDir)
		if err != nil {
			return nil, fmt.Errorf("loading local config: %w", err)
		}
		if localConfig != nil {
			sources = append(sources, LoadedSource{Path: localPath, Type: "local"})
			updateTrackedFromConfig(tracked, localConfig, SourceLocal, localPath)
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(tracked)

	tracked.Sources = sources
	return tracked, nil
}

// initTrackedFromConfig initializes tracked config from a config struct
func initTrackedFromConfig(cfg *entities.Config, source ConfigSource, path string) *TrackedConfig {
	return &TrackedConfig{
		Server: TrackedServerConfig{
			Host:            ConfigValueInfo{Value: cfg.Server.Host, Source: source, Path: path},
			Port:            ConfigValueInfo{Value: cfg.Server.Port, Source: source, Path: path},
			ReadTimeout:     ConfigValueInfo{Value: formatDuration(cfg.Server.ReadTimeout), Source: source, Path: path},
			WriteTimeout:    ConfigValueInfo{Value: formatDuration(cfg.Server.WriteTimeout), Source: source, Path: path},
			ShutdownTimeout: ConfigValueInfo{Value: formatDuration(cfg.Server.ShutdownTimeout), Source: source, Path: path},
			Environment:     ConfigValueInfo{Value: cfg.Server.Environment, Source: source, Path: path},
			CORSOrigins:     ConfigValueInfo{Value: cfg.Server.CORSOrigins, Source: source, Path: path},
		},
		Theme: TrackedThemeConfig{
			Name:       ConfigValueInfo{Value: cfg.Theme.Name, Source: source, Path: path},
			CustomPath: ConfigValueInfo{Value: cfg.Theme.CustomPath, Source: source, Path: path},
		},
		Browser: TrackedBrowserConfig{
			AutoOpen: ConfigValueInfo{Value: cfg.Browser.AutoOpen, Source: source, Path: path},
			Browser:  ConfigValueInfo{Value: cfg.Browser.Browser, Source: source, Path: path},
		},
		Watcher: TrackedWatcherConfig{
			IntervalMs:   ConfigValueInfo{Value: cfg.Watcher.IntervalMs, Source: source, Path: path},
			DebounceMs:   ConfigValueInfo{Value: cfg.Watcher.DebounceMs, Source: source, Path: path},
			MaxRetries:   ConfigValueInfo{Value: cfg.Watcher.MaxRetries, Source: source, Path: path},
			RetryDelayMs: ConfigValueInfo{Value: cfg.Watcher.RetryDelayMs, Source: source, Path: path},
		},
		Plugins: TrackedPluginsConfig{
			Enabled:        ConfigValueInfo{Value: cfg.Plugins.Enabled, Source: source, Path: path},
			Directory:      ConfigValueInfo{Value: cfg.Plugins.Directory, Source: source, Path: path},
			Whitelist:      ConfigValueInfo{Value: cfg.Plugins.Whitelist, Source: source, Path: path},
			Blacklist:      ConfigValueInfo{Value: cfg.Plugins.Blacklist, Source: source, Path: path},
			MarketplaceURL: ConfigValueInfo{Value: cfg.Plugins.MarketplaceURL, Source: source, Path: path},
		},
		Metadata: TrackedMetadataConfig{
			Author:      ConfigValueInfo{Value: cfg.Metadata.Author, Source: source, Path: path},
			Email:       ConfigValueInfo{Value: cfg.Metadata.Email, Source: source, Path: path},
			Company:     ConfigValueInfo{Value: cfg.Metadata.Company, Source: source, Path: path},
			DefaultTags: ConfigValueInfo{Value: cfg.Metadata.DefaultTags, Source: source, Path: path},
		},
		Logging: TrackedLoggingConfig{
			Level:      ConfigValueInfo{Value: cfg.Logging.Level, Source: source, Path: path},
			Verbose:    ConfigValueInfo{Value: cfg.Logging.Verbose, Source: source, Path: path},
			JSONFormat: ConfigValueInfo{Value: cfg.Logging.JSONFormat, Source: source, Path: path},
			File:       ConfigValueInfo{Value: cfg.Logging.File, Source: source, Path: path},
			MaxSize:    ConfigValueInfo{Value: cfg.Logging.MaxSize, Source: source, Path: path},
			MaxAge:     ConfigValueInfo{Value: cfg.Logging.MaxAge, Source: source, Path: path},
			MaxBackups: ConfigValueInfo{Value: cfg.Logging.MaxBackups, Source: source, Path: path},
		},
	}
}

// updateTrackedFromConfig updates tracked config with values from another config
func updateTrackedFromConfig(tracked *TrackedConfig, cfg *entities.Config, source ConfigSource, path string) {
	// Server
	if cfg.Server.Host != "" {
		tracked.Server.Host = ConfigValueInfo{Value: cfg.Server.Host, Source: source, Path: path}
	}
	if cfg.Server.Port != 0 {
		tracked.Server.Port = ConfigValueInfo{Value: cfg.Server.Port, Source: source, Path: path}
	}
	if cfg.Server.ReadTimeout != 0 {
		tracked.Server.ReadTimeout = ConfigValueInfo{Value: formatDuration(cfg.Server.ReadTimeout), Source: source, Path: path}
	}
	if cfg.Server.WriteTimeout != 0 {
		tracked.Server.WriteTimeout = ConfigValueInfo{Value: formatDuration(cfg.Server.WriteTimeout), Source: source, Path: path}
	}
	if cfg.Server.ShutdownTimeout != 0 {
		tracked.Server.ShutdownTimeout = ConfigValueInfo{Value: formatDuration(cfg.Server.ShutdownTimeout), Source: source, Path: path}
	}
	if cfg.Server.Environment != "" {
		tracked.Server.Environment = ConfigValueInfo{Value: cfg.Server.Environment, Source: source, Path: path}
	}
	if len(cfg.Server.CORSOrigins) > 0 {
		tracked.Server.CORSOrigins = ConfigValueInfo{Value: cfg.Server.CORSOrigins, Source: source, Path: path}
	}

	// Theme
	if cfg.Theme.Name != "" {
		tracked.Theme.Name = ConfigValueInfo{Value: cfg.Theme.Name, Source: source, Path: path}
	}
	if cfg.Theme.CustomPath != "" {
		tracked.Theme.CustomPath = ConfigValueInfo{Value: cfg.Theme.CustomPath, Source: source, Path: path}
	}

	// Browser - note: boolean fields are always set due to Go zero values
	tracked.Browser.AutoOpen = ConfigValueInfo{Value: cfg.Browser.AutoOpen, Source: source, Path: path}
	if cfg.Browser.Browser != "" {
		tracked.Browser.Browser = ConfigValueInfo{Value: cfg.Browser.Browser, Source: source, Path: path}
	}

	// Watcher
	if cfg.Watcher.IntervalMs != 0 {
		tracked.Watcher.IntervalMs = ConfigValueInfo{Value: cfg.Watcher.IntervalMs, Source: source, Path: path}
	}
	if cfg.Watcher.DebounceMs != 0 {
		tracked.Watcher.DebounceMs = ConfigValueInfo{Value: cfg.Watcher.DebounceMs, Source: source, Path: path}
	}
	if cfg.Watcher.MaxRetries != 0 {
		tracked.Watcher.MaxRetries = ConfigValueInfo{Value: cfg.Watcher.MaxRetries, Source: source, Path: path}
	}
	if cfg.Watcher.RetryDelayMs != 0 {
		tracked.Watcher.RetryDelayMs = ConfigValueInfo{Value: cfg.Watcher.RetryDelayMs, Source: source, Path: path}
	}

	// Plugins
	tracked.Plugins.Enabled = ConfigValueInfo{Value: cfg.Plugins.Enabled, Source: source, Path: path}
	if cfg.Plugins.Directory != "" {
		tracked.Plugins.Directory = ConfigValueInfo{Value: cfg.Plugins.Directory, Source: source, Path: path}
	}
	if len(cfg.Plugins.Whitelist) > 0 {
		tracked.Plugins.Whitelist = ConfigValueInfo{Value: cfg.Plugins.Whitelist, Source: source, Path: path}
	}
	if len(cfg.Plugins.Blacklist) > 0 {
		tracked.Plugins.Blacklist = ConfigValueInfo{Value: cfg.Plugins.Blacklist, Source: source, Path: path}
	}
	if cfg.Plugins.MarketplaceURL != "" {
		tracked.Plugins.MarketplaceURL = ConfigValueInfo{Value: cfg.Plugins.MarketplaceURL, Source: source, Path: path}
	}

	// Metadata
	if cfg.Metadata.Author != "" {
		tracked.Metadata.Author = ConfigValueInfo{Value: cfg.Metadata.Author, Source: source, Path: path}
	}
	if cfg.Metadata.Email != "" {
		tracked.Metadata.Email = ConfigValueInfo{Value: cfg.Metadata.Email, Source: source, Path: path}
	}
	if cfg.Metadata.Company != "" {
		tracked.Metadata.Company = ConfigValueInfo{Value: cfg.Metadata.Company, Source: source, Path: path}
	}
	if len(cfg.Metadata.DefaultTags) > 0 {
		tracked.Metadata.DefaultTags = ConfigValueInfo{Value: cfg.Metadata.DefaultTags, Source: source, Path: path}
	}

	// Logging
	if cfg.Logging.Level != "" {
		tracked.Logging.Level = ConfigValueInfo{Value: cfg.Logging.Level, Source: source, Path: path}
	}
	tracked.Logging.Verbose = ConfigValueInfo{Value: cfg.Logging.Verbose, Source: source, Path: path}
	tracked.Logging.JSONFormat = ConfigValueInfo{Value: cfg.Logging.JSONFormat, Source: source, Path: path}
	if cfg.Logging.File != "" {
		tracked.Logging.File = ConfigValueInfo{Value: cfg.Logging.File, Source: source, Path: path}
	}
	if cfg.Logging.MaxSize != 0 {
		tracked.Logging.MaxSize = ConfigValueInfo{Value: cfg.Logging.MaxSize, Source: source, Path: path}
	}
	if cfg.Logging.MaxAge != 0 {
		tracked.Logging.MaxAge = ConfigValueInfo{Value: cfg.Logging.MaxAge, Source: source, Path: path}
	}
	if cfg.Logging.MaxBackups != 0 {
		tracked.Logging.MaxBackups = ConfigValueInfo{Value: cfg.Logging.MaxBackups, Source: source, Path: path}
	}
}

// applyEnvOverrides checks for environment variable overrides
func applyEnvOverrides(tracked *TrackedConfig) {
	// Server
	if v := os.Getenv("SLICLI_HOST"); v != "" {
		tracked.Server.Host = ConfigValueInfo{Value: v, Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_PORT"); v != "" {
		tracked.Server.Port = ConfigValueInfo{Value: v, Source: SourceEnv}
	}

	// Theme
	if v := os.Getenv("SLICLI_THEME"); v != "" {
		tracked.Theme.Name = ConfigValueInfo{Value: v, Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_THEME_PATH"); v != "" {
		tracked.Theme.CustomPath = ConfigValueInfo{Value: v, Source: SourceEnv}
	}

	// Browser
	if v := os.Getenv("SLICLI_NO_BROWSER"); v != "" {
		tracked.Browser.AutoOpen = ConfigValueInfo{Value: v != "true" && v != "1", Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_BROWSER"); v != "" {
		tracked.Browser.Browser = ConfigValueInfo{Value: v, Source: SourceEnv}
	}

	// Plugins
	if v := os.Getenv("SLICLI_PLUGINS_ENABLED"); v != "" {
		tracked.Plugins.Enabled = ConfigValueInfo{Value: v == "true" || v == "1", Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_PLUGINS_DIR"); v != "" {
		tracked.Plugins.Directory = ConfigValueInfo{Value: v, Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_MARKETPLACE_URL"); v != "" {
		tracked.Plugins.MarketplaceURL = ConfigValueInfo{Value: v, Source: SourceEnv}
	}

	// Logging
	if v := os.Getenv("SLICLI_LOG_LEVEL"); v != "" {
		tracked.Logging.Level = ConfigValueInfo{Value: v, Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_LOG_VERBOSE"); v != "" {
		tracked.Logging.Verbose = ConfigValueInfo{Value: v == "true" || v == "1", Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_LOG_JSON"); v != "" {
		tracked.Logging.JSONFormat = ConfigValueInfo{Value: v == "true" || v == "1", Source: SourceEnv}
	}
	if v := os.Getenv("SLICLI_LOG_FILE"); v != "" {
		tracked.Logging.File = ConfigValueInfo{Value: v, Source: SourceEnv}
	}
}

// formatDuration formats seconds as a duration string
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return "30s"
	}
	return fmt.Sprintf("%ds", seconds)
}

// printConfigJSON outputs the configuration as JSON
func printConfigJSON(tracked *TrackedConfig) error {
	data, err := json.MarshalIndent(tracked, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// printConfigFormatted outputs the configuration in a human-readable format
func printConfigFormatted(tracked *TrackedConfig) error {
	fmt.Println("SLICLI Configuration")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Server section
	fmt.Println("Server:")
	printConfigValue("host", tracked.Server.Host)
	printConfigValue("port", tracked.Server.Port)
	printConfigValue("read_timeout", tracked.Server.ReadTimeout)
	printConfigValue("write_timeout", tracked.Server.WriteTimeout)
	printConfigValue("shutdown_timeout", tracked.Server.ShutdownTimeout)
	printConfigValue("environment", tracked.Server.Environment)
	printConfigValue("cors_origins", tracked.Server.CORSOrigins)
	fmt.Println()

	// Theme section
	fmt.Println("Theme:")
	printConfigValue("name", tracked.Theme.Name)
	printConfigValue("custom_path", tracked.Theme.CustomPath)
	fmt.Println()

	// Browser section
	fmt.Println("Browser:")
	printConfigValue("auto_open", tracked.Browser.AutoOpen)
	printConfigValue("browser", tracked.Browser.Browser)
	fmt.Println()

	// Watcher section
	fmt.Println("Watcher:")
	printConfigValue("interval_ms", tracked.Watcher.IntervalMs)
	printConfigValue("debounce_ms", tracked.Watcher.DebounceMs)
	printConfigValue("max_retries", tracked.Watcher.MaxRetries)
	printConfigValue("retry_delay_ms", tracked.Watcher.RetryDelayMs)
	fmt.Println()

	// Plugins section
	fmt.Println("Plugins:")
	printConfigValue("enabled", tracked.Plugins.Enabled)
	printConfigValue("directory", tracked.Plugins.Directory)
	printConfigValue("whitelist", tracked.Plugins.Whitelist)
	printConfigValue("blacklist", tracked.Plugins.Blacklist)
	printConfigValue("marketplace_url", tracked.Plugins.MarketplaceURL)
	fmt.Println()

	// Metadata section
	fmt.Println("Metadata:")
	printConfigValue("author", tracked.Metadata.Author)
	printConfigValue("email", tracked.Metadata.Email)
	printConfigValue("company", tracked.Metadata.Company)
	printConfigValue("default_tags", tracked.Metadata.DefaultTags)
	fmt.Println()

	// Logging section
	fmt.Println("Logging:")
	printConfigValue("level", tracked.Logging.Level)
	printConfigValue("verbose", tracked.Logging.Verbose)
	printConfigValue("json_format", tracked.Logging.JSONFormat)
	printConfigValue("file", tracked.Logging.File)
	printConfigValue("max_size", tracked.Logging.MaxSize)
	printConfigValue("max_age", tracked.Logging.MaxAge)
	printConfigValue("max_backups", tracked.Logging.MaxBackups)
	fmt.Println()

	// Sources loaded
	if len(tracked.Sources) > 0 {
		fmt.Println("Sources loaded:")
		for _, src := range tracked.Sources {
			fmt.Printf("  - %s (%s)\n", src.Path, src.Type)
		}
	} else {
		fmt.Println("Sources loaded: (using defaults only)")
	}

	return nil
}

// printConfigValue prints a single configuration value with its source
func printConfigValue(name string, info ConfigValueInfo) {
	valueStr := formatValue(info.Value)
	sourceStr := formatSource(info)

	fmt.Printf("  %-18s %-20s %s\n", name+":", valueStr, sourceStr)
}

// formatValue formats a configuration value for display
func formatValue(value interface{}) string {
	if value == nil {
		return "-"
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return "-"
		}
		return v
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%s]", strings.Join(v, ", "))
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatSource formats the source information for display
func formatSource(info ConfigValueInfo) string {
	switch info.Source {
	case SourceDefault:
		return "(default)"
	case SourceGlobal:
		return "(global)"
	case SourceLocal:
		return fmt.Sprintf("(local: %s)", filepath.Base(info.Path))
	case SourceEnv:
		return "(env)"
	case SourceFlag:
		return "(flag)"
	default:
		return ""
	}
}

// getMinimalConfigTemplate returns a minimal configuration template
func getMinimalConfigTemplate() string {
	return `# SLICLI Configuration
# See documentation for all options: https://github.com/fredcamaral/slicli

[server]
port = 1000

[theme]
name = "default"

[browser]
auto_open = true
`
}

// getFullConfigTemplate returns a full configuration template with all options
func getFullConfigTemplate() string {
	return `# SLICLI Configuration
# See documentation for all options: https://github.com/fredcamaral/slicli
#
# Configuration precedence (highest to lowest):
#   1. CLI flags (e.g., --port 8080)
#   2. Environment variables (e.g., SLICLI_PORT=8080)
#   3. Local config (./slicli.toml)
#   4. Global config (~/.config/slicli/config.toml)
#   5. Built-in defaults

# ==============================================================================
# Server Configuration
# ==============================================================================
[server]
# Host to bind the server to
# Default: "localhost"
host = "localhost"

# Port number for the HTTP server
# Default: 1000
port = 1000

# Read timeout in seconds for HTTP requests
# Default: 30
read_timeout = 30

# Write timeout in seconds for HTTP responses
# Default: 30
write_timeout = 30

# Graceful shutdown timeout in seconds
# Default: 5
shutdown_timeout = 5

# Server environment: "development" or "production"
# Default: "" (treated as development)
# environment = "development"

# CORS allowed origins (for API access)
# Default: ["http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:8080", "http://127.0.0.1:8080"]
# cors_origins = ["http://localhost:3000"]

# ==============================================================================
# Theme Configuration
# ==============================================================================
[theme]
# Theme name to use for presentations
# Available: default, developer-dark, academic-research, executive-pro, etc.
# Default: "default"
name = "default"

# Path to custom theme directory (optional)
# Must be an absolute path
# custom_path = ""

# ==============================================================================
# Browser Configuration
# ==============================================================================
[browser]
# Automatically open browser when starting server
# Default: true
auto_open = true

# Preferred browser to use
# Options: "default", "chrome", "firefox", "safari", "edge"
# Default: "default"
browser = "default"

# ==============================================================================
# File Watcher Configuration
# ==============================================================================
[watcher]
# Polling interval in milliseconds
# Minimum: 50, Default: 200
interval_ms = 200

# Debounce time in milliseconds to prevent multiple reloads
# Default: 500
debounce_ms = 500

# Maximum retry attempts for file watching errors
# Default: 3
max_retries = 3

# Delay between retries in milliseconds
# Default: 100
retry_delay_ms = 100

# ==============================================================================
# Plugin Configuration
# ==============================================================================
[plugins]
# Enable plugin system
# Default: true
enabled = true

# Custom plugins directory (optional)
# Must be an absolute path if specified
# directory = ""

# Plugin whitelist - only allow these plugins (optional)
# If empty, all plugins are allowed (except blacklisted)
# whitelist = ["syntax-highlight", "mermaid"]

# Plugin blacklist - never load these plugins (optional)
# blacklist = []

# Marketplace URL for plugin downloads
# Must use HTTPS for security
# Default: "https://marketplace.slicli.dev"
marketplace_url = "https://marketplace.slicli.dev"

# ==============================================================================
# Presentation Metadata Defaults
# ==============================================================================
[metadata]
# Default author for presentations
# author = ""

# Default email for presentations
# email = ""

# Default company/organization for presentations
# company = ""

# Default tags to apply to presentations
# default_tags = []

# ==============================================================================
# Logging Configuration
# ==============================================================================
[logging]
# Log level: "debug", "info", "warn", "error"
# Default: "info"
level = "info"

# Enable verbose output
# Default: false
verbose = false

# Output logs in JSON format
# Default: false
json_format = false

# Log file path (optional, logs to stdout if not specified)
# Must be an absolute path if specified
# file = ""

# Maximum log file size in MB before rotation
# Default: 100
max_size = 100

# Maximum age in days to retain old log files
# Default: 7
max_age = 7

# Maximum number of backup log files to retain
# Default: 5
max_backups = 5
`
}
