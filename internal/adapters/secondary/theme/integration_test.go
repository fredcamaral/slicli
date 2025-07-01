//go:build integration
// +build integration

package theme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) (string, *services.ThemeService) {
	tmpDir := t.TempDir()

	// Create a complete theme structure
	defaultTheme := filepath.Join(tmpDir, "default")
	require.NoError(t, os.MkdirAll(filepath.Join(defaultTheme, "templates"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(defaultTheme, "assets", "css"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(defaultTheme, "assets", "js"), 0755))

	// Theme configuration
	themeConfig := `display_name = "Default Theme"
description = "Default theme for testing"
author = "Test"
version = "1.0.0"

[variables]
primary-color = "#2563eb"
secondary-color = "#64748b"
background-color = "#ffffff"
text-color = "#1e293b"
font-family = "Arial, sans-serif"

[transitions]
type = "fade"
duration = 300
easing = "ease-in-out"

[features]
syntax-highlighting = true
speaker-notes = true
progress-bar = true
slide-numbers = true
`
	require.NoError(t, os.WriteFile(
		filepath.Join(defaultTheme, "theme.toml"),
		[]byte(themeConfig),
		0644,
	))

	// Presentation template
	presentationTemplate := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Presentation.Title}}</title>
    <style>
        :root {
            {{range $key, $value := .ThemeConfig.Variables}}
            --{{$key}}: {{$value}};
            {{end}}
        }
    </style>
    <link rel="stylesheet" href="/assets/css/main.css">
</head>
<body>
    <div class="presentation">
        <h1>{{.Presentation.Title}}</h1>
        <p>by {{.Presentation.Author}}</p>
        <div class="slides">{{.SlidesHTML}}</div>
    </div>
    <script src="/assets/js/theme.js"></script>
</body>
</html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(defaultTheme, "templates", "presentation.html"),
		[]byte(presentationTemplate),
		0644,
	))

	// Slide template
	slideTemplate := `<div class="slide" data-slide="{{.SlideNumber}}">
    <div class="slide-content">
        {{.Slide.HTML | safeHTML}}
    </div>
    {{if .Slide.Notes}}
    <div class="speaker-notes">{{.Slide.Notes}}</div>
    {{end}}
</div>`
	require.NoError(t, os.WriteFile(
		filepath.Join(defaultTheme, "templates", "slide.html"),
		[]byte(slideTemplate),
		0644,
	))

	// CSS asset
	cssContent := `body {
    font-family: var(--font-family);
    color: var(--text-color);
    background-color: var(--background-color);
}

.slide {
    padding: 2rem;
    max-width: 1200px;
    margin: 0 auto;
}

h1 { color: var(--primary-color); }
h2 { color: var(--secondary-color); }`
	require.NoError(t, os.WriteFile(
		filepath.Join(defaultTheme, "assets", "css", "main.css"),
		[]byte(cssContent),
		0644,
	))

	// JS asset
	jsContent := `console.log('Theme loaded: {{theme-name}}');
const config = {
    primaryColor: '{{primary-color}}',
    transitions: {{transition-duration}}
};`
	require.NoError(t, os.WriteFile(
		filepath.Join(defaultTheme, "assets", "js", "theme.js"),
		[]byte(jsContent),
		0644,
	))

	// Create service with real implementations
	loader := NewDirectoryLoader(tmpDir)
	cache := NewMemoryCache(10, 1*time.Hour)
	processor := NewAssetProcessor()
	service := services.NewThemeService(loader, cache, processor)

	return tmpDir, service
}

func TestIntegration_CompleteThemeWorkflow(t *testing.T) {
	_, service := setupIntegrationTest(t)
	ctx := context.Background()

	// Test loading theme
	theme, err := service.LoadTheme(ctx, "default")
	require.NoError(t, err)
	assert.Equal(t, "default", theme.Name)
	assert.Equal(t, "Default Theme", theme.Config.DisplayName)

	// Test rendering presentation
	presentation := &entities.Presentation{
		Title:  "Test Presentation",
		Author: "Test Author",
		Slides: []entities.Slide{
			{
				ID:      "1",
				Content: "# Slide 1\n\nThis is the first slide",
				HTML:    "<h1>Slide 1</h1>\n<p>This is the first slide</p>",
			},
			{
				ID:      "2",
				Content: "# Slide 2\n\nThis is the second slide",
				HTML:    "<h1>Slide 2</h1>\n<p>This is the second slide</p>",
				Notes:   "Speaker notes for slide 2",
			},
		},
	}

	html, err := service.RenderPresentation(theme, presentation)
	require.NoError(t, err)

	// Verify rendered content
	htmlStr := string(html)
	assert.Contains(t, htmlStr, "Test Presentation")
	assert.Contains(t, htmlStr, "Test Author")
	assert.Contains(t, htmlStr, "--primary-color: #2563eb")
	assert.Contains(t, htmlStr, `<link rel="stylesheet" href="/assets/css/main.css">`)
	assert.Contains(t, htmlStr, `<script src="/assets/js/theme.js"></script>`)

	// Test rendering individual slide
	slideHTML, err := service.RenderSlide(theme, &presentation.Slides[1], 2, 2)
	require.NoError(t, err)

	slideStr := string(slideHTML)
	assert.Contains(t, slideStr, `data-slide="2"`)
	assert.Contains(t, slideStr, "Slide 2")
	assert.Contains(t, slideStr, "Speaker notes for slide 2")

	// Test serving CSS asset
	cssAsset, err := service.ServeAsset(ctx, "default", "css/main.css")
	require.NoError(t, err)
	assert.Equal(t, "text/css", cssAsset.ContentType)

	// Verify CSS processing
	cssStr := string(cssAsset.Content)
	assert.Contains(t, cssStr, "font-family: Arial, sans-serif")
	assert.Contains(t, cssStr, "color: #1e293b")
	assert.Contains(t, cssStr, "background-color: #ffffff")
	assert.Contains(t, cssStr, "color: #2563eb") // primary-color
	assert.NotContains(t, cssStr, "var(--")      // Variables should be replaced

	// Test serving JS asset
	jsAsset, err := service.ServeAsset(ctx, "default", "js/theme.js")
	require.NoError(t, err)
	assert.Equal(t, "application/javascript", jsAsset.ContentType)

	// Verify JS processing
	jsStr := string(jsAsset.Content)
	assert.Contains(t, jsStr, "primaryColor: '#2563eb'")
	assert.Contains(t, jsStr, "transitions: 300")
	assert.NotContains(t, jsStr, "{{") // Template variables should be replaced
}

func TestIntegration_ThemeInheritance(t *testing.T) {
	tmpDir, service := setupIntegrationTest(t)
	ctx := context.Background()

	// Create a child theme
	darkTheme := filepath.Join(tmpDir, "dark")
	require.NoError(t, os.MkdirAll(filepath.Join(darkTheme, "templates"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(darkTheme, "assets", "css"), 0755))

	// Child theme config with parent
	childConfig := `display_name = "Dark Theme"
version = "1.0.0"
parent = "default"

[variables]
primary-color = "#60a5fa"
background-color = "#0f172a"
text-color = "#e2e8f0"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(darkTheme, "theme.toml"),
		[]byte(childConfig),
		0644,
	))

	// Override only the presentation template
	darkPresentationTemplate := `<!DOCTYPE html>
<html class="dark-theme">
<head>
    <title>{{.Presentation.Title}} - Dark Theme</title>
    <style>
        :root {
            {{range $key, $value := .ThemeConfig.Variables}}
            --{{$key}}: {{$value}};
            {{end}}
        }
    </style>
    <link rel="stylesheet" href="/assets/css/main.css">
    <link rel="stylesheet" href="/assets/css/dark.css">
</head>
<body>
    <div class="presentation dark">
        <h1>{{.Presentation.Title}}</h1>
        <div class="slides">{{.SlidesHTML}}</div>
    </div>
</body>
</html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(darkTheme, "templates", "presentation.html"),
		[]byte(darkPresentationTemplate),
		0644,
	))

	// Additional dark theme CSS
	darkCSS := `.dark-theme {
    filter: invert(1);
}

.dark h1 {
    text-shadow: 0 0 10px var(--primary-color);
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(darkTheme, "assets", "css", "dark.css"),
		[]byte(darkCSS),
		0644,
	))

	// Load child theme
	theme, err := service.LoadTheme(ctx, "dark")
	require.NoError(t, err)
	assert.Equal(t, "dark", theme.Name)
	assert.Equal(t, "default", theme.Parent)

	// Verify inheritance
	assert.NotNil(t, theme.Templates["presentation"]) // From child
	assert.NotNil(t, theme.Templates["slide"])        // From parent
	assert.NotNil(t, theme.Assets["css/main.css"])    // From parent
	assert.NotNil(t, theme.Assets["css/dark.css"])    // From child

	// Verify variable merging
	assert.Equal(t, "#60a5fa", theme.Config.Variables["primary-color"])         // Overridden
	assert.Equal(t, "#0f172a", theme.Config.Variables["background-color"])      // Overridden
	assert.Equal(t, "#e2e8f0", theme.Config.Variables["text-color"])            // Overridden
	assert.Equal(t, "#64748b", theme.Config.Variables["secondary-color"])       // Inherited
	assert.Equal(t, "Arial, sans-serif", theme.Config.Variables["font-family"]) // Inherited

	// Test rendering with inherited theme
	presentation := &entities.Presentation{
		Title: "Dark Theme Test",
		Slides: []entities.Slide{
			{ID: "1", Content: "Test", HTML: "<p>Test</p>"},
		},
	}

	html, err := service.RenderPresentation(theme, presentation)
	require.NoError(t, err)

	htmlStr := string(html)
	assert.Contains(t, htmlStr, "Dark Theme Test - Dark Theme")
	assert.Contains(t, htmlStr, `class="dark-theme"`)
	assert.Contains(t, htmlStr, `class="presentation dark"`)
	assert.Contains(t, htmlStr, `<link rel="stylesheet" href="/assets/css/dark.css">`)
}

func TestIntegration_CacheAndReload(t *testing.T) {
	tmpDir, service := setupIntegrationTest(t)
	ctx := context.Background()

	// Load theme first time
	theme1, err := service.LoadTheme(ctx, "default")
	require.NoError(t, err)

	// Check cache stats
	stats := service.CacheStats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 0, stats.Hits)

	// Load again - should come from cache
	theme2, err := service.LoadTheme(ctx, "default")
	require.NoError(t, err)
	assert.Equal(t, theme1.LoadedAt, theme2.LoadedAt) // Same instance

	stats = service.CacheStats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 1, stats.Hits)

	// Modify theme file
	cssPath := filepath.Join(tmpDir, "default", "assets", "css", "main.css")
	newCSS := `body { background: red; }`
	require.NoError(t, os.WriteFile(cssPath, []byte(newCSS), 0644))

	// Reload theme
	err = service.ReloadTheme(ctx, "default")
	require.NoError(t, err)

	// Verify reload worked
	theme3, err := service.LoadTheme(ctx, "default")
	require.NoError(t, err)
	assert.True(t, theme3.LoadedAt.After(theme1.LoadedAt))

	// Check asset was reloaded
	asset, err := service.ServeAsset(ctx, "default", "css/main.css")
	require.NoError(t, err)
	assert.Equal(t, newCSS, string(asset.Content))
}

func TestIntegration_ThemeValidation(t *testing.T) {
	tmpDir, service := setupIntegrationTest(t)
	ctx := context.Background()

	// Create invalid theme (missing required template)
	invalidTheme := filepath.Join(tmpDir, "invalid")
	require.NoError(t, os.MkdirAll(filepath.Join(invalidTheme, "templates"), 0755))

	invalidConfig := `display_name = "Invalid Theme"
version = "1.0.0"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(invalidTheme, "theme.toml"),
		[]byte(invalidConfig),
		0644,
	))

	// Only create presentation template, missing slide template
	require.NoError(t, os.WriteFile(
		filepath.Join(invalidTheme, "templates", "presentation.html"),
		[]byte(`<html></html>`),
		0644,
	))

	// Should fail to load
	_, err := service.LoadTheme(ctx, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slide")

	// Validation should also fail
	err = service.ValidateTheme(ctx, "invalid")
	assert.Error(t, err)
}

func TestIntegration_ListThemes(t *testing.T) {
	tmpDir, service := setupIntegrationTest(t)
	ctx := context.Background()

	// Create additional themes
	themes := []string{"minimal", "corporate", "academic"}
	for _, name := range themes {
		themeDir := filepath.Join(tmpDir, name)
		require.NoError(t, os.MkdirAll(themeDir, 0755))

		config := `display_name = "%s Theme"
version = "1.0.0"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(themeDir, "theme.toml"),
			[]byte(fmt.Sprintf(config, name)),
			0644,
		))
	}

	// List all themes
	themeList, err := service.ListThemes(ctx)
	require.NoError(t, err)
	assert.Len(t, themeList, 4) // default + 3 new ones

	// Verify theme info
	themeNames := make(map[string]bool)
	for _, info := range themeList {
		themeNames[info.Name] = true
		assert.NotEmpty(t, info.DisplayName)
		assert.NotEmpty(t, info.Path)
	}

	assert.True(t, themeNames["default"])
	assert.True(t, themeNames["minimal"])
	assert.True(t, themeNames["corporate"])
	assert.True(t, themeNames["academic"])
}
