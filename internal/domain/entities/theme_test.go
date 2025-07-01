package entities

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheme_Validate(t *testing.T) {
	tests := []struct {
		name    string
		theme   Theme
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid theme",
			theme: Theme{
				Name:          "my-theme",
				DisplayName:   "My Theme",
				Version:       "1.0.0",
				TemplatesPath: "templates",
				AssetsPath:    "assets",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			theme: Theme{
				DisplayName: "My Theme",
				Version:     "1.0.0",
			},
			wantErr: true,
			errMsg:  "theme name is required",
		},
		{
			name: "invalid name with uppercase",
			theme: Theme{
				Name: "MyTheme",
			},
			wantErr: true,
			errMsg:  "theme name must contain only lowercase letters, numbers, and hyphens",
		},
		{
			name: "invalid name with spaces",
			theme: Theme{
				Name: "my theme",
			},
			wantErr: true,
			errMsg:  "theme name must contain only lowercase letters, numbers, and hyphens",
		},
		{
			name: "invalid name starting with hyphen",
			theme: Theme{
				Name: "-theme",
			},
			wantErr: true,
			errMsg:  "theme name must contain only lowercase letters, numbers, and hyphens",
		},
		{
			name: "invalid name ending with hyphen",
			theme: Theme{
				Name: "theme-",
			},
			wantErr: true,
			errMsg:  "theme name must contain only lowercase letters, numbers, and hyphens",
		},
		{
			name: "absolute templates path",
			theme: Theme{
				Name:          "valid",
				TemplatesPath: "/absolute/path",
			},
			wantErr: true,
			errMsg:  "templates path must be relative",
		},
		{
			name: "absolute assets path",
			theme: Theme{
				Name:       "valid",
				AssetsPath: "/absolute/path",
			},
			wantErr: true,
			errMsg:  "assets path must be relative",
		},
		{
			name: "defaults applied",
			theme: Theme{
				Name: "test-theme",
				// DisplayName and Version should be set to defaults
			},
			wantErr: false,
		},
		{
			name: "valid complex name",
			theme: Theme{
				Name:        "theme-2024-v3",
				DisplayName: "Theme 2024 v3",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := tt.theme // Make a copy
			err := theme.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Check defaults were applied
				if tt.name == "defaults applied" {
					assert.Equal(t, "test-theme", theme.DisplayName)
					assert.Equal(t, "1.0.0", theme.Version)
				}
			}
		})
	}
}

func TestTheme_GetTemplatePath(t *testing.T) {
	tests := []struct {
		name         string
		theme        Theme
		templateName string
		want         string
	}{
		{
			name: "with custom templates path",
			theme: Theme{
				Name:          "custom",
				TemplatesPath: "custom/templates",
			},
			templateName: "slide.html",
			want:         filepath.Join("custom/templates", "slide.html"),
		},
		{
			name: "without templates path",
			theme: Theme{
				Name: "default",
			},
			templateName: "slide.html",
			want:         filepath.Join("themes", "default", "templates", "slide.html"),
		},
		{
			name: "nested template",
			theme: Theme{
				Name:          "modern",
				TemplatesPath: "modern/tmpl",
			},
			templateName: "partials/header.html",
			want:         filepath.Join("modern/tmpl", "partials/header.html"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.theme.GetTemplatePath(tt.templateName)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTheme_GetAssetPath(t *testing.T) {
	tests := []struct {
		name      string
		theme     Theme
		assetName string
		want      string
	}{
		{
			name: "with custom assets path",
			theme: Theme{
				Name:       "custom",
				AssetsPath: "custom/static",
			},
			assetName: "main.css",
			want:      filepath.Join("custom/static", "main.css"),
		},
		{
			name: "without assets path",
			theme: Theme{
				Name: "default",
			},
			assetName: "main.css",
			want:      filepath.Join("themes", "default", "assets", "main.css"),
		},
		{
			name: "nested asset",
			theme: Theme{
				Name:       "modern",
				AssetsPath: "modern/public",
			},
			assetName: "css/styles.css",
			want:      filepath.Join("modern/public", "css/styles.css"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.theme.GetAssetPath(tt.assetName)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTheme_IsBuiltIn(t *testing.T) {
	tests := []struct {
		name  string
		theme Theme
		want  bool
	}{
		{
			name:  "default theme",
			theme: Theme{Name: "default"},
			want:  true,
		},
		{
			name:  "minimal theme",
			theme: Theme{Name: "minimal"},
			want:  true,
		},
		{
			name:  "dark theme",
			theme: Theme{Name: "dark"},
			want:  true,
		},
		{
			name:  "custom theme",
			theme: Theme{Name: "custom"},
			want:  false,
		},
		{
			name:  "theme with similar name",
			theme: Theme{Name: "default-custom"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.theme.IsBuiltIn())
		})
	}
}

func TestIsValidThemeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "theme", true},
		{"valid with numbers", "theme123", true},
		{"valid with hyphens", "my-theme-2", true},
		{"empty", "", false},
		{"uppercase", "Theme", false},
		{"with spaces", "my theme", false},
		{"with underscore", "my_theme", false},
		{"starts with hyphen", "-theme", false},
		{"ends with hyphen", "theme-", false},
		{"special chars", "theme!", false},
		{"numbers only", "123", true},
		{"single char", "a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidThemeName(tt.input))
		})
	}
}

func TestTheme_CompleteStruct(t *testing.T) {
	theme := &Theme{
		Name:          "advanced-theme",
		DisplayName:   "Advanced Theme",
		Description:   "A feature-rich presentation theme",
		Author:        "Test Author",
		Version:       "2.1.0",
		TemplatesPath: "themes/advanced/templates",
		AssetsPath:    "themes/advanced/assets",
		Config: map[string]interface{}{
			"colors": map[string]string{
				"primary":   "#007bff",
				"secondary": "#6c757d",
			},
			"fonts": []string{"Roboto", "Arial"},
		},
	}

	// Test all fields
	assert.Equal(t, "advanced-theme", theme.Name)
	assert.Equal(t, "Advanced Theme", theme.DisplayName)
	assert.Equal(t, "A feature-rich presentation theme", theme.Description)
	assert.Equal(t, "Test Author", theme.Author)
	assert.Equal(t, "2.1.0", theme.Version)
	assert.Equal(t, "themes/advanced/templates", theme.TemplatesPath)
	assert.Equal(t, "themes/advanced/assets", theme.AssetsPath)

	// Check config
	colors := theme.Config["colors"].(map[string]string)
	assert.Equal(t, "#007bff", colors["primary"])
	fonts := theme.Config["fonts"].([]string)
	assert.Contains(t, fonts, "Roboto")
}
