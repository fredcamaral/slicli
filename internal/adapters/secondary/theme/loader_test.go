package theme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestTheme(t *testing.T) string {
	tmpDir := t.TempDir()
	themeDir := filepath.Join(tmpDir, "test-theme")

	// Create theme structure
	require.NoError(t, os.MkdirAll(filepath.Join(themeDir, "templates"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(themeDir, "assets", "css"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(themeDir, "assets", "js"), 0755))

	// Create theme.toml
	themeConfig := `display_name = "Test Theme"
description = "Test theme for unit tests"
author = "Test Author"
version = "1.0.0"

[variables]
primary-color = "#000"
font-family = "Arial"

[transitions]
type = "fade"
duration = 300

[features]
slide-numbers = true
progress-bar = true
`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "theme.toml"),
		[]byte(themeConfig),
		0644,
	))

	// Create templates
	presentationTemplate := `<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>{{.Content}}</body>
</html>`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "templates", "presentation.html"),
		[]byte(presentationTemplate),
		0644,
	))

	slideTemplate := `<div class="slide">{{.Content}}</div>`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "templates", "slide.html"),
		[]byte(slideTemplate),
		0644,
	))

	notesTemplate := `<div class="notes">{{.Notes}}</div>`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "templates", "notes.html"),
		[]byte(notesTemplate),
		0644,
	))

	// Create assets
	cssContent := `body { color: var(--primary-color); }`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "assets", "css", "main.css"),
		[]byte(cssContent),
		0644,
	))

	jsContent := `console.log('test theme');`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "assets", "js", "theme.js"),
		[]byte(jsContent),
		0644,
	))

	return tmpDir
}

func TestDirectoryLoader_Load(t *testing.T) {
	tmpDir := setupTestTheme(t)
	loader := NewDirectoryLoader(tmpDir)

	ctx := context.Background()
	theme, err := loader.Load(ctx, "test-theme")
	require.NoError(t, err)
	require.NotNil(t, theme)

	// Check basic properties
	assert.Equal(t, "test-theme", theme.Name)
	assert.Equal(t, filepath.Join(tmpDir, "test-theme"), theme.Path)
	// Config fields are now in Variables map
	assert.Equal(t, "#000", theme.Config.Variables["primary-color"])

	// Check variables
	assert.Equal(t, "#000", theme.Config.Variables["primary-color"])
	assert.Equal(t, "Arial", theme.Config.Variables["font-family"])

	// Check templates
	assert.Len(t, theme.Templates, 3)
	assert.NotNil(t, theme.Templates["presentation"])
	assert.NotNil(t, theme.Templates["slide"])
	assert.NotNil(t, theme.Templates["notes"])

	// Check assets
	assert.Len(t, theme.Assets, 2)
	assert.NotNil(t, theme.Assets["css/main.css"])
	assert.NotNil(t, theme.Assets["js/theme.js"])
	assert.Equal(t, "text/css", theme.Assets["css/main.css"].ContentType)
	assert.Equal(t, "application/javascript", theme.Assets["js/theme.js"].ContentType)
}

func TestDirectoryLoader_LoadWithParent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create parent theme
	parentDir := filepath.Join(tmpDir, "parent-theme")
	require.NoError(t, os.MkdirAll(filepath.Join(parentDir, "templates"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(parentDir, "assets", "css"), 0755))

	parentConfig := `display_name = "Parent Theme"
version = "1.0.0"

[variables]
primary-color = "#000"
secondary-color = "#333"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "theme.toml"),
		[]byte(parentConfig),
		0644,
	))

	// Parent templates
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "templates", "presentation.html"),
		[]byte(`<html>parent presentation</html>`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "templates", "slide.html"),
		[]byte(`<div>parent slide</div>`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "templates", "header.html"),
		[]byte(`<header>parent header</header>`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "templates", "notes.html"),
		[]byte(`<div>parent notes</div>`),
		0644,
	))

	// Parent assets
	require.NoError(t, os.WriteFile(
		filepath.Join(parentDir, "assets", "css", "main.css"),
		[]byte(`body { color: #000; }`),
		0644,
	))

	// Create child theme
	childDir := filepath.Join(tmpDir, "child-theme")
	require.NoError(t, os.MkdirAll(filepath.Join(childDir, "templates"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(childDir, "assets", "css"), 0755))

	childConfig := `display_name = "Child Theme"
version = "1.0.0"
parent = "parent-theme"

[variables]
primary-color = "#fff"
accent-color = "#f00"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(childDir, "theme.toml"),
		[]byte(childConfig),
		0644,
	))

	// Child overrides only presentation template
	require.NoError(t, os.WriteFile(
		filepath.Join(childDir, "templates", "presentation.html"),
		[]byte(`<html>child presentation</html>`),
		0644,
	))

	loader := NewDirectoryLoader(tmpDir)
	ctx := context.Background()
	theme, err := loader.Load(ctx, "child-theme")
	require.NoError(t, err)
	require.NotNil(t, theme)

	// Check inheritance
	assert.Equal(t, "child-theme", theme.Name)
	assert.Equal(t, "parent-theme", theme.Parent)

	// Check templates (should have all four)
	assert.Len(t, theme.Templates, 4)
	assert.NotNil(t, theme.Templates["presentation"]) // From child
	assert.NotNil(t, theme.Templates["slide"])        // From parent
	assert.NotNil(t, theme.Templates["header"])       // From parent
	assert.NotNil(t, theme.Templates["notes"])        // From parent

	// Check variables (merged)
	assert.Equal(t, "#fff", theme.Config.Variables["primary-color"])   // Overridden
	assert.Equal(t, "#333", theme.Config.Variables["secondary-color"]) // Inherited
	assert.Equal(t, "#f00", theme.Config.Variables["accent-color"])    // New in child
}

func TestDirectoryLoader_List(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple themes
	themes := []string{"theme1", "theme2", "theme3"}
	for i, name := range themes {
		themeDir := filepath.Join(tmpDir, name)
		require.NoError(t, os.MkdirAll(themeDir, 0755))

		config := `display_name = "Theme %d"
description = "Test theme %d"
version = "1.0.0"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(themeDir, "theme.toml"),
			[]byte(fmt.Sprintf(config, i+1, i+1)),
			0644,
		))
	}

	// Create a non-theme directory (should be ignored)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "not-a-theme"), 0755))

	loader := NewDirectoryLoader(tmpDir)
	ctx := context.Background()
	themeList, err := loader.List(ctx)
	require.NoError(t, err)

	assert.Len(t, themeList, 3)

	// Check theme info
	themeNames := make([]string, len(themeList))
	for i, info := range themeList {
		themeNames[i] = info.Name
		assert.NotEmpty(t, info.DisplayName)
		// Version should default to "1.0.0"
		assert.Equal(t, "1.0.0", info.Version)
	}

	assert.ElementsMatch(t, themes, themeNames)
}

func TestDirectoryLoader_Exists(t *testing.T) {
	tmpDir := setupTestTheme(t)
	loader := NewDirectoryLoader(tmpDir)

	ctx := context.Background()
	assert.True(t, loader.Exists(ctx, "test-theme"))
	assert.False(t, loader.Exists(ctx, "non-existent-theme"))
}

func TestDirectoryLoader_Reload(t *testing.T) {
	tmpDir := setupTestTheme(t)
	loader := NewDirectoryLoader(tmpDir)

	ctx := context.Background()

	// Load theme first time
	theme1, err := loader.Load(ctx, "test-theme")
	require.NoError(t, err)
	loadTime1 := theme1.LoadedAt

	// Modify theme file
	themeDir := filepath.Join(tmpDir, "test-theme")
	newCSS := `body { color: blue; }`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "assets", "css", "main.css"),
		[]byte(newCSS),
		0644,
	))

	// Reload theme
	theme2, err := loader.Reload(ctx, "test-theme")
	require.NoError(t, err)

	// Check that it was actually reloaded
	assert.True(t, theme2.LoadedAt.After(loadTime1))
	assert.Equal(t, newCSS, string(theme2.Assets["css/main.css"].Content))
}

func TestDirectoryLoader_InvalidTheme(t *testing.T) {
	tmpDir := t.TempDir()

	// Create theme without required files
	themeDir := filepath.Join(tmpDir, "invalid-theme")
	require.NoError(t, os.MkdirAll(themeDir, 0755))

	// No theme.toml
	loader := NewDirectoryLoader(tmpDir)
	ctx := context.Background()
	_, err := loader.Load(ctx, "invalid-theme")
	assert.Error(t, err)
	// Error could be about theme.toml or templates directory

	// Create invalid theme.toml
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "theme.toml"),
		[]byte("invalid toml content"),
		0644,
	))
	_, err = loader.Load(ctx, "invalid-theme")
	assert.Error(t, err)

	// Create valid theme.toml but missing templates
	validConfig := `display_name = "Invalid Theme"
version = "1.0.0"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeDir, "theme.toml"),
		[]byte(validConfig),
		0644,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(themeDir, "templates"), 0755))

	_, err = loader.Load(ctx, "invalid-theme")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template")
}

func TestDirectoryLoader_CircularParentReference(t *testing.T) {
	tmpDir := t.TempDir()

	// Create theme A that has B as parent
	themeADir := filepath.Join(tmpDir, "theme-a")
	require.NoError(t, os.MkdirAll(filepath.Join(themeADir, "templates"), 0755))
	configA := `display_name = "Theme A"
version = "1.0.0"
parent = "theme-b"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeADir, "theme.toml"),
		[]byte(configA),
		0644,
	))

	// Create templates for theme A
	require.NoError(t, os.WriteFile(
		filepath.Join(themeADir, "templates", "presentation.html"),
		[]byte(`<html>A</html>`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(themeADir, "templates", "slide.html"),
		[]byte(`<div>A</div>`),
		0644,
	))

	// Create theme B that has A as parent (circular)
	themeBDir := filepath.Join(tmpDir, "theme-b")
	require.NoError(t, os.MkdirAll(filepath.Join(themeBDir, "templates"), 0755))
	configB := `display_name = "Theme B"
version = "1.0.0"
parent = "theme-a"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(themeBDir, "theme.toml"),
		[]byte(configB),
		0644,
	))

	// Create templates for theme B
	require.NoError(t, os.WriteFile(
		filepath.Join(themeBDir, "templates", "presentation.html"),
		[]byte(`<html>B</html>`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(themeBDir, "templates", "slide.html"),
		[]byte(`<div>B</div>`),
		0644,
	))

	loader := NewDirectoryLoader(tmpDir)
	ctx := context.Background()

	// Loading either theme should fail due to circular reference
	_, err := loader.Load(ctx, "theme-a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}
