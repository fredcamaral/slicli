package renderer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

func TestTemplateRenderer_RenderPresentation(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("render complete presentation", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title:  "Test Presentation",
			Author: "John Doe",
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Theme:  "default",
			Slides: []entities.Slide{
				{
					Index: 0,
					Title: "First Slide",
					HTML:  "<h1>First Slide</h1><p>Content</p>",
					Notes: "Speaker notes",
				},
				{
					Index: 1,
					Title: "Second Slide",
					HTML:  "<h2>Second Slide</h2><ul><li>Item 1</li><li>Item 2</li></ul>",
				},
			},
		}

		html, err := renderer.RenderPresentation(ctx, presentation)
		require.NoError(t, err)

		htmlStr := string(html)

		// Check basic structure
		assert.Contains(t, htmlStr, "<!DOCTYPE html>")
		assert.Contains(t, htmlStr, "<title>Test Presentation</title>")
		assert.Contains(t, htmlStr, "John Doe")
		assert.Contains(t, htmlStr, "2024-01-01")
		assert.Contains(t, htmlStr, `data-theme="default"`)

		// Check slides
		assert.Contains(t, htmlStr, "<h1>First Slide</h1>")
		assert.Contains(t, htmlStr, "<h2>Second Slide</h2>")
		assert.Contains(t, htmlStr, "Speaker notes")

		// Check controls
		assert.Contains(t, htmlStr, "previousSlide()")
		assert.Contains(t, htmlStr, "nextSlide()")

		// Check slide counter
		assert.Contains(t, htmlStr, `<span id="total-slides">2</span>`)
	})

	t.Run("render minimal presentation", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title: "Minimal",
			Theme: "default",
			Slides: []entities.Slide{
				{
					Index: 0,
					Title: "Only Slide",
					HTML:  "<h1>Only Slide</h1>",
				},
			},
		}

		html, err := renderer.RenderPresentation(ctx, presentation)
		require.NoError(t, err)

		htmlStr := string(html)
		assert.Contains(t, htmlStr, "<title>Minimal</title>")
		assert.Contains(t, htmlStr, "<h1>Only Slide</h1>")

		// Should not contain author/date if not provided
		assert.NotContains(t, htmlStr, "<div></div>")
	})

	t.Run("render with metadata", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title: "With Metadata",
			Theme: "default",
			Metadata: map[string]interface{}{
				"version": "1.0",
				"tags":    []string{"test", "demo"},
			},
			Slides: []entities.Slide{
				{Index: 0, Title: "Slide", HTML: "<p>Content</p>"},
			},
		}

		html, err := renderer.RenderPresentation(ctx, presentation)
		require.NoError(t, err)

		// Basic check that it renders without error
		assert.Contains(t, string(html), "<p>Content</p>")
	})

	t.Run("render with special characters", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title:  "Test & Demo <Presentation>",
			Author: "Jane & John",
			Theme:  "default",
			Slides: []entities.Slide{
				{
					Index: 0,
					Title: "Code Example",
					HTML:  `<pre><code>if x &lt; 10 &amp;&amp; y &gt; 5 { }</code></pre>`,
				},
			},
		}

		html, err := renderer.RenderPresentation(ctx, presentation)
		require.NoError(t, err)

		htmlStr := string(html)
		// Title should be escaped in <title> tag
		assert.Contains(t, htmlStr, "<title>Test &amp; Demo &lt;Presentation&gt;</title>")
		// HTML content should be preserved
		assert.Contains(t, htmlStr, `<pre><code>if x &lt; 10 &amp;&amp; y &gt; 5 { }</code></pre>`)
	})
}

func TestTemplateRenderer_RenderSlide(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("render slide with notes", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "Test Slide",
			HTML:  "<h1>Test Slide</h1><p>Some content</p>",
			Notes: "These are speaker notes",
		}

		html, err := renderer.RenderSlide(ctx, slide)
		require.NoError(t, err)

		htmlStr := string(html)
		assert.Contains(t, htmlStr, "<h1>Test Slide</h1>")
		assert.Contains(t, htmlStr, "<p>Some content</p>")
		assert.Contains(t, htmlStr, `<div class="speaker-notes" style="display: none;">`)
		assert.Contains(t, htmlStr, "These are speaker notes")
	})

	t.Run("render slide without notes", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "No Notes",
			HTML:  "<h2>No Notes</h2>",
		}

		html, err := renderer.RenderSlide(ctx, slide)
		require.NoError(t, err)

		htmlStr := string(html)
		assert.Contains(t, htmlStr, "<h2>No Notes</h2>")
		assert.NotContains(t, htmlStr, "speaker-notes")
	})

	t.Run("render slide with complex HTML", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "Complex",
			HTML: `<h1>Complex Slide</h1>
<ul>
    <li>Item 1</li>
    <li>Item 2</li>
</ul>
<table>
    <tr><th>Header</th><th>Value</th></tr>
    <tr><td>Row 1</td><td>Data</td></tr>
</table>
<pre><code class="language-go">func main() {
    fmt.Println("Hello")
}</code></pre>`,
		}

		html, err := renderer.RenderSlide(ctx, slide)
		require.NoError(t, err)

		htmlStr := string(html)
		assert.Contains(t, htmlStr, "<ul>")
		assert.Contains(t, htmlStr, "<table>")
		assert.Contains(t, htmlStr, `<code class="language-go">`)
	})
}

func TestNewTemplateRenderer(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		renderer, err := NewTemplateRenderer()
		require.NoError(t, err)
		assert.NotNil(t, renderer)
		assert.NotNil(t, renderer.templates)
	})
}

func TestTemplateStyles(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	require.NoError(t, err)

	ctx := context.Background()

	presentation := &entities.Presentation{
		Title: "Style Test",
		Theme: "default",
		Slides: []entities.Slide{
			{
				Index: 0,
				Title: "All Elements",
				HTML: strings.Join([]string{
					"<h1>Heading 1</h1>",
					"<h2>Heading 2</h2>",
					"<h3>Heading 3</h3>",
					"<p>Paragraph with <strong>bold</strong> and <em>italic</em></p>",
					"<blockquote>A quote</blockquote>",
					"<pre><code>Code block</code></pre>",
					"<ul><li>Unordered</li></ul>",
					"<ol><li>Ordered</li></ol>",
					"<table><tr><th>Header</th></tr><tr><td>Cell</td></tr></table>",
				}, "\n"),
			},
		},
	}

	html, err := renderer.RenderPresentation(ctx, presentation)
	require.NoError(t, err)

	htmlStr := string(html)

	// Check that styles are included
	assert.Contains(t, htmlStr, "font-family:")
	assert.Contains(t, htmlStr, ".slide h1")
	assert.Contains(t, htmlStr, ".slide h2")
	assert.Contains(t, htmlStr, ".slide h3")
	assert.Contains(t, htmlStr, ".slide pre")
	assert.Contains(t, htmlStr, ".slide code")
	assert.Contains(t, htmlStr, ".slide blockquote")
	assert.Contains(t, htmlStr, ".slide table")
}
