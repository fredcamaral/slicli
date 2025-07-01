package parser

import (
	"context"
	"strings"
	"sync"

	"github.com/fredcamaral/slicli/internal/domain/ports"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// PluginRenderer handles rendering of plugin blocks
type PluginRenderer struct {
	html.Config
	pluginService ports.PluginService
	assets        map[string][]pluginapi.Asset // Store assets for later inclusion
	mu            sync.Mutex                   // Protect assets map
}

// NewPluginRenderer creates a new plugin renderer
func NewPluginRenderer(pluginService ports.PluginService, opts ...html.Option) *PluginRenderer {
	r := &PluginRenderer{
		Config:        html.NewConfig(),
		pluginService: pluginService,
		assets:        make(map[string][]pluginapi.Asset),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers rendering functions for plugin blocks
func (r *PluginRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	// Register for fenced code blocks
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

// renderFencedCodeBlock renders a fenced code block, potentially using plugins
func (r *PluginRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*ast.FencedCodeBlock)
	language := string(n.Language(source))

	// Skip if no plugin service
	if r.pluginService == nil {
		return r.renderDefaultCodeBlock(w, source, n, language)
	}

	// Extract content
	var content strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		content.Write(line.Value(source))
	}

	// Determine if this should be handled by a plugin
	pluginName := r.determinePlugin(language, content.String())
	if pluginName == "" {
		return r.renderDefaultCodeBlock(w, source, n, language)
	}

	// Execute plugin
	ctx := context.Background()
	input := pluginapi.PluginInput{
		Content:  content.String(),
		Language: language,
		Options:  r.extractOptions(n),
	}

	output, err := r.pluginService.ExecutePlugin(ctx, pluginName, input)
	if err != nil {
		// Fallback to default rendering on error
		return r.renderDefaultCodeBlock(w, source, n, language)
	}

	// Write plugin output
	if _, err := w.WriteString(output.HTML); err != nil {
		return ast.WalkStop, err
	}

	// Store assets for later inclusion
	if len(output.Assets) > 0 {
		r.storeAssets(output.Assets)
	}

	return ast.WalkSkipChildren, nil
}

// renderDefaultCodeBlock renders a code block without plugin processing
func (r *PluginRenderer) renderDefaultCodeBlock(w util.BufWriter, source []byte, n *ast.FencedCodeBlock, language string) (ast.WalkStatus, error) {
	if _, err := w.WriteString(`<pre><code`); err != nil {
		return ast.WalkStop, err
	}
	if language != "" {
		if _, err := w.WriteString(` class="language-`); err != nil {
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(language); err != nil { // Language identifiers are safe
			return ast.WalkStop, err
		}
		if _, err := w.WriteString(`"`); err != nil {
			return ast.WalkStop, err
		}
	}
	if _, err := w.WriteString(`>`); err != nil {
		return ast.WalkStop, err
	}

	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		if _, err := w.Write(util.EscapeHTML(line.Value(source))); err != nil {
			return ast.WalkStop, err
		}
	}

	if _, err := w.WriteString(`</code></pre>`); err != nil {
		return ast.WalkStop, err
	}
	if err := w.WriteByte('\n'); err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkSkipChildren, nil
}

// determinePlugin determines which plugin should handle this block
func (r *PluginRenderer) determinePlugin(language string, content string) string {
	// Direct plugin mappings
	switch strings.ToLower(language) {
	case "mermaid":
		return "mermaid"
	case "exec", "execute", "run":
		return "code-exec"
	}

	// Check if it's a programming language that needs highlighting
	if r.isProgrammingLanguage(language) {
		return "syntax-highlight"
	}

	// No automatic content matching for now
	// This could be extended with a separate matcher service

	return ""
}

// isProgrammingLanguage checks if the language is a known programming language
func (r *PluginRenderer) isProgrammingLanguage(language string) bool {
	programmingLangs := map[string]bool{
		"go": true, "golang": true, "python": true, "py": true,
		"javascript": true, "js": true, "typescript": true, "ts": true,
		"java": true, "c": true, "cpp": true, "c++": true, "csharp": true, "c#": true,
		"rust": true, "ruby": true, "rb": true, "php": true, "swift": true,
		"kotlin": true, "scala": true, "r": true, "julia": true, "dart": true,
		"bash": true, "sh": true, "shell": true, "powershell": true,
		"sql": true, "html": true, "css": true, "scss": true, "sass": true,
		"json": true, "xml": true, "yaml": true, "yml": true, "toml": true,
		"dockerfile": true, "makefile": true, "cmake": true,
		"lua": true, "perl": true, "haskell": true, "clojure": true,
		"elixir": true, "erlang": true, "ocaml": true, "fsharp": true, "f#": true,
	}

	return programmingLangs[strings.ToLower(language)]
}

// optionRegex matches JSON-like options in code block info strings (kept for potential future use)
// Examples: ```go {lineNumbers: false, theme: "dark"}
// var optionRegex = regexp.MustCompile(`\{([^}]+)\}`)

// extractOptions extracts plugin options from node attributes
func (r *PluginRenderer) extractOptions(n *ast.FencedCodeBlock) map[string]interface{} {
	options := make(map[string]interface{})

	// For now, skip option extraction due to complexity with text segments
	// This can be enhanced later when needed
	return options
}

// storeAssets stores plugin assets for later inclusion
func (r *PluginRenderer) storeAssets(assets []pluginapi.Asset) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Group assets by type for efficient inclusion
	for _, asset := range assets {
		// Derive asset type from ContentType
		assetKey := "other"
		if strings.HasPrefix(asset.ContentType, "text/css") {
			assetKey = "css"
		} else if strings.HasPrefix(asset.ContentType, "application/javascript") || strings.HasPrefix(asset.ContentType, "text/javascript") {
			assetKey = "javascript"
		}
		r.assets[assetKey] = append(r.assets[assetKey], asset)
	}
}

// GetAssets returns all stored assets grouped by type
func (r *PluginRenderer) GetAssets() map[string][]pluginapi.Asset {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Return a copy to avoid concurrent modification
	assetsCopy := make(map[string][]pluginapi.Asset)
	for key, assets := range r.assets {
		assetsCopy[key] = make([]pluginapi.Asset, len(assets))
		copy(assetsCopy[key], assets)
	}
	return assetsCopy
}

// ClearAssets clears all stored assets
func (r *PluginRenderer) ClearAssets() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assets = make(map[string][]pluginapi.Asset)
}

// GenerateAssetHTML generates HTML for including assets in the page head
func (r *PluginRenderer) GenerateAssetHTML() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var html strings.Builder

	// Include CSS assets
	if cssAssets, exists := r.assets["css"]; exists {
		for _, asset := range cssAssets {
			if len(asset.Content) > 0 {
				html.WriteString(`<style>`)
				html.WriteString(string(asset.Content))
				html.WriteString(`</style>`)
				html.WriteString("\n")
			}
		}
	}

	// Include JavaScript assets
	if jsAssets, exists := r.assets["javascript"]; exists {
		for _, asset := range jsAssets {
			if len(asset.Content) > 0 {
				html.WriteString(`<script>`)
				html.WriteString(string(asset.Content))
				html.WriteString(`</script>`)
				html.WriteString("\n")
			}
		}
	}

	return html.String()
}

// parseInt parses a string as an integer (kept for potential future use)
// func parseInt(s string) (int, error) {
//	// Simple integer parsing without importing strconv to keep dependencies minimal
//	if s == "" {
//		return 0, errors.New("empty string")
//	}
//
//	result := 0
//	negative := false
//	start := 0
//
//	if s[0] == '-' {
//		negative = true
//		start = 1
//	} else if s[0] == '+' {
//		start = 1
//	}
//
//	for i := start; i < len(s); i++ {
//		c := s[i]
//		if c < '0' || c > '9' {
//			return 0, errors.New("invalid character")
//		}
//		result = result*10 + int(c-'0')
//	}
//
//	if negative {
//		result = -result
//	}
//
//	return result, nil
// }

// parseFloat parses a string as a float64 (kept for potential future use)
// func parseFloat(s string) (float64, error) {
//	// Simple float parsing for basic cases
//	if s == "" {
//		return 0, errors.New("empty string")
//	}
//
//	parts := strings.Split(s, ".")
//	if len(parts) != 2 {
//		return 0, errors.New("invalid float format")
//	}
//
//	// Parse integer part
//	intPart, err := parseInt(parts[0])
//	if err != nil {
//		return 0, err
//	}
//
//	// Parse decimal part
//	decStr := parts[1]
//	decPart := 0
//	for _, c := range decStr {
//		if c < '0' || c > '9' {
//			return 0, errors.New("invalid decimal")
//		}
//		decPart = decPart*10 + int(c-'0')
//	}
//
//	// Convert to float
//	result := float64(intPart)
//	if len(decStr) > 0 {
//		divisor := 1.0
//		for i := 0; i < len(decStr); i++ {
//			divisor *= 10
//		}
//		if intPart >= 0 {
//			result += float64(decPart) / divisor
//		} else {
//			result -= float64(decPart) / divisor
//		}
//	}
//
//	return result, nil
// }

// PluginExtension is a Goldmark extension for plugin support
type PluginExtension struct {
	pluginService ports.PluginService
}

// NewPluginExtension creates a new plugin extension
func NewPluginExtension(pluginService ports.PluginService) *PluginExtension {
	return &PluginExtension{
		pluginService: pluginService,
	}
}

// Extend extends the markdown parser with plugin support
func (e *PluginExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewPluginRenderer(e.pluginService), 100),
		),
	)
}
