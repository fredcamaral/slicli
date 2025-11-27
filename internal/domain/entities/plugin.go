package entities

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
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
	// Security contains integrity and signature information for the plugin
	Security PluginSecurity `toml:"security"`
}

// PluginSecurity contains security-related information for a plugin.
// This enables integrity verification and optional cryptographic signatures.
type PluginSecurity struct {
	// Signature is an optional cryptographic signature for the plugin binary.
	// Format: base64-encoded signature using the algorithm specified in SignatureType.
	Signature string `toml:"signature"`
	// SignatureType specifies the signature algorithm (e.g., "ed25519", "rsa-sha256").
	// Required if Signature is provided.
	SignatureType string `toml:"signature_type"`
	// PublicKeyID identifies the public key used to verify the signature.
	// This allows key rotation and identification of the signing authority.
	PublicKeyID string `toml:"public_key_id"`
	// SHA256 is the hex-encoded SHA256 hash of the plugin binary for integrity verification.
	SHA256 string `toml:"sha256"`
}

// HasSignature returns true if the manifest has valid signature fields.
func (s *PluginSecurity) HasSignature() bool {
	return s.Signature != "" && s.SignatureType != ""
}

// HasChecksum returns true if the manifest has checksum information.
func (s *PluginSecurity) HasChecksum() bool {
	return s.SHA256 != ""
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
// Content patterns are user-supplied regex patterns; invalid patterns are skipped.
func (c *PluginConfig) MatchesContent(content string, fileExt string) bool {
	// Check file extension first (fast path)
	for _, ext := range c.FileExtensions {
		if ext == fileExt {
			return true
		}
	}

	// Check content patterns with compiled regex for better performance.
	// Invalid patterns are logged and skipped to prevent blocking.
	for _, pattern := range c.ContentPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// Invalid pattern - skip it rather than fail
			continue
		}
		// Use FindStringIndex for early termination on match
		if re.FindStringIndex(content) != nil {
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
	// Check version compatibility using proper semantic version comparison
	if r.MinSlicliVersion != "" {
		if compareSemanticVersions(slicliVersion, r.MinSlicliVersion) < 0 {
			return false
		}
	}
	if r.MaxSlicliVersion != "" {
		if compareSemanticVersions(slicliVersion, r.MaxSlicliVersion) > 0 {
			return false
		}
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

// compareSemanticVersions compares two semantic version strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
// Handles versions with or without "v" prefix.
func compareSemanticVersions(v1, v2 string) int {
	// Normalize versions by removing "v" prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Split versions into components
	parts1 := parseVersionParts(v1)
	parts2 := parseVersionParts(v2)

	// Compare major, minor, patch
	for i := 0; i < 3; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// parseVersionParts parses a version string into numeric components.
// For example, "1.10.2-beta" returns [1, 10, 2].
func parseVersionParts(version string) []int {
	// Remove prerelease and build metadata
	if idx := strings.IndexAny(version, "-+"); idx != -1 {
		version = version[:idx]
	}

	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			// If parsing fails, treat as 0
			num = 0
		}
		result = append(result, num)
	}

	return result
}
