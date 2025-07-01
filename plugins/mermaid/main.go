package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"

	"github.com/fredcamaral/slicli/pkg/plugin"
)

type MermaidPlugin struct {
	config map[string]interface{}
}

func (p *MermaidPlugin) Name() string        { return "mermaid" }
func (p *MermaidPlugin) Version() string     { return "1.0.0" }
func (p *MermaidPlugin) Description() string { return "Render Mermaid diagrams" }

func (p *MermaidPlugin) Init(config map[string]interface{}) error {
	p.config = config
	return nil
}

func (p *MermaidPlugin) Execute(ctx context.Context, input plugin.PluginInput) (plugin.PluginOutput, error) {
	// Extract options
	theme := "default"
	if t, ok := input.Options["theme"].(string); ok {
		theme = t
	}

	// Generate unique ID for diagram
	diagramID := p.generateID(input.Content)

	// Create HTML wrapper
	htmlOutput := fmt.Sprintf(`
		<div class="mermaid-diagram" id="%s">
			<pre class="mermaid" data-theme="%s">%s</pre>
		</div>
		<script>
			if (typeof mermaid !== 'undefined') {
				mermaid.init(undefined, document.querySelector('#%s .mermaid'));
			}
		</script>
	`, diagramID, theme, html.EscapeString(input.Content), diagramID)

	// Include Mermaid library and styles
	assets := []plugin.Asset{
		{
			Name:        "mermaid-init.js",
			Content:     []byte(mermaidInitScript),
			ContentType: "application/javascript",
		},
		{
			Name:        "mermaid.css",
			Content:     []byte(mermaidStyles),
			ContentType: "text/css",
		},
	}

	return plugin.PluginOutput{
		HTML:   htmlOutput,
		Assets: assets,
		Metadata: map[string]interface{}{
			"type":   "diagram",
			"engine": "mermaid",
			"theme":  theme,
		},
	}, nil
}

func (p *MermaidPlugin) Cleanup() error {
	// Clear configuration to free memory
	p.config = make(map[string]interface{})
	
	return nil
}

func (p *MermaidPlugin) generateID(content string) string {
	hash := sha256.Sum256([]byte(content))
	return "mermaid-" + base64.URLEncoding.EncodeToString(hash[:8])
}

var mermaidInitScript = `
// Lazy load Mermaid library
(function() {
	if (typeof mermaid === 'undefined') {
		var script = document.createElement('script');
		script.src = 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js';
		script.onload = function() {
			mermaid.initialize({ 
				startOnLoad: true,
				theme: document.documentElement.dataset.mermaidTheme || 'default',
				securityLevel: 'loose',
				fontFamily: 'monospace'
			});
			mermaid.init();
		};
		document.head.appendChild(script);
	}
})();
`

var mermaidStyles = `
.mermaid-diagram {
	margin: 1rem 0;
	padding: 1rem;
	background-color: #f8f9fa;
	border-radius: 0.5rem;
	overflow-x: auto;
}

.mermaid-diagram pre {
	margin: 0;
	background-color: transparent;
}

.mermaid-diagram .error {
	color: #d73a49;
	background-color: #ffeef0;
	padding: 0.5rem 1rem;
	border-radius: 0.25rem;
	font-family: monospace;
}

/* Dark theme adjustments */
.theme-dark .mermaid-diagram {
	background-color: #1e1e1e;
}

/* Print styles */
@media print {
	.mermaid-diagram {
		break-inside: avoid;
		page-break-inside: avoid;
	}
}
`

// Export plugin
var Plugin plugin.Plugin = &MermaidPlugin{}
