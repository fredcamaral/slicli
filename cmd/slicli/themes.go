package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// ErrInvalidThemeName is returned when a theme name is not safe
var ErrInvalidThemeName = errors.New("invalid theme name")

// themeNamePattern defines the allowed pattern for theme names
// Only alphanumeric characters, dashes, and underscores are allowed
var themeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// maxThemeNameLength is the maximum allowed length for theme names
const maxThemeNameLength = 64

// defaultThemeName is the default theme used by slicli
const defaultThemeName = "default"

// ValidateThemeName validates that a theme name is safe for use in file paths.
func ValidateThemeName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty theme name", ErrInvalidThemeName)
	}

	if len(name) > maxThemeNameLength {
		return fmt.Errorf("%w: theme name exceeds maximum length of %d characters", ErrInvalidThemeName, maxThemeNameLength)
	}

	// Check for path traversal attempts
	if strings.Contains(name, "..") {
		return fmt.Errorf("%w: theme name contains path traversal sequence", ErrInvalidThemeName)
	}

	// Check for path separators
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("%w: theme name contains path separator", ErrInvalidThemeName)
	}

	// Check against allowed pattern
	if !themeNamePattern.MatchString(name) {
		return fmt.Errorf("%w: theme name contains invalid characters (only alphanumeric, dash, underscore allowed)", ErrInvalidThemeName)
	}

	return nil
}

// ThemeConfig represents the complete theme configuration from theme.toml
// Supports both legacy format (root-level fields) and new format ([theme] section)
type ThemeConfig struct {
	// Root-level fields (legacy format - default, minimal, dark themes)
	Name        string `toml:"name"`
	DisplayName string `toml:"display_name"`
	Description string `toml:"description"`
	Author      string `toml:"author"`
	Version     string `toml:"version"`
	License     string `toml:"license"`

	// Theme section (new format - professional themes)
	Theme ThemeMetadata `toml:"theme"`

	// Layout configuration
	Layout ThemeLayout `toml:"layout"`

	// Typography configuration
	Typography ThemeTypography `toml:"typography"`

	// Colors configuration
	Colors ThemeColors `toml:"colors"`

	// Variables (legacy format)
	Variables map[string]string `toml:"variables"`

	// Metadata section (some themes use this)
	Metadata ThemeMetadataSection `toml:"metadata"`

	// Assets configuration
	Assets ThemeAssets `toml:"assets"`

	// Features configuration
	Features map[string]interface{} `toml:"features"`

	// Syntax highlighting (developer themes)
	SyntaxHighlighting map[string]string `toml:"syntax_highlighting"`

	// Citation configuration (academic themes)
	Citation ThemeCitation `toml:"citation"`
}

// ThemeMetadata represents the [theme] section in theme.toml
type ThemeMetadata struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Author      string   `toml:"author"`
	Description string   `toml:"description"`
	Category    string   `toml:"category"`
	License     string   `toml:"license"`
	Homepage    string   `toml:"homepage"`
	Repository  string   `toml:"repository"`
	Tags        []string `toml:"tags"`
	Premium     bool     `toml:"premium"`
	Price       float64  `toml:"price"`
	Featured    bool     `toml:"featured"`
}

// ThemeMetadataSection represents the [metadata] section in some themes
type ThemeMetadataSection struct {
	Tags     []string `toml:"tags"`
	Homepage string   `toml:"homepage"`
	Preview  string   `toml:"preview"`
}

// ThemeLayout represents the [layout] section
type ThemeLayout struct {
	Default        string `toml:"default"`
	TitleSlide     string `toml:"title_slide"`
	SectionSlide   string `toml:"section_slide"`
	ContentSlide   string `toml:"content_slide"`
	ClosingSlide   string `toml:"closing_slide"`
	CodeSlide      string `toml:"code_slide"`
	TerminalSlide  string `toml:"terminal_slide"`
	CitationSlide  string `toml:"citation_slide"`
	ReferenceSlide string `toml:"reference_slide"`
}

// ThemeTypography represents the [typography] section
type ThemeTypography struct {
	PrimaryFont   string  `toml:"primary_font"`
	SecondaryFont string  `toml:"secondary_font"`
	MonoFont      string  `toml:"mono_font"`
	BaseSize      string  `toml:"base_size"`
	TitleScale    float64 `toml:"title_scale"`
	SubtitleScale float64 `toml:"subtitle_scale"`
	// Legacy format fields
	HeadingFont string `toml:"heading_font"`
	BodyFont    string `toml:"body_font"`
}

// ThemeColors represents the [colors] section
type ThemeColors struct {
	Primary    string `toml:"primary"`
	Secondary  string `toml:"secondary"`
	Accent     string `toml:"accent"`
	Background string `toml:"background"`
	Surface    string `toml:"surface"`
	Text       string `toml:"text"`
	TextMuted  string `toml:"text_muted"`
	Border     string `toml:"border"`
	Success    string `toml:"success"`
	Warning    string `toml:"warning"`
	Error      string `toml:"error"`
}

// ThemeAssets represents the [assets] section
type ThemeAssets struct {
	Logo              string   `toml:"logo"`
	BackgroundPattern string   `toml:"background_pattern"`
	Icons             string   `toml:"icons"`
	Watermark         string   `toml:"watermark"`
	TerminalCursor    string   `toml:"terminal_cursor"`
	Styles            []string `toml:"styles"`
	Scripts           []string `toml:"scripts"`
	Fonts             []string `toml:"fonts"`
}

// ThemeCitation represents the [citation] section for academic themes
type ThemeCitation struct {
	Style     string `toml:"style"`
	Format    string `toml:"format"`
	Numbering string `toml:"numbering"`
}

// ThemeListEntry represents a theme entry for list output
type ThemeListEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	IsDefault   bool   `json:"is_default"`
	Source      string `json:"source"` // "builtin" or "user"
}

// ThemeDetailedInfo represents detailed theme information
type ThemeDetailedInfo struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	License     string                 `json:"license"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags,omitempty"`
	Homepage    string                 `json:"homepage,omitempty"`
	Repository  string                 `json:"repository,omitempty"`
	Featured    bool                   `json:"featured"`
	IsDefault   bool                   `json:"is_default"`
	Source      string                 `json:"source"`
	Path        string                 `json:"path"`
	Colors      ThemeColors            `json:"colors,omitempty"`
	Typography  ThemeTypography        `json:"typography,omitempty"`
	Features    map[string]interface{} `json:"features,omitempty"`
}

// Flags for themes command
var (
	themesJSONOutput bool
)

var themesCmd = &cobra.Command{
	Use:   "themes",
	Short: "Manage and browse presentation themes",
	Long: `The themes command provides access to available presentation themes.
You can list all themes or get detailed information about specific themes.`,
}

var themesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available themes",
	Long: `List all themes available in the themes directory and user themes directory.
Shows name, description, and category for each theme.`,
	RunE: runThemesList,
}

var themesInfoCmd = &cobra.Command{
	Use:   "info <theme-name>",
	Short: "Show detailed information about a theme",
	Long: `Display detailed information about a specific theme including:
- Name, description, and author
- Color scheme
- Typography settings
- Available features`,
	Args: cobra.ExactArgs(1),
	RunE: runThemesInfo,
}

func init() {
	// Add themes subcommands
	themesCmd.AddCommand(themesListCmd)
	themesCmd.AddCommand(themesInfoCmd)

	// Flags for list command
	themesListCmd.Flags().BoolVar(&themesJSONOutput, "json", false, "Output in JSON format")

	// Flags for info command
	themesInfoCmd.Flags().BoolVar(&themesJSONOutput, "json", false, "Output in JSON format")

	// Add to root command
	rootCmd.AddCommand(themesCmd)
}

// getThemesDirectories returns all directories to search for themes
func getThemesDirectories() []string {
	var dirs []string

	// Built-in themes directory (relative to current directory or executable)
	dirs = append(dirs, "themes")

	// User themes directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userThemesDir := filepath.Join(homeDir, ".slicli", "themes")
		dirs = append(dirs, userThemesDir)
	}

	// XDG config directory
	if configDir := os.Getenv("XDG_CONFIG_HOME"); configDir != "" {
		xdgThemesDir := filepath.Join(configDir, "slicli", "themes")
		dirs = append(dirs, xdgThemesDir)
	}

	return dirs
}

// loadThemeConfig loads and parses a theme.toml file
func loadThemeConfig(themePath string) (*ThemeConfig, error) {
	configPath := filepath.Join(themePath, "theme.toml")

	data, err := os.ReadFile(configPath) // #nosec G304 - path validated before calling
	if err != nil {
		return nil, fmt.Errorf("reading theme.toml: %w", err)
	}

	var config ThemeConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing theme.toml: %w", err)
	}

	return &config, nil
}

// discoverThemes finds all available themes in the themes directories
func discoverThemes() ([]ThemeListEntry, error) {
	themes := make(map[string]ThemeListEntry) // Use map to deduplicate

	dirs := getThemesDirectories()

	for _, baseDir := range dirs {
		// Check if directory exists
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue // Skip directories we can't read
		}

		isBuiltIn := baseDir == "themes"

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			themeName := entry.Name()

			// Validate theme name
			if err := ValidateThemeName(themeName); err != nil {
				continue
			}

			// Skip if already found (built-in takes precedence)
			if _, exists := themes[themeName]; exists {
				continue
			}

			themePath := filepath.Join(baseDir, themeName)

			// Check for theme.toml
			configPath := filepath.Join(themePath, "theme.toml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				continue
			}

			// Load theme config
			config, err := loadThemeConfig(themePath)
			if err != nil {
				// Skip malformed themes
				continue
			}

			// Build theme entry
			entry := buildThemeListEntry(themeName, config, isBuiltIn)
			themes[themeName] = entry
		}
	}

	// Convert map to sorted slice
	result := make([]ThemeListEntry, 0, len(themes))
	for _, theme := range themes {
		result = append(result, theme)
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// buildThemeListEntry creates a ThemeListEntry from a ThemeConfig
func buildThemeListEntry(name string, config *ThemeConfig, isBuiltIn bool) ThemeListEntry {
	entry := ThemeListEntry{
		Name:      name,
		IsDefault: name == defaultThemeName,
	}

	if isBuiltIn {
		entry.Source = "builtin"
	} else {
		entry.Source = "user"
	}

	// Handle both legacy and new format
	if config.Theme.Name != "" {
		// New format ([theme] section)
		entry.DisplayName = config.Theme.Name
		entry.Description = config.Theme.Description
		entry.Category = config.Theme.Category
		entry.Version = config.Theme.Version
		entry.Author = config.Theme.Author
	} else {
		// Legacy format (root-level fields)
		entry.DisplayName = config.DisplayName
		if entry.DisplayName == "" {
			entry.DisplayName = config.Name
		}
		entry.Description = config.Description
		entry.Version = config.Version
		entry.Author = config.Author
		// Legacy themes don't have explicit category
		entry.Category = ""
	}

	// Fallbacks
	if entry.DisplayName == "" {
		entry.DisplayName = name
	}
	if entry.Version == "" {
		entry.Version = "1.0.0"
	}

	return entry
}

// findTheme locates a theme by name and returns its path and config
func findTheme(name string) (string, *ThemeConfig, error) {
	// Validate theme name first
	if err := ValidateThemeName(name); err != nil {
		return "", nil, err
	}

	dirs := getThemesDirectories()

	for _, baseDir := range dirs {
		themePath := filepath.Join(baseDir, name)

		// Check if theme directory exists
		info, err := os.Stat(themePath)
		if err != nil || !info.IsDir() {
			continue
		}

		// Check for theme.toml
		configPath := filepath.Join(themePath, "theme.toml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		// Load and return config
		config, err := loadThemeConfig(themePath)
		if err != nil {
			return "", nil, fmt.Errorf("loading theme config: %w", err)
		}

		return themePath, config, nil
	}

	return "", nil, fmt.Errorf("theme '%s' not found", name)
}

// buildDetailedInfo creates ThemeDetailedInfo from a ThemeConfig
func buildDetailedInfo(name, themePath string, config *ThemeConfig) ThemeDetailedInfo {
	info := ThemeDetailedInfo{
		Name:      name,
		Path:      themePath,
		IsDefault: name == defaultThemeName,
		Features:  config.Features,
	}

	// Determine source
	if strings.HasPrefix(themePath, "themes") {
		info.Source = "builtin"
	} else {
		info.Source = "user"
	}

	// Handle both legacy and new format
	if config.Theme.Name != "" {
		// New format
		info.DisplayName = config.Theme.Name
		info.Description = config.Theme.Description
		info.Version = config.Theme.Version
		info.Author = config.Theme.Author
		info.License = config.Theme.License
		info.Category = config.Theme.Category
		info.Tags = config.Theme.Tags
		info.Homepage = config.Theme.Homepage
		info.Repository = config.Theme.Repository
		info.Featured = config.Theme.Featured
		info.Colors = config.Colors
		info.Typography = config.Typography
	} else {
		// Legacy format
		info.DisplayName = config.DisplayName
		if info.DisplayName == "" {
			info.DisplayName = config.Name
		}
		info.Description = config.Description
		info.Version = config.Version
		info.Author = config.Author
		info.License = config.License

		// Extract colors from variables if available
		if config.Variables != nil {
			info.Colors = ThemeColors{
				Primary:    config.Variables["primary-color"],
				Background: config.Variables["background-color"],
				Text:       config.Variables["text-color"],
			}
		}

		// Extract typography from variables
		if config.Variables != nil {
			info.Typography = ThemeTypography{
				PrimaryFont: config.Variables["font-family"],
				MonoFont:    config.Variables["code-font"],
				BaseSize:    config.Variables["base-font-size"],
			}
		}

		// Get tags from metadata section if available
		if len(config.Metadata.Tags) > 0 {
			info.Tags = config.Metadata.Tags
		}
		if config.Metadata.Homepage != "" {
			info.Homepage = config.Metadata.Homepage
		}
	}

	// Fallbacks
	if info.DisplayName == "" {
		info.DisplayName = name
	}
	if info.Version == "" {
		info.Version = "1.0.0"
	}

	return info
}

func runThemesList(cmd *cobra.Command, args []string) error {
	themes, err := discoverThemes()
	if err != nil {
		return fmt.Errorf("discovering themes: %w", err)
	}

	if len(themes) == 0 {
		fmt.Println("No themes found.")
		return nil
	}

	if themesJSONOutput {
		return printThemesListJSON(themes)
	}

	return printThemesListTable(themes)
}

func runThemesInfo(cmd *cobra.Command, args []string) error {
	themeName := args[0]

	themePath, config, err := findTheme(themeName)
	if err != nil {
		return fmt.Errorf("theme not found: %w", err)
	}

	info := buildDetailedInfo(themeName, themePath, config)

	if themesJSONOutput {
		return printThemeInfoJSON(info)
	}

	return printThemeInfoTable(info)
}

func printThemesListTable(themes []ThemeListEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "NAME\tDESCRIPTION\tCATEGORY\tVERSION\tSOURCE\n")

	for _, t := range themes {
		description := t.Description
		if len(description) > 60 {
			description = description[:57] + "..."
		}

		name := t.Name
		if t.IsDefault {
			name = name + " (default)"
		}

		category := t.Category
		if category == "" {
			category = "-"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, description, category, t.Version, t.Source)
	}

	return w.Flush()
}

func printThemesListJSON(themes []ThemeListEntry) error {
	output := struct {
		Themes []ThemeListEntry `json:"themes"`
		Count  int              `json:"count"`
	}{
		Themes: themes,
		Count:  len(themes),
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func printThemeInfoTable(info ThemeDetailedInfo) error {
	fmt.Printf("Theme: %s\n", info.DisplayName)
	fmt.Printf("Name: %s\n", info.Name)
	fmt.Printf("Version: %s\n", info.Version)

	if info.Author != "" {
		fmt.Printf("Author: %s\n", info.Author)
	}

	if info.Description != "" {
		fmt.Printf("Description: %s\n", info.Description)
	}

	if info.Category != "" {
		fmt.Printf("Category: %s\n", info.Category)
	}

	if info.License != "" {
		fmt.Printf("License: %s\n", info.License)
	}

	fmt.Printf("Source: %s\n", info.Source)
	fmt.Printf("Path: %s\n", info.Path)

	if info.IsDefault {
		fmt.Println("Status: Default theme")
	}

	if info.Featured {
		fmt.Println("Featured: Yes")
	}

	if len(info.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(info.Tags, ", "))
	}

	if info.Homepage != "" {
		fmt.Printf("Homepage: %s\n", info.Homepage)
	}

	if info.Repository != "" {
		fmt.Printf("Repository: %s\n", info.Repository)
	}

	// Print colors if available
	if info.Colors.Primary != "" || info.Colors.Background != "" || info.Colors.Text != "" {
		fmt.Println("\nColor Scheme:")
		if info.Colors.Primary != "" {
			fmt.Printf("  Primary: %s\n", info.Colors.Primary)
		}
		if info.Colors.Secondary != "" {
			fmt.Printf("  Secondary: %s\n", info.Colors.Secondary)
		}
		if info.Colors.Accent != "" {
			fmt.Printf("  Accent: %s\n", info.Colors.Accent)
		}
		if info.Colors.Background != "" {
			fmt.Printf("  Background: %s\n", info.Colors.Background)
		}
		if info.Colors.Text != "" {
			fmt.Printf("  Text: %s\n", info.Colors.Text)
		}
		if info.Colors.Surface != "" {
			fmt.Printf("  Surface: %s\n", info.Colors.Surface)
		}
	}

	// Print typography if available
	if info.Typography.PrimaryFont != "" || info.Typography.MonoFont != "" {
		fmt.Println("\nTypography:")
		if info.Typography.PrimaryFont != "" {
			fmt.Printf("  Primary Font: %s\n", info.Typography.PrimaryFont)
		}
		if info.Typography.SecondaryFont != "" {
			fmt.Printf("  Secondary Font: %s\n", info.Typography.SecondaryFont)
		}
		if info.Typography.MonoFont != "" {
			fmt.Printf("  Monospace Font: %s\n", info.Typography.MonoFont)
		}
		if info.Typography.BaseSize != "" {
			fmt.Printf("  Base Size: %s\n", info.Typography.BaseSize)
		}
		// Handle legacy format
		if info.Typography.HeadingFont != "" {
			fmt.Printf("  Heading Font: %s\n", info.Typography.HeadingFont)
		}
		if info.Typography.BodyFont != "" {
			fmt.Printf("  Body Font: %s\n", info.Typography.BodyFont)
		}
	}

	// Print features if available
	if len(info.Features) > 0 {
		fmt.Println("\nFeatures:")
		for feature, value := range info.Features {
			switch v := value.(type) {
			case bool:
				if v {
					fmt.Printf("  %s: enabled\n", formatFeatureName(feature))
				}
			case string:
				fmt.Printf("  %s: %s\n", formatFeatureName(feature), v)
			default:
				fmt.Printf("  %s: %v\n", formatFeatureName(feature), v)
			}
		}
	}

	return nil
}

func printThemeInfoJSON(info ThemeDetailedInfo) error {
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// formatFeatureName converts snake_case feature names to Title Case
func formatFeatureName(name string) string {
	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Title case each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}
