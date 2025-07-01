package main

import (
	"context"
	"fmt"
	stdhtml "html"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/fredcamaral/slicli/pkg/plugin"
)

type SyntaxHighlightPlugin struct {
	config    map[string]interface{}
	formatter *html.Formatter
	mu        sync.RWMutex
}

func (p *SyntaxHighlightPlugin) Name() string        { return "syntax-highlight" }
func (p *SyntaxHighlightPlugin) Version() string     { return "1.0.0" }
func (p *SyntaxHighlightPlugin) Description() string { return "Syntax highlighting for code blocks" }

func (p *SyntaxHighlightPlugin) Init(config map[string]interface{}) error {
	p.config = config

	// Configure formatter
	p.formatter = html.New(
		html.WithLineNumbers(true),
		html.WithLinkableLineNumbers(true, "L"),
		html.TabWidth(4),
	)

	return nil
}

func (p *SyntaxHighlightPlugin) Execute(ctx context.Context, input plugin.PluginInput) (plugin.PluginOutput, error) {
	// Get language
	language := input.Language
	if language == "" {
		language = p.detectLanguage(input.Content)
	}

	// Resolve any aliases
	language = resolveLanguage(language)

	// Get lexer
	lexer := getLexer(language)

	// Get style
	styleName := "github"
	if s, ok := input.Options["theme"].(string); ok {
		styleName = s
	}

	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}

	// Configure formatter options
	options := []html.Option{
		html.WithLineNumbers(p.shouldShowLineNumbers(input.Options)),
		html.WithClasses(true), // Use CSS classes instead of inline styles
		html.PreventSurroundingPre(false),
	}

	formatter := html.New(options...)

	// Tokenize and format
	var output strings.Builder
	iterator, err := lexer.Tokenise(nil, input.Content)
	if err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("tokenizing code: %w", err)
	}

	err = formatter.Format(&output, style, iterator)
	if err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("formatting code: %w", err)
	}

	// Wrap in container
	htmlOutput := fmt.Sprintf(`
		<div class="code-block" data-language="%s">
			<div class="code-header">
				<span class="code-language">%s</span>
			</div>
			%s
		</div>
	`, stdhtml.EscapeString(language), stdhtml.EscapeString(language), output.String())

	// Generate CSS for the style
	var cssBuilder strings.Builder
	if err := formatter.WriteCSS(&cssBuilder, style); err != nil {
		return plugin.PluginOutput{}, fmt.Errorf("failed to generate CSS: %w", err)
	}

	return plugin.PluginOutput{
		HTML: htmlOutput,
		Assets: []plugin.Asset{
			{
				Name:        fmt.Sprintf("highlight-%s.css", styleName),
				Content:     []byte(cssBuilder.String()),
				ContentType: "text/css",
			},
			{
				Name:        "code-block.css",
				Content:     []byte(codeBlockStyles),
				ContentType: "text/css",
			},
		},
		Metadata: map[string]interface{}{
			"language": language,
			"lines":    strings.Count(input.Content, "\n") + 1,
			"style":    styleName,
		},
	}, nil
}

func (p *SyntaxHighlightPlugin) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Clear configuration
	p.config = make(map[string]interface{})
	
	// Clear formatter reference 
	p.formatter = nil
	
	// Clear global lexer cache
	lexerMu.Lock()
	lexerCache = make(map[string]chroma.Lexer)
	lexerMu.Unlock()
	
	return nil
}

func (p *SyntaxHighlightPlugin) detectLanguage(content string) string {
	lexer := lexers.Analyse(content)
	if lexer != nil {
		return lexer.Config().Name
	}
	return "text"
}

func (p *SyntaxHighlightPlugin) shouldShowLineNumbers(options map[string]interface{}) bool {
	if ln, ok := options["lineNumbers"].(bool); ok {
		return ln
	}
	// Default to showing line numbers
	return true
}

// Lexer cache for performance
var (
	lexerCache = make(map[string]chroma.Lexer)
	lexerMu    sync.RWMutex
)

func getLexer(language string) chroma.Lexer {
	lexerMu.RLock()
	lexer, ok := lexerCache[language]
	lexerMu.RUnlock()

	if ok {
		return lexer
	}

	lexerMu.Lock()
	defer lexerMu.Unlock()

	// Double-check after acquiring write lock
	if lexer, ok = lexerCache[language]; ok {
		return lexer
	}

	lexer = lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	lexerCache[language] = lexer
	return lexer
}

var codeBlockStyles = `
.code-block {
	margin: 1rem 0;
	border-radius: 0.5rem;
	overflow: hidden;
	background-color: #f6f8fa;
}

.code-header {
	padding: 0.5rem 1rem;
	background-color: #e1e4e8;
	border-bottom: 1px solid #d1d5da;
	font-size: 0.875rem;
	color: #586069;
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
}

.code-language {
	font-weight: 500;
}

.code-block pre {
	margin: 0;
	padding: 1rem;
	overflow-x: auto;
	background-color: #f6f8fa;
}

.code-block .chroma {
	margin: 0;
	background-color: transparent;
}

/* Line numbers styling */
.code-block .line-numbers {
	user-select: none;
	color: #768390;
	padding-right: 1rem;
}

.code-block .line-numbers a {
	color: inherit;
	text-decoration: none;
}

/* Dark theme adjustments */
.theme-dark .code-block {
	background-color: #1e1e1e;
}

.theme-dark .code-header {
	background-color: #2d2d30;
	border-bottom-color: #3e3e42;
	color: #cccccc;
}

.theme-dark .code-block pre {
	background-color: #1e1e1e;
}

/* Mobile responsiveness */
@media (max-width: 768px) {
	.code-block {
		border-radius: 0;
		margin: 0.5rem -1rem;
	}
	
	.code-block pre {
		padding: 0.75rem;
		font-size: 0.875rem;
	}
}

/* Print styles */
@media print {
	.code-block {
		break-inside: avoid;
		page-break-inside: avoid;
	}
	
	.code-header {
		background-color: #f0f0f0;
		color: #000;
	}
}
`

// Export plugin
var Plugin plugin.Plugin = &SyntaxHighlightPlugin{}
