package entities

import (
	"errors"
	"regexp"
	"time"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// PluginStatus represents the current state of a plugin.
type PluginStatus string

const (
	PluginStatusUnloaded PluginStatus = "unloaded"
	PluginStatusLoading  PluginStatus = "loading"
	PluginStatusLoaded   PluginStatus = "loaded"
	PluginStatusActive   PluginStatus = "active"
	PluginStatusError    PluginStatus = "error"
	PluginStatusDisabled PluginStatus = "disabled"
)

// PluginType represents the type of plugin.
type PluginType string

const (
	PluginTypeProcessor PluginType = "processor" // Content processor (e.g., Mermaid, syntax highlighting)
	PluginTypeExporter  PluginType = "exporter"  // Export format (e.g., PDF, PPTX)
	PluginTypeTheme     PluginType = "theme"     // Theme provider
	PluginTypeAnalyzer  PluginType = "analyzer"  // Content analyzer (e.g., readability, statistics)
)

// PluginMetadata contains metadata about a plugin.
type PluginMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	Homepage    string            `json:"homepage"`
	Type        PluginType        `json:"type"`
	Tags        []string          `json:"tags"`
	Config      map[string]string `json:"config"` // Default configuration
}

// LoadedPlugin represents a plugin that has been loaded into memory.
type LoadedPlugin struct {
	Metadata   PluginMetadata
	Path       string
	Status     PluginStatus
	LoadedAt   time.Time
	LastUsed   time.Time
	ErrorMsg   string
	Statistics PluginStatistics
	Config     *PluginConfig   // Runtime configuration
	Manifest   *PluginManifest // Plugin manifest if available
}

// PluginStatistics tracks plugin usage and performance.
type PluginStatistics struct {
	ExecutionCount  int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	LastExecuted    time.Time
	SuccessCount    int64
	ErrorCount      int64
	TimeoutCount    int64
	PanicCount      int64
	BytesProcessed  int64
	BytesGenerated  int64
}

// PluginConfig represents runtime configuration for a plugin.
type PluginConfig struct {
	Enabled         bool                   `toml:"enabled"`
	Priority        int                    `toml:"priority"`         // Execution order when multiple plugins match
	Timeout         time.Duration          `toml:"timeout"`          // Execution timeout
	MaxMemory       int64                  `toml:"max_memory"`       // Maximum memory usage in bytes
	CacheResults    bool                   `toml:"cache_results"`    // Whether to cache plugin outputs
	CacheTTL        time.Duration          `toml:"cache_ttl"`        // Cache time-to-live
	Options         map[string]interface{} `toml:"options"`          // Plugin-specific options
	FileExtensions  []string               `toml:"file_extensions"`  // File extensions this plugin handles
	ContentPatterns []string               `toml:"content_patterns"` // Regex patterns for content matching
}

// PluginManifest represents the manifest file for a plugin.
type PluginManifest struct {
	Metadata      PluginMetadata     `toml:"metadata"`
	Requirements  PluginRequirements `toml:"requirements"`
	Capabilities  PluginCapabilities `toml:"capabilities"`
	DefaultConfig PluginConfig       `toml:"config"`
}

// PluginRequirements specifies what a plugin needs to run.
type PluginRequirements struct {
	MinSlicliVersion string   `toml:"min_slicli_version"`
	MaxSlicliVersion string   `toml:"max_slicli_version"`
	OS               []string `toml:"os"`           // Supported operating systems
	Arch             []string `toml:"arch"`         // Supported architectures
	Dependencies     []string `toml:"dependencies"` // Other required plugins
}

// PluginCapabilities describes what a plugin can do.
type PluginCapabilities struct {
	InputFormats  []string `toml:"input_formats"`  // Formats the plugin can process
	OutputFormats []string `toml:"output_formats"` // Formats the plugin can generate
	Features      []string `toml:"features"`       // Named features the plugin provides
	Concurrent    bool     `toml:"concurrent"`     // Whether the plugin is thread-safe
	Streaming     bool     `toml:"streaming"`      // Whether the plugin supports streaming
}

// PluginInstance represents a plugin instance ready for execution
type PluginInstance struct {
	Instance pluginapi.Plugin
	Metadata PluginMetadata
}

// Validate validates the plugin metadata.
func (m *PluginMetadata) Validate() error {
	if m.Name == "" {
		return errors.New("plugin name is required")
	}

	// Validate name format (alphanumeric, hyphens, underscores)
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !nameRegex.MatchString(m.Name) {
		return errors.New("plugin name must contain only alphanumeric characters, hyphens, and underscores")
	}

	if m.Version == "" {
		return errors.New("plugin version is required")
	}

	// Validate semantic version format
	versionRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	if !versionRegex.MatchString(m.Version) {
		return errors.New("plugin version must follow semantic versioning (e.g., 1.0.0)")
	}

	if m.Type == "" {
		return errors.New("plugin type is required")
	}

	// Validate plugin type
	switch m.Type {
	case PluginTypeProcessor, PluginTypeExporter, PluginTypeTheme, PluginTypeAnalyzer:
		// Valid type
	default:
		return errors.New("invalid plugin type")
	}

	return nil
}

// IsActive returns true if the plugin is in an active state.
func (p *LoadedPlugin) IsActive() bool {
	return p.Status == PluginStatusActive || p.Status == PluginStatusLoaded
}

// UpdateStatistics updates the plugin statistics after an execution.
func (p *LoadedPlugin) UpdateStatistics(duration time.Duration, success bool, bytesIn, bytesOut int64) {
	p.Statistics.ExecutionCount++
	p.Statistics.TotalDuration += duration
	p.Statistics.AverageDuration = p.Statistics.TotalDuration / time.Duration(p.Statistics.ExecutionCount)
	p.Statistics.LastExecuted = time.Now()
	p.Statistics.BytesProcessed += bytesIn
	p.Statistics.BytesGenerated += bytesOut

	if success {
		p.Statistics.SuccessCount++
	} else {
		p.Statistics.ErrorCount++
	}

	p.LastUsed = time.Now()
}

// IncrementTimeout increments the timeout counter.
func (p *LoadedPlugin) IncrementTimeout() {
	p.Statistics.TimeoutCount++
	p.Statistics.ErrorCount++
}

// IncrementPanic increments the panic counter.
func (p *LoadedPlugin) IncrementPanic() {
	p.Statistics.PanicCount++
	p.Statistics.ErrorCount++
}

// GetPluginTimeout returns the effective timeout for this plugin.
func (p *LoadedPlugin) GetPluginTimeout() time.Duration {
	// Priority order: Runtime Config > Manifest Config > Default

	// 1. Check runtime configuration
	if p.Config != nil && p.Config.Timeout > 0 {
		return p.Config.Timeout
	}

	// 2. Check manifest default configuration
	if p.Manifest != nil && p.Manifest.DefaultConfig.Timeout > 0 {
		return p.Manifest.DefaultConfig.Timeout
	}

	// 3. Return zero to indicate no custom timeout (caller should use default)
	return 0
}

// MatchesContent checks if the plugin should handle the given content.
func (c *PluginConfig) MatchesContent(content string, fileExt string) bool {
	// Check file extension
	for _, ext := range c.FileExtensions {
		if ext == fileExt {
			return true
		}
	}

	// Check content patterns
	for _, pattern := range c.ContentPatterns {
		if matched, err := regexp.MatchString(pattern, content); err == nil && matched {
			return true
		}
	}

	return false
}

// GetTimeout returns the effective timeout for the plugin.
func (c *PluginConfig) GetTimeout() time.Duration {
	if c.Timeout <= 0 {
		return 5 * time.Second // Default timeout
	}
	return c.Timeout
}

// IsCompatible checks if the plugin is compatible with the current environment.
func (r *PluginRequirements) IsCompatible(slicliVersion, os, arch string) bool {
	// Check version compatibility
	if r.MinSlicliVersion != "" && slicliVersion < r.MinSlicliVersion {
		return false
	}
	if r.MaxSlicliVersion != "" && slicliVersion > r.MaxSlicliVersion {
		return false
	}

	// Check OS compatibility
	if len(r.OS) > 0 {
		found := false
		for _, supportedOS := range r.OS {
			if supportedOS == os || supportedOS == "any" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check architecture compatibility
	if len(r.Arch) > 0 {
		found := false
		for _, supportedArch := range r.Arch {
			if supportedArch == arch || supportedArch == "any" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
