package entities

import (
	"html/template"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThemeEngine_Validate(t *testing.T) {
	tests := []struct {
		name    string
		theme   *ThemeEngine
		wantErr bool
	}{
		{
			name: "valid theme",
			theme: &ThemeEngine{
				Name: "test",
				Path: "/themes/test",
				Templates: map[string]*template.Template{
					"presentation": template.New("presentation"),
					"slide":        template.New("slide"),
					"notes":        template.New("notes"),
				},
				Assets: map[string]*ThemeAsset{
					"css/main.css": {
						Path:        "/themes/test/assets/css/main.css",
						Content:     []byte("body { color: red; }"),
						ContentType: "text/css",
					},
				},
				Config: ThemeEngineConfig{
					Variables: map[string]string{
						"primary-color": "red",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			theme: &ThemeEngine{
				Path: "/themes/test",
			},
			wantErr: true,
		},
		{
			name: "missing required template",
			theme: &ThemeEngine{
				Name: "test",
				Path: "/themes/test",
				Templates: map[string]*template.Template{
					"presentation": template.New("presentation"),
					"slide":        template.New("slide"),
					// missing "notes" template
				},
				Assets: map[string]*ThemeAsset{
					"css/main.css": {
						Path:        "/themes/test/assets/css/main.css",
						Content:     []byte("body { color: red; }"),
						ContentType: "text/css",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.theme.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

/*
func TestThemeEngine_Merge(t *testing.T) {
	parent := &ThemeEngine{
		Name: "parent",
		Path: "/themes/parent",
		Templates: map[string]*template.Template{
			"presentation": template.New("parent-presentation"),
			"slide":        template.New("parent-slide"),
			"header":       template.New("parent-header"),
		},
		Assets: map[string]*ThemeAsset{
			"css/main.css": {
				Path:    "/themes/parent/assets/css/main.css",
				Content: []byte("// parent css"),
			},
			"js/main.js": {
				Path:    "/themes/parent/assets/js/main.js",
				Content: []byte("// parent js"),
			},
		},
		Config: ThemeEngineConfig{
			Variables: map[string]string{
				"primary-color":   "#000",
				"secondary-color": "#333",
			},
		},
	}

	child := &ThemeEngine{
		Name: "child",
		Path: "/themes/child",
		Templates: map[string]*template.Template{
			"presentation": template.New("child-presentation"),
		},
		Assets: map[string]*ThemeAsset{
			"css/main.css": {
				Path:    "/themes/child/assets/css/main.css",
				Content: []byte("// child css"),
			},
			"css/custom.css": {
				Path:    "/themes/child/assets/css/custom.css",
				Content: []byte("// custom css"),
			},
		},
		Config: ThemeEngineConfig{
			Variables: map[string]string{
				"primary-color": "#fff",
				"accent-color":  "#f00",
			},
		},
	}

	// child.Merge(parent) - Method doesn't exist yet

	// Check templates
	assert.Len(t, child.Templates, 3)
	assert.NotNil(t, child.Templates["presentation"])
	assert.NotNil(t, child.Templates["slide"])
	assert.NotNil(t, child.Templates["header"])

	// Check assets
	assert.Len(t, child.Assets, 3)
	assert.Equal(t, "// child css", string(child.Assets["css/main.css"].Content))
	assert.NotNil(t, child.Assets["js/main.js"])
	assert.NotNil(t, child.Assets["css/custom.css"])

	// Check variables
	assert.Equal(t, "#fff", child.Config.Variables["primary-color"])
	assert.Equal(t, "#333", child.Config.Variables["secondary-color"])
	assert.Equal(t, "#f00", child.Config.Variables["accent-color"])
}
*/

func TestThemeAsset_ComputeHash(t *testing.T) {
	asset := &ThemeAsset{
		Path:        "/test/asset.css",
		Content:     []byte("body { color: red; }"),
		ContentType: "text/css",
	}

	asset.ComputeHash()
	assert.NotEmpty(t, asset.Hash)
	assert.Len(t, asset.Hash, 64) // SHA256 hash in hex is 64 chars

	// Same content should produce same hash
	firstHash := asset.Hash
	asset.ComputeHash()
	assert.Equal(t, firstHash, asset.Hash)

	// Different content should produce different hash
	asset.Content = []byte("body { color: blue; }")
	asset.ComputeHash()
	assert.NotEqual(t, firstHash, asset.Hash)
}

func TestThemeAsset_ETag(t *testing.T) {
	asset := &ThemeAsset{
		Path:        "/test/asset.css",
		Content:     []byte("body { color: red; }"),
		ContentType: "text/css",
		ModTime:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Hash:        "abc123",
	}

	etag := asset.GetETag()
	assert.Equal(t, `"abc123"`, etag)

	// Without hash - it should compute it
	asset.Hash = ""
	etag2 := asset.GetETag()
	assert.NotEmpty(t, etag2)
	assert.Regexp(t, `^"[a-f0-9]+"$`, etag2)
}

func TestThemeAsset_detectContentType(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
	}{
		{"/assets/css/main.css", "text/css"},
		{"/assets/js/app.js", "application/javascript"},
		{"/assets/img/logo.png", "image/png"},
		{"/assets/img/photo.jpg", "image/jpeg"},
		{"/assets/img/photo.jpeg", "image/jpeg"},
		{"/assets/img/icon.svg", "image/svg+xml"},
		{"/assets/img/icon.ico", "application/octet-stream"},
		{"/assets/fonts/font.woff", "font/woff"},
		{"/assets/fonts/font.woff2", "font/woff2"},
		{"/assets/data.json", "application/octet-stream"},
		{"/assets/unknown.xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			contentType := GetContentType(tt.path)
			assert.Equal(t, tt.contentType, contentType)
		})
	}
}

func TestThemeEngineConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ThemeEngineConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ThemeEngineConfig{
				Variables: map[string]string{
					"primary-color": "#000",
				},
				Transitions: TransitionConfig{
					Type:     "fade",
					Duration: 300,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid transition type",
			config: ThemeEngineConfig{
				Variables: map[string]string{
					"primary-color": "#000",
				},
				Transitions: TransitionConfig{
					Type:     "invalid",
					Duration: 300,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid transition duration",
			config: ThemeEngineConfig{
				Variables: map[string]string{
					"primary-color": "#000",
				},
				Transitions: TransitionConfig{
					Type:     "fade",
					Duration: -100,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFontConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  FontConfig
		wantErr bool
	}{
		{
			name: "valid font with files",
			config: FontConfig{
				Name: "Roboto",
				Files: map[string]string{
					"regular": "fonts/roboto-regular.woff2",
					"bold":    "fonts/roboto-bold.woff2",
				},
				Fallback: "sans-serif",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: FontConfig{
				Files: map[string]string{
					"regular": "fonts/font.woff2",
				},
				Fallback: "sans-serif",
			},
			wantErr: true,
		},
		{
			name: "missing files",
			config: FontConfig{
				Name:     "Arial",
				Fallback: "sans-serif",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransitionConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  TransitionConfig
		wantErr bool
	}{
		{
			name: "valid fade transition",
			config: TransitionConfig{
				Type:     "fade",
				Duration: 300,
				Easing:   "ease-in-out",
			},
			wantErr: false,
		},
		{
			name: "valid slide transition",
			config: TransitionConfig{
				Type:     "slide",
				Duration: 400,
				Easing:   "ease-out",
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			config: TransitionConfig{
				Type:     "invalid",
				Duration: 300,
			},
			wantErr: true,
		},
		{
			name: "negative duration",
			config: TransitionConfig{
				Type:     "fade",
				Duration: -100,
			},
			wantErr: true,
		},
		{
			name: "invalid easing",
			config: TransitionConfig{
				Type:     "fade",
				Duration: 300,
				Easing:   "invalid-easing",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestThemeEngine_GetAsset(t *testing.T) {
	theme := &ThemeEngine{
		Assets: map[string]*ThemeAsset{
			"css/main.css": {
				Path:        "/themes/test/assets/css/main.css",
				Content:     []byte("body { color: red; }"),
				ContentType: "text/css",
			},
			"js/app.js": {
				Path:        "/themes/test/assets/js/app.js",
				Content:     []byte("console.log('hello');"),
				ContentType: "application/javascript",
			},
		},
	}

	// Test getting existing asset
	asset := theme.GetAsset("css/main.css")
	require.NotNil(t, asset)
	assert.Equal(t, "text/css", asset.ContentType)
	assert.Equal(t, "body { color: red; }", string(asset.Content))

	// Test getting non-existing asset
	asset = theme.GetAsset("css/nonexistent.css")
	assert.Nil(t, asset)
}

func TestThemeEngine_GetTemplate(t *testing.T) {
	theme := &ThemeEngine{
		Templates: map[string]*template.Template{
			"presentation": template.New("presentation"),
			"slide":        template.New("slide"),
		},
	}

	// Test getting existing template
	tmpl := theme.GetTemplate("presentation")
	require.NotNil(t, tmpl)
	assert.Equal(t, "presentation", tmpl.Name())

	// Test getting non-existing template
	tmpl = theme.GetTemplate("nonexistent")
	assert.Nil(t, tmpl)
}
