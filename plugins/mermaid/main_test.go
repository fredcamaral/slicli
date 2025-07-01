package main

import (
	"context"
	"strings"
	"testing"

	"github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMermaidPlugin_Basic(t *testing.T) {
	p := &MermaidPlugin{}

	assert.Equal(t, "mermaid", p.Name())
	assert.Equal(t, "1.0.0", p.Version())
	assert.Equal(t, "Render Mermaid diagrams", p.Description())
}

func TestMermaidPlugin_Init(t *testing.T) {
	p := &MermaidPlugin{}
	config := map[string]interface{}{
		"defaultTheme": "dark",
	}

	err := p.Init(config)
	require.NoError(t, err)
	assert.Equal(t, config, p.config)
}

func TestMermaidPlugin_Execute(t *testing.T) {
	tests := []struct {
		name     string
		input    plugin.PluginInput
		wantErr  bool
		validate func(t *testing.T, output plugin.PluginOutput)
	}{
		{
			name: "simple flowchart",
			input: plugin.PluginInput{
				Content: `graph TD
					A[Start] --> B{Is it?}
					B -->|Yes| C[OK]
					B -->|No| D[End]`,
				Language: "mermaid",
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "class=\"mermaid\"")
				assert.Contains(t, output.HTML, "graph TD")
				assert.Contains(t, output.HTML, "data-theme=\"default\"")
				assert.Len(t, output.Assets, 2)
				assert.Equal(t, "diagram", output.Metadata["type"])
				assert.Equal(t, "mermaid", output.Metadata["engine"])
			},
		},
		{
			name: "with custom theme",
			input: plugin.PluginInput{
				Content: `sequenceDiagram
					Alice->>Bob: Hello Bob!
					Bob->>Alice: Hi Alice!`,
				Language: "mermaid",
				Options: map[string]interface{}{
					"theme": "dark",
				},
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "data-theme=\"dark\"")
				assert.Equal(t, "dark", output.Metadata["theme"])
			},
		},
		{
			name: "gantt chart",
			input: plugin.PluginInput{
				Content: `gantt
					title A Gantt Diagram
					dateFormat  YYYY-MM-DD
					section Section
					A task           :a1, 2024-01-01, 30d
					Another task     :after a1  , 20d`,
				Language: "mermaid",
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				assert.Contains(t, output.HTML, "gantt")
				assert.Contains(t, output.HTML, "title A Gantt Diagram")
			},
		},
		{
			name: "with special characters",
			input: plugin.PluginInput{
				Content: `graph LR
					A["Node with <script>alert('xss')</script>"]
					B["Node with & and < and >"]
					A --> B`,
				Language: "mermaid",
			},
			validate: func(t *testing.T, output plugin.PluginOutput) {
				// Check that HTML is properly escaped
				assert.NotContains(t, output.HTML, "<script>alert")
				assert.Contains(t, output.HTML, "&lt;script&gt;")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &MermaidPlugin{}
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

func TestMermaidPlugin_GenerateID(t *testing.T) {
	p := &MermaidPlugin{}

	// Same content should generate same ID
	content := "graph TD\nA-->B"
	id1 := p.generateID(content)
	id2 := p.generateID(content)
	assert.Equal(t, id1, id2)
	assert.True(t, strings.HasPrefix(id1, "mermaid-"))

	// Different content should generate different ID
	id3 := p.generateID("different content")
	assert.NotEqual(t, id1, id3)
}

func TestMermaidPlugin_Assets(t *testing.T) {
	p := &MermaidPlugin{}
	err := p.Init(nil)
	require.NoError(t, err)

	output, err := p.Execute(context.Background(), plugin.PluginInput{
		Content: "graph TD\nA-->B",
	})
	require.NoError(t, err)

	// Check assets
	assert.Len(t, output.Assets, 2)

	var hasJS, hasCSS bool
	for _, asset := range output.Assets {
		switch asset.Name {
		case "mermaid-init.js":
			hasJS = true
			assert.Equal(t, "application/javascript", asset.ContentType)
			assert.Contains(t, string(asset.Content), "mermaid.initialize")
		case "mermaid.css":
			hasCSS = true
			assert.Equal(t, "text/css", asset.ContentType)
			assert.Contains(t, string(asset.Content), ".mermaid-diagram")
		}
	}

	assert.True(t, hasJS, "Should have JavaScript asset")
	assert.True(t, hasCSS, "Should have CSS asset")
}

func TestMermaidPlugin_Cleanup(t *testing.T) {
	p := &MermaidPlugin{}
	
	// Initialize with some config
	config := map[string]interface{}{
		"theme": "dark",
		"scale": 1.5,
		"background": "white",
	}
	err := p.Init(config)
	assert.NoError(t, err)
	
	// Verify initialization worked
	assert.NotNil(t, p.config)
	assert.Len(t, p.config, 3)
	
	// Cleanup
	err = p.Cleanup()
	assert.NoError(t, err)
	
	// Verify cleanup
	assert.Empty(t, p.config, "Config should be cleared after cleanup")
}
