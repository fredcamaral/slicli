package main

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntaxHighlightPlugin_Basic(t *testing.T) {
	p := &SyntaxHighlightPlugin{}

	assert.Equal(t, "syntax-highlight", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
	assert.Equal(t, "Syntax highlighting for code blocks", p.Description())
}

func TestSyntaxHighlightPlugin_Init(t *testing.T) {
	p := &SyntaxHighlightPlugin{}
	config := map[string]interface{}{
		"defaultTheme": "monokai",
	}

	err := p.Init(config)
	require.NoError(t, err)
	assert.NotNil(t, p.formatter)
}

func TestSyntaxHighlightPlugin_Execute(t *testing.T) {
	tests := []struct {
		name     string
		input    plugin.PluginInput
		wantErr  bool
		validate func(t *testing.T, output plugin.PluginOutput)
	}{
		{
			name: "Go code",
			input: plugin.PluginInput{
				Content: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
				Language: "go",
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "code-block")
				assert.Contains(t, output.HTML, "data-language=\"go\"")
				// Check for either plain or HTML-encoded content
				assert.True(t, strings.Contains(output.HTML, "package main") ||
					strings.Contains(output.HTML, "package</span> <span") ||
					strings.Contains(output.HTML, "package") && strings.Contains(output.HTML, "main"))
				assert.Len(t, output.Assets, 2)
				assert.Equal(t, "go", output.Metadata["language"])
				assert.Equal(t, 7, output.Metadata["lines"])
			},
		},
		{
			name: "Python with custom theme",
			input: plugin.PluginInput{
				Content: `def hello(name):
    print(f"Hello, {name}!")

if __name__ == "__main__":
    hello("World")`,
				Language: "python",
				Options: map[string]interface{}{
					"theme": "monokai",
				},
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "data-language=\"python\"")
				assert.Equal(t, "python", output.Metadata["language"])
				assert.Equal(t, "monokai", output.Metadata["style"])
				// Check for monokai CSS
				hasMonokaiCSS := false
				for _, asset := range output.Assets {
					if asset.Name == "highlight-monokai.css" {
						hasMonokaiCSS = true
						break
					}
				}
				assert.True(t, hasMonokaiCSS)
			},
		},
		{
			name: "Auto-detect language",
			input: plugin.PluginInput{
				Content: `function greet(name) {
	console.log("Hello, " + name + "!");
}

greet("World");`,
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				lang, ok := output.Metadata["language"].(string)
				assert.True(t, ok)
				// Language detection might not be perfect, just ensure we have a language
				assert.NotEmpty(t, lang)
			},
		},
		{
			name: "Without line numbers",
			input: plugin.PluginInput{
				Content:  `SELECT * FROM users WHERE active = true;`,
				Language: "sql",
				Options: map[string]interface{}{
					"lineNumbers": false,
				},
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "data-language=\"sql\"")
				assert.Equal(t, "sql", output.Metadata["language"])
			},
		},
		{
			name: "Language alias",
			input: plugin.PluginInput{
				Content:  `console.log("test");`,
				Language: "js", // Should resolve to javascript
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Equal(t, "javascript", output.Metadata["language"])
			},
		},
		{
			name: "HTML with special characters",
			input: plugin.PluginInput{
				Content: `<script>
	alert("XSS attempt");
</script>`,
				Language: "html",
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				// The HTML output itself should be properly formatted,
				// not the content which will be handled by Chroma
				assert.Contains(t, output.HTML, "code-block")
				assert.Contains(t, output.HTML, "data-language=\"html\"")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &SyntaxHighlightPlugin{}
			err := p.Init(nil)
			require.NoError(t, err)

			output, err := p.Execute(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.validate(t, output)
			}
		})
	}
}

func TestSyntaxHighlightPlugin_DetectLanguage(t *testing.T) {
	p := &SyntaxHighlightPlugin{}

	tests := []struct {
		content  string
		expected string // We don't know exact result, just that it shouldn't be empty
	}{
		{
			content: `package main
import "fmt"
func main() { fmt.Println("Go") }`,
		},
		{
			content: `def hello():
    print("Python")`,
		},
		{
			content: `function test() {
	console.log("JavaScript");
}`,
		},
	}

	for _, tt := range tests {
		lang := p.detectLanguage(tt.content)
		assert.NotEmpty(t, lang)
	}
}

func TestSyntaxHighlightPlugin_Assets(t *testing.T) {
	p := &SyntaxHighlightPlugin{}
	err := p.Init(nil)
	require.NoError(t, err)

	output, err := p.Execute(context.Background(), plugin.PluginInput{
		Content:  "print('hello')",
		Language: "python",
	})
	require.NoError(t, err)

	// Should have 2 assets: style CSS and code-block CSS
	assert.Len(t, output.Assets, 2)

	var hasStyleCSS, hasBlockCSS bool
	for _, asset := range output.Assets {
		if strings.HasPrefix(asset.Name, "highlight-") {
			hasStyleCSS = true
			assert.Equal(t, "text/css", asset.ContentType)
			assert.NotEmpty(t, asset.Content)
		}
		if asset.Name == "code-block.css" {
			hasBlockCSS = true
			assert.Equal(t, "text/css", asset.ContentType)
			assert.Contains(t, string(asset.Content), ".code-block")
		}
	}

	assert.True(t, hasStyleCSS, "Should have syntax style CSS")
	assert.True(t, hasBlockCSS, "Should have code block CSS")
}

func TestSyntaxHighlightPlugin_Cleanup(t *testing.T) {
	p := &SyntaxHighlightPlugin{}
	
	// Initialize with some config
	config := map[string]interface{}{
		"style": "github",
		"line_numbers": true,
	}
	err := p.Init(config)
	assert.NoError(t, err)
	
	// Verify initialization worked
	assert.NotNil(t, p.config)
	assert.NotNil(t, p.formatter)
	assert.Len(t, p.config, 2)
	
	// Populate lexer cache by getting a lexer
	_ = getLexer("go")
	assert.Greater(t, len(lexerCache), 0, "Lexer cache should have entries")
	
	// Cleanup
	err = p.Cleanup()
	assert.NoError(t, err)
	
	// Verify cleanup
	assert.Empty(t, p.config, "Config should be cleared after cleanup")
	assert.Nil(t, p.formatter, "Formatter should be nil after cleanup")
	assert.Empty(t, lexerCache, "Lexer cache should be cleared after cleanup")
}

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"js", "javascript"},
		{"ts", "typescript"},
		{"py", "python"},
		{"yml", "yaml"},
		{"c++", "cpp"},
		{"go", "go"},           // No alias, should return as-is
		{"unknown", "unknown"}, // Unknown, should return as-is
	}

	for _, tt := range tests {
		result := resolveLanguage(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestGetLexer(t *testing.T) {
	// Test lexer caching
	lexer1 := getLexer("go")
	lexer2 := getLexer("go")

	assert.NotNil(t, lexer1)
	assert.NotNil(t, lexer2)
	// Should be the same instance due to caching
	assert.Equal(t, lexer1, lexer2)

	// Test fallback for unknown language
	lexer3 := getLexer("unknown-language-xyz")
	assert.NotNil(t, lexer3)
}
