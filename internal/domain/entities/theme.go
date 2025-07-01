package entities

import (
	"errors"
	"path/filepath"
	"strings"
)

// Theme represents a presentation theme configuration
type Theme struct {
	// Name is the theme identifier
	Name string `toml:"name" json:"name"`

	// DisplayName is the human-readable theme name
	DisplayName string `toml:"display_name" json:"display_name"`

	// Description provides details about the theme
	Description string `toml:"description" json:"description"`

	// Author is the theme creator
	Author string `toml:"author" json:"author"`

	// Version is the theme version
	Version string `toml:"version" json:"version"`

	// TemplatesPath is the path to theme templates
	TemplatesPath string `toml:"templates_path" json:"templates_path"`

	// AssetsPath is the path to theme assets (CSS, JS, images)
	AssetsPath string `toml:"assets_path" json:"assets_path"`

	// Config contains theme-specific configuration
	Config map[string]interface{} `toml:"config" json:"config,omitempty"`
}

// Validate ensures the theme has valid required fields
func (t *Theme) Validate() error {
	if t.Name == "" {
		return errors.New("theme name is required")
	}

	// Name should be lowercase with only alphanumeric and hyphens
	if !isValidThemeName(t.Name) {
		return errors.New("theme name must contain only lowercase letters, numbers, and hyphens")
	}

	if t.DisplayName == "" {
		t.DisplayName = t.Name
	}

	if t.Version == "" {
		t.Version = "1.0.0"
	}

	// Ensure paths are relative
	if filepath.IsAbs(t.TemplatesPath) {
		return errors.New("templates path must be relative")
	}

	if filepath.IsAbs(t.AssetsPath) {
		return errors.New("assets path must be relative")
	}

	return nil
}

// GetTemplatePath returns the full path to a template file
func (t *Theme) GetTemplatePath(templateName string) string {
	if t.TemplatesPath == "" {
		return filepath.Join("themes", t.Name, "templates", templateName)
	}
	return filepath.Join(t.TemplatesPath, templateName)
}

// GetAssetPath returns the full path to an asset file
func (t *Theme) GetAssetPath(assetName string) string {
	if t.AssetsPath == "" {
		return filepath.Join("themes", t.Name, "assets", assetName)
	}
	return filepath.Join(t.AssetsPath, assetName)
}

// IsBuiltIn returns true if this is a built-in theme
func (t *Theme) IsBuiltIn() bool {
	builtInThemes := []string{"default", "minimal", "dark"}
	for _, name := range builtInThemes {
		if t.Name == name {
			return true
		}
	}
	return false
}

// isValidThemeName checks if a theme name is valid
func isValidThemeName(name string) bool {
	if name == "" {
		return false
	}

	for _, char := range name {
		isLowercase := char >= 'a' && char <= 'z'
		isDigit := char >= '0' && char <= '9'
		isHyphen := char == '-'

		if !isLowercase && !isDigit && !isHyphen {
			return false
		}
	}

	// Cannot start or end with hyphen
	return !strings.HasPrefix(name, "-") && !strings.HasSuffix(name, "-")
}
