// Package main implements an example slicli plugin.
// This serves as a template for creating new plugins.
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/fredcamaral/slicli/pkg/plugin"
)

// ExamplePlugin demonstrates how to implement a slicli plugin.
type ExamplePlugin struct {
	config map[string]interface{}
}

// Name returns the unique name of the plugin.
func (p *ExamplePlugin) Name() string {
	return "example"
}

// Version returns the semantic version of the plugin.
func (p *ExamplePlugin) Version() string {
	return "1.0.0"
}

// Description returns a human-readable description of the plugin.
func (p *ExamplePlugin) Description() string {
	return "Example plugin that demonstrates the plugin API"
}

// Init initializes the plugin with the provided configuration.
func (p *ExamplePlugin) Init(config map[string]interface{}) error {
	p.config = config

	// Validate configuration
	if p.config == nil {
		p.config = make(map[string]interface{})
	}

	// Set defaults
	if _, ok := p.config["style"].(string); !ok {
		p.config["style"] = "default"
	}

	return nil
}

// Execute processes the input and returns the output.
func (p *ExamplePlugin) Execute(ctx context.Context, input plugin.PluginInput) (plugin.PluginOutput, error) {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return plugin.PluginOutput{}, ctx.Err()
	default:
	}

	// Process based on language hint
	var html string
	switch input.Language {
	case "example-box":
		html = p.renderBox(input.Content)
	case "example-highlight":
		html = p.renderHighlight(input.Content)
	default:
		// Default processing
		html = p.renderDefault(input.Content)
	}

	// Generate CSS based on style
	style := p.config["style"].(string)
	css := p.generateCSS(style)

	return plugin.PluginOutput{
		HTML: html,
		Assets: []plugin.Asset{
			{
				Name:        "example.css",
				Content:     []byte(css),
				ContentType: "text/css",
			},
		},
		Metadata: map[string]interface{}{
			"processed_by": p.Name(),
			"style":        style,
		},
	}, nil
}

// Cleanup releases any resources held by the plugin.
func (p *ExamplePlugin) Cleanup() error {
	// Clean up any resources
	// This is called when the plugin is unloaded
	return nil
}

// renderBox renders content in a styled box.
func (p *ExamplePlugin) renderBox(content string) string {
	lines := strings.Split(content, "\n")
	title := "Example"
	if len(lines) > 0 && strings.HasPrefix(lines[0], "title:") {
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "title:"))
		lines = lines[1:]
	}

	contentHTML := strings.Join(lines, "<br>")
	return fmt.Sprintf(`<div class="example-box">
		<div class="example-box-title">%s</div>
		<div class="example-box-content">%s</div>
	</div>`, title, contentHTML)
}

// renderHighlight renders content with syntax highlighting.
func (p *ExamplePlugin) renderHighlight(content string) string {
	// Simple line numbering
	lines := strings.Split(content, "\n")
	var htmlLines []string
	for i, line := range lines {
		htmlLines = append(htmlLines, fmt.Sprintf(
			`<span class="line-number">%d</span><span class="line-content">%s</span>`,
			i+1, strings.ReplaceAll(line, " ", "&nbsp;"),
		))
	}

	return fmt.Sprintf(`<div class="example-highlight">
		<pre><code>%s</code></pre>
	</div>`, strings.Join(htmlLines, "\n"))
}

// renderDefault renders content with default styling.
func (p *ExamplePlugin) renderDefault(content string) string {
	return fmt.Sprintf(`<div class="example-default">%s</div>`, content)
}

// generateCSS generates CSS based on the style configuration.
func (p *ExamplePlugin) generateCSS(style string) string {
	switch style {
	case "dark":
		return `
.example-box {
	background: #1e1e1e;
	border: 1px solid #444;
	border-radius: 8px;
	margin: 1rem 0;
	overflow: hidden;
}

.example-box-title {
	background: #2d2d2d;
	color: #fff;
	padding: 0.5rem 1rem;
	font-weight: bold;
	border-bottom: 1px solid #444;
}

.example-box-content {
	padding: 1rem;
	color: #ddd;
}

.example-highlight {
	background: #1e1e1e;
	border-radius: 8px;
	overflow: auto;
	margin: 1rem 0;
}

.example-highlight pre {
	margin: 0;
	padding: 1rem;
	color: #ddd;
}

.line-number {
	color: #666;
	margin-right: 1rem;
	user-select: none;
}

.line-content {
	color: #ddd;
}

.example-default {
	padding: 1rem;
	background: #2d2d2d;
	color: #ddd;
	border-radius: 4px;
	margin: 1rem 0;
}`

	default: // "default" style
		return `
.example-box {
	background: #f5f5f5;
	border: 1px solid #ddd;
	border-radius: 8px;
	margin: 1rem 0;
	overflow: hidden;
}

.example-box-title {
	background: #e0e0e0;
	color: #333;
	padding: 0.5rem 1rem;
	font-weight: bold;
	border-bottom: 1px solid #ddd;
}

.example-box-content {
	padding: 1rem;
	color: #333;
}

.example-highlight {
	background: #f5f5f5;
	border-radius: 8px;
	overflow: auto;
	margin: 1rem 0;
}

.example-highlight pre {
	margin: 0;
	padding: 1rem;
	color: #333;
}

.line-number {
	color: #999;
	margin-right: 1rem;
	user-select: none;
}

.line-content {
	color: #333;
}

.example-default {
	padding: 1rem;
	background: #f0f0f0;
	color: #333;
	border-radius: 4px;
	margin: 1rem 0;
}`
	}
}

// Plugin is the exported plugin instance.
// This MUST be named "Plugin" for the loader to find it.
var Plugin plugin.Plugin = &ExamplePlugin{}
