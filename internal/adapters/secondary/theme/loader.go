package theme

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DirectoryLoader loads themes from filesystem directories
type DirectoryLoader struct {
	baseDir string
	funcMap template.FuncMap
}

// NewDirectoryLoader creates a new directory-based theme loader
func NewDirectoryLoader(baseDir string) *DirectoryLoader {
	return &DirectoryLoader{
		baseDir: baseDir,
		funcMap: createDefaultFuncMap(),
	}
}

// Load loads a theme by name
func (l *DirectoryLoader) Load(ctx context.Context, name string) (*entities.ThemeEngine, error) {
	return l.loadWithHistory(ctx, name, make(map[string]bool))
}

// loadWithHistory loads a theme while tracking visited themes to prevent circular references
func (l *DirectoryLoader) loadWithHistory(ctx context.Context, name string, visited map[string]bool) (*entities.ThemeEngine, error) {
	// Check for circular reference
	if visited[name] {
		return nil, fmt.Errorf("circular reference detected in theme hierarchy: %s", name)
	}
	visited[name] = true
	themePath := filepath.Join(l.baseDir, name)

	// Check if theme directory exists
	info, err := os.Stat(themePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("theme '%s' not found", name)
		}
		return nil, fmt.Errorf("accessing theme directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("theme path is not a directory: %s", themePath)
	}

	theme := &entities.ThemeEngine{
		Name:      name,
		Path:      themePath,
		Templates: make(map[string]*template.Template),
		Assets:    make(map[string]*entities.ThemeAsset),
		LoadedAt:  time.Now(),
	}

	// Load theme configuration
	if err := l.loadConfig(theme); err != nil {
		return nil, fmt.Errorf("loading theme config: %w", err)
	}

	// Load templates
	if err := l.loadTemplates(theme); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	// Load assets
	if err := l.loadAssets(theme); err != nil {
		return nil, fmt.Errorf("loading assets: %w", err)
	}

	// Handle theme inheritance
	if theme.Parent != "" {
		parent, err := l.loadWithHistory(ctx, theme.Parent, visited)
		if err != nil {
			return nil, fmt.Errorf("loading parent theme '%s': %w", theme.Parent, err)
		}
		l.mergeThemes(theme, parent)
	}

	// Validate the loaded theme
	if err := theme.Validate(); err != nil {
		return nil, fmt.Errorf("theme validation failed: %w", err)
	}

	return theme, nil
}

// List returns information about all available themes
func (l *DirectoryLoader) List(ctx context.Context) ([]entities.ThemeInfo, error) {
	entries, err := os.ReadDir(l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading themes directory: %w", err)
	}

	var themes []entities.ThemeInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Try to load theme config
		configPath := filepath.Join(l.baseDir, entry.Name(), "theme.toml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue // Skip directories without theme.toml
		}

		info, err := l.loadThemeInfo(entry.Name(), configPath)
		if err != nil {
			// Log error but continue with other themes
			continue
		}

		themes = append(themes, info)
	}

	return themes, nil
}

// Exists checks if a theme exists
func (l *DirectoryLoader) Exists(ctx context.Context, name string) bool {
	themePath := filepath.Join(l.baseDir, name)
	info, err := os.Stat(themePath)
	return err == nil && info.IsDir()
}

// Reload reloads a theme (for hot reload)
func (l *DirectoryLoader) Reload(ctx context.Context, name string) (*entities.ThemeEngine, error) {
	// For now, just load again - cache invalidation handled elsewhere
	return l.Load(ctx, name)
}

// loadConfig loads the theme.toml configuration file
func (l *DirectoryLoader) loadConfig(theme *entities.ThemeEngine) error {
	configPath := filepath.Join(theme.Path, "theme.toml")

	data, err := os.ReadFile(configPath) // #nosec G304 - configPath constructed from validated theme path
	if err != nil {
		if os.IsNotExist(err) {
			// Use default config if theme.toml doesn't exist
			theme.Config = entities.ThemeEngineConfig{
				Variables: make(map[string]string),
				Features:  make(map[string]bool),
			}
			return nil
		}
		return fmt.Errorf("reading theme.toml: %w", err)
	}

	var config struct {
		Parent      string                    `toml:"parent"`
		Variables   map[string]string         `toml:"variables"`
		Fonts       []entities.FontConfig     `toml:"fonts"`
		Transitions entities.TransitionConfig `toml:"transitions"`
		Features    map[string]bool           `toml:"features"`
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing theme.toml: %w", err)
	}

	theme.Parent = config.Parent
	theme.Config = entities.ThemeEngineConfig{
		Variables:   config.Variables,
		Fonts:       config.Fonts,
		Transitions: config.Transitions,
		Features:    config.Features,
	}

	// Initialize maps if nil
	if theme.Config.Variables == nil {
		theme.Config.Variables = make(map[string]string)
	}
	if theme.Config.Features == nil {
		theme.Config.Features = make(map[string]bool)
	}

	return nil
}

// loadTemplates loads all HTML templates from the templates directory
func (l *DirectoryLoader) loadTemplates(theme *entities.ThemeEngine) error {
	templatesDir := filepath.Join(theme.Path, "templates")

	// Check if templates directory exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return errors.New("templates directory not found")
	}

	return filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Calculate template name relative to templates directory
		rel, err := filepath.Rel(templatesDir, path)
		if err != nil {
			return err
		}

		// Convert path to template name (remove .html, use forward slashes)
		name := strings.TrimSuffix(rel, ".html")
		name = strings.ReplaceAll(name, string(filepath.Separator), "/")

		// Read template content
		content, err := os.ReadFile(path) // #nosec G304 - path from controlled theme directory
		if err != nil {
			return fmt.Errorf("reading template %s: %w", name, err)
		}

		// Parse template
		tmpl, err := template.New(name).Funcs(l.funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}

		theme.Templates[name] = tmpl
		return nil
	})
}

// loadAssets loads all assets from the assets directory
func (l *DirectoryLoader) loadAssets(theme *entities.ThemeEngine) error {
	assetsDir := filepath.Join(theme.Path, "assets")

	// Check if assets directory exists
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		// Assets directory is optional
		return nil
	}

	return filepath.WalkDir(assetsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Calculate asset path relative to assets directory
		rel, err := filepath.Rel(assetsDir, path)
		if err != nil {
			return err
		}

		// Use forward slashes for asset paths
		assetPath := strings.ReplaceAll(rel, string(filepath.Separator), "/")

		// Get file info
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting file info for %s: %w", assetPath, err)
		}

		// Read asset content
		content, err := os.ReadFile(path) // #nosec G304 - path from controlled theme directory
		if err != nil {
			return fmt.Errorf("reading asset %s: %w", assetPath, err)
		}

		asset := &entities.ThemeAsset{
			Path:        assetPath,
			Content:     content,
			ContentType: entities.GetContentType(assetPath),
			ModTime:     info.ModTime(),
			Size:        info.Size(),
		}

		// Compute hash for caching
		asset.ComputeHash()

		theme.Assets[assetPath] = asset
		return nil
	})
}

// mergeThemes merges parent theme into child theme (inheritance)
func (l *DirectoryLoader) mergeThemes(child, parent *entities.ThemeEngine) {
	// Merge templates (child overrides parent)
	for name, tmpl := range parent.Templates {
		if _, exists := child.Templates[name]; !exists {
			child.Templates[name] = tmpl
		}
	}

	// Merge assets (child overrides parent)
	for path, asset := range parent.Assets {
		if _, exists := child.Assets[path]; !exists {
			child.Assets[path] = asset
		}
	}

	// Merge CSS variables (child overrides parent)
	if child.Config.Variables == nil {
		child.Config.Variables = make(map[string]string)
	}
	for key, value := range parent.Config.Variables {
		if _, exists := child.Config.Variables[key]; !exists {
			child.Config.Variables[key] = value
		}
	}

	// Merge features (child overrides parent)
	if child.Config.Features == nil {
		child.Config.Features = make(map[string]bool)
	}
	for key, value := range parent.Config.Features {
		if _, exists := child.Config.Features[key]; !exists {
			child.Config.Features[key] = value
		}
	}

	// Use parent's transition config if child doesn't specify
	if child.Config.Transitions.Type == "" && parent.Config.Transitions.Type != "" {
		child.Config.Transitions = parent.Config.Transitions
	}
}

// loadThemeInfo loads basic theme information from theme.toml
func (l *DirectoryLoader) loadThemeInfo(name, configPath string) (entities.ThemeInfo, error) {
	data, err := os.ReadFile(configPath) // #nosec G304 - configPath constructed from validated theme directory
	if err != nil {
		return entities.ThemeInfo{}, err
	}

	var config struct {
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
		Author      string `toml:"author"`
		Version     string `toml:"version"`
		Parent      string `toml:"parent"`
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		return entities.ThemeInfo{}, err
	}

	info := entities.ThemeInfo{
		Name:        name,
		DisplayName: config.DisplayName,
		Description: config.Description,
		Author:      config.Author,
		Version:     config.Version,
		Parent:      config.Parent,
		BuiltIn:     isBuiltInTheme(name),
	}

	// Set defaults
	if info.DisplayName == "" {
		info.DisplayName = name
	}
	if info.Version == "" {
		info.Version = "1.0.0"
	}

	return info, nil
}

// createDefaultFuncMap creates the default template function map
func createDefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// String functions
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": func(s string) string {
			// Using golang.org/x/text/cases for proper title case
			c := cases.Title(language.Und)
			return c.String(s)
		},
		"trim": strings.TrimSpace,

		// Slide functions
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },

		// CSS class helpers
		"classIf": func(condition bool, class string) string {
			if condition {
				return class
			}
			return ""
		},

		// Asset helpers
		"asset": func(path string) string {
			return "/assets/" + path
		},

		// Date formatting
		"formatDate": func(t time.Time, format string) string {
			return t.Format(format)
		},

		// Safe HTML - intentional template helper functions
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203 - intentional safe HTML template function
		},
		"safeJS": func(s string) template.JS {
			return template.JS(s) // #nosec G203 - intentional safe JS template function
		},
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s) // #nosec G203 - intentional safe CSS template function
		},
	}
}

// isBuiltInTheme checks if a theme name is a built-in theme
func isBuiltInTheme(name string) bool {
	builtInThemes := []string{"default", "minimal", "dark"}
	for _, builtin := range builtInThemes {
		if name == builtin {
			return true
		}
	}
	return false
}

// Ensure DirectoryLoader implements ThemeLoader
var _ ports.ThemeLoader = (*DirectoryLoader)(nil)
