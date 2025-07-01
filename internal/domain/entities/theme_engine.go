package entities

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"time"
)

// ThemeEngine represents a loaded theme with templates and assets
type ThemeEngine struct {
	// Name is the theme identifier
	Name string

	// Path is the filesystem path to the theme
	Path string

	// Templates contains parsed HTML templates
	Templates map[string]*template.Template

	// Assets contains theme assets (CSS, JS, images, fonts)
	Assets map[string]*ThemeAsset

	// Config contains theme configuration
	Config ThemeEngineConfig

	// Parent is the name of the parent theme for inheritance
	Parent string

	// LoadedAt tracks when the theme was loaded
	LoadedAt time.Time
}

// ThemeAsset represents a theme asset file
type ThemeAsset struct {
	// Path is the relative path within the theme
	Path string

	// Content is the file content
	Content []byte

	// ContentType is the MIME type
	ContentType string

	// Hash is the SHA256 hash for caching
	Hash string

	// ModTime is the modification time
	ModTime time.Time

	// Size is the file size in bytes
	Size int64
}

// ThemeEngineConfig contains theme-specific configuration
type ThemeEngineConfig struct {
	// Variables contains CSS variable overrides
	Variables map[string]string `toml:"variables"`

	// Fonts contains font configurations
	Fonts []FontConfig `toml:"fonts"`

	// Transitions contains slide transition settings
	Transitions TransitionConfig `toml:"transitions"`

	// Features contains feature toggles
	Features map[string]bool `toml:"features"`
}

// FontConfig defines a custom font
type FontConfig struct {
	// Name is the font family name
	Name string `toml:"name"`

	// Files contains font file paths by weight/style
	Files map[string]string `toml:"files"`

	// Fallback is the fallback font stack
	Fallback string `toml:"fallback"`
}

// TransitionConfig defines slide transition settings
type TransitionConfig struct {
	// Type is the transition type (fade, slide, zoom, etc.)
	Type string `toml:"type"`

	// Duration in milliseconds
	Duration int `toml:"duration"`

	// Easing function (ease, ease-in, ease-out, etc.)
	Easing string `toml:"easing"`
}

// ThemeInfo provides basic theme information for listing
type ThemeInfo struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Version     string    `json:"version"`
	Parent      string    `json:"parent,omitempty"`
	BuiltIn     bool      `json:"built_in"`
	LoadedAt    time.Time `json:"loaded_at,omitempty"`
}

// Validate ensures the theme engine has valid required components
func (t *ThemeEngine) Validate() error {
	if t.Name == "" {
		return errors.New("theme name is required")
	}

	if t.Path == "" {
		return errors.New("theme path is required")
	}

	// Check required templates
	requiredTemplates := []string{"presentation", "slide", "notes"}
	for _, req := range requiredTemplates {
		if _, ok := t.Templates[req]; !ok {
			return fmt.Errorf("missing required template: %s", req)
		}
	}

	// Check required assets
	requiredAssets := []string{"css/main.css"}
	for _, req := range requiredAssets {
		if _, ok := t.Assets[req]; !ok {
			return fmt.Errorf("missing required asset: %s", req)
		}
	}

	// Validate configuration
	if err := t.Config.Validate(); err != nil {
		return fmt.Errorf("invalid theme config: %w", err)
	}

	return nil
}

// Validate ensures the theme config is valid
func (c *ThemeEngineConfig) Validate() error {
	// Validate CSS variables
	for key, value := range c.Variables {
		if key == "" || value == "" {
			return errors.New("CSS variables cannot have empty keys or values")
		}
	}

	// Validate fonts
	for _, font := range c.Fonts {
		if err := font.Validate(); err != nil {
			return fmt.Errorf("invalid font config: %w", err)
		}
	}

	// Validate transitions
	if err := c.Transitions.Validate(); err != nil {
		return fmt.Errorf("invalid transition config: %w", err)
	}

	return nil
}

// ComputeHash calculates the SHA256 hash of the asset content
func (a *ThemeAsset) ComputeHash() {
	hash := sha256.Sum256(a.Content)
	a.Hash = hex.EncodeToString(hash[:])
}

// GetETag returns an ETag header value for the asset
func (a *ThemeAsset) GetETag() string {
	if a.Hash == "" {
		a.ComputeHash()
	}
	// Use first 16 chars of hash for shorter ETag
	if len(a.Hash) >= 16 {
		return fmt.Sprintf(`"%s"`, a.Hash[:16])
	}
	return fmt.Sprintf(`"%s"`, a.Hash)
}

// GetCacheControl returns appropriate cache control headers
func (a *ThemeAsset) GetCacheControl() string {
	// Cache static assets for 1 hour in production
	// In development, this should be overridden to no-cache
	return "public, max-age=3600"
}

// IsTemplate checks if a path is a template file
func IsTemplatePath(path string) bool {
	return filepath.Ext(path) == ".html"
}

// IsAssetPath checks if a path is an asset file
func IsAssetPath(path string) bool {
	ext := filepath.Ext(path)
	assetExts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2", ".ttf", ".eot"}
	for _, e := range assetExts {
		if ext == e {
			return true
		}
	}
	return false
}

// GetAsset retrieves an asset by path from the theme
func (t *ThemeEngine) GetAsset(path string) *ThemeAsset {
	if asset, exists := t.Assets[path]; exists {
		return asset
	}
	return nil
}

// GetTemplate retrieves a template by name from the theme
func (t *ThemeEngine) GetTemplate(name string) *template.Template {
	if tmpl, exists := t.Templates[name]; exists {
		return tmpl
	}
	return nil
}

// ListAssets returns a list of all asset paths in the theme
func (t *ThemeEngine) ListAssets() []string {
	paths := make([]string, 0, len(t.Assets))
	for path := range t.Assets {
		paths = append(paths, path)
	}
	return paths
}

// ListTemplates returns a list of all template names in the theme
func (t *ThemeEngine) ListTemplates() []string {
	names := make([]string, 0, len(t.Templates))
	for name := range t.Templates {
		names = append(names, name)
	}
	return names
}

// HasAsset checks if an asset exists in the theme
func (t *ThemeEngine) HasAsset(path string) bool {
	_, exists := t.Assets[path]
	return exists
}

// HasTemplate checks if a template exists in the theme
func (t *ThemeEngine) HasTemplate(name string) bool {
	_, exists := t.Templates[name]
	return exists
}

// Validate validates individual font configuration
func (f *FontConfig) Validate() error {
	if f.Name == "" {
		return errors.New("font name is required")
	}

	if len(f.Files) == 0 {
		return errors.New("at least one font file is required")
	}

	// Validate font file paths
	for style, path := range f.Files {
		if style == "" {
			return errors.New("font style/weight cannot be empty")
		}
		if path == "" {
			return errors.New("font file path cannot be empty")
		}
		// Validate font file extensions
		ext := filepath.Ext(path)
		validExts := []string{".woff", ".woff2", ".ttf", ".otf", ".eot"}
		valid := false
		for _, validExt := range validExts {
			if ext == validExt {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid font file extension: %s (must be .woff, .woff2, .ttf, .otf, or .eot)", ext)
		}
	}

	// Validate fallback
	if f.Fallback != "" {
		validFallbacks := []string{"serif", "sans-serif", "monospace", "cursive", "fantasy"}
		valid := false
		for _, validFallback := range validFallbacks {
			if f.Fallback == validFallback {
				valid = true
				break
			}
		}
		// Allow custom fallback fonts (comma-separated list)
		if !valid && !strings.Contains(f.Fallback, ",") {
			return fmt.Errorf("invalid font fallback: %s (must be a valid CSS font family)", f.Fallback)
		}
	}

	return nil
}

// Validate validates individual transition configuration
func (tc *TransitionConfig) Validate() error {
	if tc.Type == "" {
		return nil // Empty type is valid (no transitions)
	}

	// Validate transition type
	validTypes := []string{"fade", "slide", "zoom", "flip", "cube", "none"}
	valid := false
	for _, validType := range validTypes {
		if tc.Type == validType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid transition type: %s (must be fade, slide, zoom, flip, cube, or none)", tc.Type)
	}

	// Validate duration
	if tc.Duration < 0 {
		return errors.New("transition duration must be non-negative")
	}
	if tc.Duration > 10000 {
		return errors.New("transition duration must be less than 10 seconds (10000ms)")
	}

	// Validate easing
	if tc.Easing != "" {
		validEasings := []string{"ease", "ease-in", "ease-out", "ease-in-out", "linear"}
		valid := false
		for _, validEasing := range validEasings {
			if tc.Easing == validEasing {
				valid = true
				break
			}
		}
		// Allow custom cubic-bezier functions
		if !valid && !strings.HasPrefix(tc.Easing, "cubic-bezier(") {
			return fmt.Errorf("invalid easing function: %s (must be ease, ease-in, ease-out, ease-in-out, linear, or a cubic-bezier function)", tc.Easing)
		}
	}

	return nil
}

// GetContentType returns the MIME type for a file path
func GetContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return "application/octet-stream"
	}
}
