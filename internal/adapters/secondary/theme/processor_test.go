package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetProcessor_ProcessCSS(t *testing.T) {
	processor := NewAssetProcessor(false)

	tests := []struct {
		name      string
		css       string
		variables map[string]string
		want      string
	}{
		{
			name: "simple variable replacement",
			css: `body {
				color: var(--primary-color);
				background: var(--bg-color);
			}`,
			variables: map[string]string{
				"primary-color": "#000",
				"bg-color":      "#fff",
			},
			want: `body {
				color: #000;
				background: #fff;
			}`,
		},
		{
			name: "variable with fallback",
			css: `body {
				color: var(--primary-color, #333);
				font-size: var(--font-size, 16px);
			}`,
			variables: map[string]string{
				"primary-color": "#000",
			},
			want: `body {
				color: #000;
				font-size: 16px;
			}`,
		},
		{
			name: "root variables extraction",
			css: `:root {
				--primary-color: #123;
				--secondary-color: #456;
			}
			body {
				color: var(--primary-color);
				background: var(--secondary-color);
			}`,
			variables: map[string]string{
				"primary-color": "#abc", // Override root value
			},
			want: `:root {
				--primary-color: #abc;
				--secondary-color: #456;
			}
			body {
				color: #abc;
				background: #456;
			}`,
		},
		{
			name: "nested var() calls",
			css: `body {
				color: var(--text-color, var(--fallback-color, #000));
			}`,
			variables: map[string]string{
				"fallback-color": "#333",
			},
			want: `body {
				color: #333;
			}`,
		},
		{
			name: "preserve non-variable content",
			css: `body {
				content: "var(--not-a-variable)";
				background: url('var(--also-not)');
				color: var(--primary-color);
			}`,
			variables: map[string]string{
				"primary-color": "#000",
			},
			want: `body {
				content: "var(--not-a-variable)";
				background: url('var(--also-not)');
				color: #000;
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessCSS([]byte(tt.css), tt.variables)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(result))
		})
	}
}

func TestAssetProcessor_ProcessJS(t *testing.T) {
	processor := NewAssetProcessor(false)

	tests := []struct {
		name      string
		js        string
		variables map[string]string
		want      string
	}{
		{
			name: "simple variable replacement",
			js: `const theme = {
				primaryColor: '{{primary-color}}',
				fontSize: '{{font-size}}'
			};`,
			variables: map[string]string{
				"primary-color": "#000",
				"font-size":     "16px",
			},
			want: `const theme = {
				primaryColor: '#000',
				fontSize: '16px'
			};`,
		},
		{
			name: "preserve non-template content",
			js: `console.log('{{}}'); // Not a variable
			const color = '{{primary-color}}';`,
			variables: map[string]string{
				"primary-color": "#000",
			},
			want: `console.log('{{}}'); // Not a variable
			const color = '#000';`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessJS([]byte(tt.js), tt.variables)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(result))
		})
	}
}

func TestAssetProcessor_MinifyCSS(t *testing.T) {
	processor := NewAssetProcessor(true)

	css := `
		/* Comment */
		body {
			color: #000;
			background-color: #fff;
		}
		
		.class {
			margin: 0;
			padding: 0;
		}
	`

	result, err := processor.MinifyCSS([]byte(css))
	require.NoError(t, err)

	// Should remove comments and extra whitespace
	assert.NotContains(t, string(result), "/* Comment */")
	assert.NotContains(t, string(result), "\n\n")
	assert.Contains(t, string(result), "body{")
	assert.Contains(t, string(result), "color:#000")
}

func TestAssetProcessor_MinifyJS(t *testing.T) {
	processor := NewAssetProcessor(true)

	js := `
		// Single line comment
		function test() {
			const x = 1;
			/* Multi line
			   comment */
			return x + 2;
		}
	`

	result, err := processor.MinifyJS([]byte(js))
	require.NoError(t, err)

	// Should remove comments
	assert.NotContains(t, string(result), "// Single line comment")
	assert.NotContains(t, string(result), "/* Multi line")
	assert.Contains(t, string(result), "function test()")
}

// func TestAssetProcessor_extractRootVariables(t *testing.T) {
// 	processor := NewAssetProcessor(false)

// 	css := `:root {
// 		--primary-color: #123;
// 		--secondary-color: #456;
// 		--font-size: 16px;
// 	}

// 	:root {
// 		--additional-var: #789;
// 	}

// 	body {
// 		color: var(--primary-color);
// 	}`

// 	vars := processor.extractRootVariables(css)

// 	expected := map[string]string{
// 		"primary-color":   "#123",
// 		"secondary-color": "#456",
// 		"font-size":       "16px",
// 		"additional-var":  "#789",
// 	}

// 	assert.Equal(t, expected, vars)
// }

func TestAssetProcessor_CSSVariableEdgeCases(t *testing.T) {
	processor := NewAssetProcessor(false)

	tests := []struct {
		name      string
		css       string
		variables map[string]string
		want      string
	}{
		{
			name: "calc with variables",
			css:  `width: calc(100% - var(--spacing));`,
			variables: map[string]string{
				"spacing": "20px",
			},
			want: `width: calc(100% - 20px);`,
		},
		{
			name: "multiple variables in one line",
			css:  `margin: var(--top) var(--right) var(--bottom) var(--left);`,
			variables: map[string]string{
				"top":    "10px",
				"right":  "20px",
				"bottom": "30px",
				"left":   "40px",
			},
			want: `margin: 10px 20px 30px 40px;`,
		},
		{
			name: "rgb with variables",
			css:  `color: rgb(var(--r), var(--g), var(--b));`,
			variables: map[string]string{
				"r": "255",
				"g": "128",
				"b": "0",
			},
			want: `color: rgb(255, 128, 0);`,
		},
		{
			name: "complex fallback chain",
			css:  `color: var(--primary, var(--secondary, var(--tertiary, black)));`,
			variables: map[string]string{
				"tertiary": "blue",
			},
			want: `color: blue;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessCSS([]byte(tt.css), tt.variables)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(result))
		})
	}
}

func TestAssetProcessor_Process(t *testing.T) {
	processor := NewAssetProcessor(false)

	tests := []struct {
		name        string
		content     []byte
		contentType string
		variables   map[string]string
		wantErr     bool
	}{
		{
			name:        "process CSS",
			content:     []byte(`body { color: var(--color); }`),
			contentType: "text/css",
			variables:   map[string]string{"color": "#000"},
			wantErr:     false,
		},
		{
			name:        "process JavaScript",
			content:     []byte(`const color = '{{color}}';`),
			contentType: "application/javascript",
			variables:   map[string]string{"color": "#000"},
			wantErr:     false,
		},
		{
			name:        "process JS (text/javascript)",
			content:     []byte(`const color = '{{color}}';`),
			contentType: "text/javascript",
			variables:   map[string]string{"color": "#000"},
			wantErr:     false,
		},
		{
			name:        "skip processing for images",
			content:     []byte{0xFF, 0xD8, 0xFF}, // JPEG header
			contentType: "image/jpeg",
			variables:   map[string]string{},
			wantErr:     false,
		},
		{
			name:        "skip processing for unknown types",
			content:     []byte(`some content`),
			contentType: "application/octet-stream",
			variables:   map[string]string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Process(tt.content, tt.contentType, tt.variables)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
