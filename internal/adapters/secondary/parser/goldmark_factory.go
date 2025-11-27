package parser

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
)

// GoldmarkConfig defines configuration options for creating Goldmark instances
type GoldmarkConfig struct {
	// EnableGFM enables GitHub Flavored Markdown extensions
	EnableGFM bool
	// EnableTypographer enables smart punctuation
	EnableTypographer bool
	// EnableTables enables table support
	EnableTables bool
	// EnableStrikethrough enables ~~strikethrough~~ syntax
	EnableStrikethrough bool
	// EnableTaskList enables task list support (- [ ] items)
	EnableTaskList bool
	// EnableAutoHeadingID enables automatic heading IDs
	EnableAutoHeadingID bool
	// AllowUnsafeHTML allows raw HTML in markdown
	AllowUnsafeHTML bool
}

// DefaultGoldmarkConfig returns the default secure configuration used across the application.
// This ensures consistent markdown rendering behavior.
// SECURITY: AllowUnsafeHTML is disabled by default to prevent XSS attacks.
func DefaultGoldmarkConfig() GoldmarkConfig {
	return GoldmarkConfig{
		EnableGFM:           true,
		EnableTypographer:   true,
		EnableTables:        true,
		EnableStrikethrough: true,
		EnableTaskList:      true,
		EnableAutoHeadingID: true,
		AllowUnsafeHTML:     false, // SECURITY: Keep disabled to prevent XSS
	}
}

// UnsafeGoldmarkConfig returns a configuration that allows raw HTML.
// WARNING: Only use this when rendering trusted content or when HTML sanitization
// is performed separately. Using this with untrusted input can lead to XSS attacks.
func UnsafeGoldmarkConfig() GoldmarkConfig {
	cfg := DefaultGoldmarkConfig()
	cfg.AllowUnsafeHTML = true
	return cfg
}

// NewConfiguredGoldmark creates a new Goldmark instance with the default configuration.
// This is the recommended way to create Goldmark instances for consistent behavior.
func NewConfiguredGoldmark() goldmark.Markdown {
	return NewGoldmarkWithConfig(DefaultGoldmarkConfig())
}

// NewGoldmarkWithConfig creates a new Goldmark instance with the specified configuration.
// Use this when you need custom configuration that differs from the default.
func NewGoldmarkWithConfig(cfg GoldmarkConfig) goldmark.Markdown {
	var extensions []goldmark.Extender
	var parserOpts []parser.Option
	var renderOpts []renderer.Option

	// Build extensions list based on configuration
	if cfg.EnableGFM {
		extensions = append(extensions, extension.GFM)
	}
	if cfg.EnableTypographer {
		extensions = append(extensions, extension.Typographer)
	}
	if cfg.EnableTables {
		extensions = append(extensions, extension.Table)
	}
	if cfg.EnableStrikethrough {
		extensions = append(extensions, extension.Strikethrough)
	}
	if cfg.EnableTaskList {
		extensions = append(extensions, extension.TaskList)
	}

	// Build parser options
	if cfg.EnableAutoHeadingID {
		parserOpts = append(parserOpts, parser.WithAutoHeadingID())
	}

	// Build renderer options
	if cfg.AllowUnsafeHTML {
		renderOpts = append(renderOpts, html.WithUnsafe())
	}

	// Create goldmark instance with all options
	opts := []goldmark.Option{
		goldmark.WithExtensions(extensions...),
	}

	if len(parserOpts) > 0 {
		opts = append(opts, goldmark.WithParserOptions(parserOpts...))
	}

	if len(renderOpts) > 0 {
		opts = append(opts, goldmark.WithRendererOptions(renderOpts...))
	}

	return goldmark.New(opts...)
}
