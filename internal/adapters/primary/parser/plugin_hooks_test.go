package parser

import (
	"bytes"
	"context"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Mock plugin service
type MockPluginService struct {
	mock.Mock
}

func (m *MockPluginService) LoadPlugin(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockPluginService) UnloadPlugin(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockPluginService) ExecutePlugin(ctx context.Context, name string, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	args := m.Called(ctx, name, input)
	return args.Get(0).(pluginapi.PluginOutput), args.Error(1)
}

func (m *MockPluginService) ListPlugins() []entities.LoadedPlugin {
	args := m.Called()
	if result := args.Get(0); result != nil {
		return result.([]entities.LoadedPlugin)
	}
	return nil
}

func (m *MockPluginService) GetPlugin(name string) (pluginapi.Plugin, error) {
	args := m.Called(name)
	if result := args.Get(0); result != nil {
		return result.(pluginapi.Plugin), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginService) GetPluginInfo(name string) (*entities.LoadedPlugin, error) {
	args := m.Called(name)
	if result := args.Get(0); result != nil {
		return result.(*entities.LoadedPlugin), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginService) DiscoverPlugins(ctx context.Context) ([]pluginapi.PluginInfo, error) {
	args := m.Called(ctx)
	if result := args.Get(0); result != nil {
		return result.([]pluginapi.PluginInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginService) ProcessContent(ctx context.Context, content string, language string) ([]pluginapi.PluginOutput, error) {
	args := m.Called(ctx, content, language)
	if result := args.Get(0); result != nil {
		return result.([]pluginapi.PluginOutput), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPluginService) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestPluginRenderer_DeterminePlugin(t *testing.T) {
	mockService := new(MockPluginService)
	renderer := NewPluginRenderer(mockService)

	tests := []struct {
		name     string
		language string
		content  string
		expected string
	}{
		{
			name:     "Mermaid diagram",
			language: "mermaid",
			content:  "graph TD\nA-->B",
			expected: "mermaid",
		},
		{
			name:     "Code execution",
			language: "exec",
			content:  "print('hello')",
			expected: "code-exec",
		},
		{
			name:     "Go code",
			language: "go",
			content:  "package main",
			expected: "syntax-highlight",
		},
		{
			name:     "Python code",
			language: "python",
			content:  "def hello():",
			expected: "syntax-highlight",
		},
		{
			name:     "JavaScript alias",
			language: "js",
			content:  "console.log('test')",
			expected: "syntax-highlight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.determinePlugin(tt.language, tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPluginRenderer_IsProgrammingLanguage(t *testing.T) {
	renderer := NewPluginRenderer(nil)

	tests := []struct {
		language string
		expected bool
	}{
		{"go", true},
		{"python", true},
		{"js", true},
		{"javascript", true},
		{"rust", true},
		{"mermaid", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			result := renderer.isProgrammingLanguage(tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPluginRenderer_RenderFencedCodeBlock(t *testing.T) {
	mockService := new(MockPluginService)

	// Test with Mermaid plugin
	t.Run("Mermaid rendering", func(t *testing.T) {
		mermaidOutput := pluginapi.PluginOutput{
			HTML: `<div class="mermaid">graph TD; A-->B;</div>`,
			Assets: []pluginapi.Asset{
				{Name: "mermaid.js", Content: []byte("//js"), ContentType: "application/javascript"},
			},
		}

		mockService.On("ExecutePlugin", mock.Anything, "mermaid", mock.Anything).Return(mermaidOutput, nil).Once()

		md := goldmark.New(
			goldmark.WithRendererOptions(
				renderer.WithNodeRenderers(
					util.Prioritized(NewPluginRenderer(mockService), 100),
				),
			),
		)

		source := "```mermaid\ngraph TD\nA-->B\n```"
		var buf bytes.Buffer
		err := md.Convert([]byte(source), &buf)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), `<div class="mermaid">`)
		mockService.AssertExpectations(t)
	})

	// Test with syntax highlighting
	t.Run("Syntax highlighting", func(t *testing.T) {
		syntaxOutput := pluginapi.PluginOutput{
			HTML: `<div class="code-block">highlighted code</div>`,
		}

		mockService.On("ExecutePlugin", mock.Anything, "syntax-highlight", mock.Anything).Return(syntaxOutput, nil).Once()

		md := goldmark.New(
			goldmark.WithRendererOptions(
				renderer.WithNodeRenderers(
					util.Prioritized(NewPluginRenderer(mockService), 100),
				),
			),
		)

		source := "```go\npackage main\n```"
		var buf bytes.Buffer
		err := md.Convert([]byte(source), &buf)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), `<div class="code-block">`)
		mockService.AssertExpectations(t)
	})
}

func TestPluginRenderer_DefaultRendering(t *testing.T) {
	// Test without plugin service
	md := goldmark.New(
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewPluginRenderer(nil), 100),
			),
		),
	)

	source := "```unknown\nsome code\n```"
	var buf bytes.Buffer
	err := md.Convert([]byte(source), &buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `<pre><code class="language-unknown">`)
	assert.Contains(t, output, "some code")
}
