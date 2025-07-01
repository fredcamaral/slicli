package renderer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

func TestSlideRendererAdapter_RenderSlide(t *testing.T) {
	renderer := NewSlideRendererAdapter()

	t.Run("render markdown content", func(t *testing.T) {
		slide := &entities.Slide{
			Index:   0,
			Title:   "Test Slide",
			Content: "# Test Slide\n\nThis is **bold** and this is *italic*",
			Notes:   "Speaker notes here",
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		assert.Equal(t, slide, result.Slide)
		assert.Contains(t, result.HTML, `<h1 id="test-slide">Test Slide</h1>`)
		assert.Contains(t, result.HTML, "<strong>bold</strong>")
		assert.Contains(t, result.HTML, "<em>italic</em>")
		assert.Equal(t, "<p>Speaker notes here</p>", result.NotesHTML)
	})

	t.Run("render with pre-rendered HTML", func(t *testing.T) {
		slide := &entities.Slide{
			Index:   0,
			Title:   "Pre-rendered",
			Content: "# Original",
			HTML:    "<h1>Pre-rendered HTML</h1>",
			Notes:   "Notes",
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		// Should use the pre-rendered HTML
		assert.Equal(t, "<h1>Pre-rendered HTML</h1>", result.HTML)
		assert.Equal(t, "<p>Notes</p>", result.NotesHTML)
	})

	t.Run("render without notes", func(t *testing.T) {
		slide := &entities.Slide{
			Index:   0,
			Title:   "No Notes",
			Content: "# No Notes\n\nJust content",
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		assert.Contains(t, result.HTML, `<h1 id="no-notes">No Notes</h1>`)
		assert.Empty(t, result.NotesHTML)
	})

	t.Run("render nil slide", func(t *testing.T) {
		_, err := renderer.RenderSlide(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "slide cannot be nil")
	})

	t.Run("render with GFM features", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "GFM Test",
			Content: `# GFM Features

## Task List
- [x] Completed
- [ ] Not completed

## Table
| Header | Value |
|--------|-------|
| Cell 1 | Data 1 |

## Strikethrough
~~deleted text~~

## Code
` + "```go\nfunc main() {}\n```",
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		// Check task list
		assert.Contains(t, result.HTML, `<input checked="" disabled=""`)
		assert.Contains(t, result.HTML, `<input disabled=""`)

		// Check table
		assert.Contains(t, result.HTML, "<table>")
		assert.Contains(t, result.HTML, "<th>Header</th>")

		// Check strikethrough
		assert.Contains(t, result.HTML, "<del>deleted text</del>")

		// Check code block
		assert.Contains(t, result.HTML, "<pre><code")
		assert.Contains(t, result.HTML, "func main() {}")
	})

	t.Run("render with links and images", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "Links",
			Content: `# Links and Images

[Link text](https://example.com)

![Alt text](image.png)`,
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		assert.Contains(t, result.HTML, `<a href="https://example.com">Link text</a>`)
		assert.Contains(t, result.HTML, `<img src="image.png" alt="Alt text"`)
	})

	t.Run("render with nested structures", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "Nested",
			Content: `# Nested Structures

> Blockquote with **bold** text
> 
> Second paragraph

1. First item
   - Nested bullet
   - Another nested
2. Second item

   Code in list:
   ` + "```\n   code block\n   ```",
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		assert.Contains(t, result.HTML, "<blockquote>")
		assert.Contains(t, result.HTML, "<ol>")
		assert.Contains(t, result.HTML, "<ul>")
		assert.Contains(t, result.HTML, "code block")
	})

	t.Run("render with raw HTML", func(t *testing.T) {
		slide := &entities.Slide{
			Index: 0,
			Title: "Raw HTML",
			Content: `# Raw HTML Test

<div class="custom">
  <span style="color: red;">Custom HTML</span>
</div>`,
		}

		result, err := renderer.RenderSlide(slide)
		require.NoError(t, err)

		// Raw HTML should be preserved (WithUnsafe option)
		assert.Contains(t, result.HTML, `<div class="custom">`)
		assert.Contains(t, result.HTML, `<span style="color: red;">Custom HTML</span>`)
	})
}

func TestRenderNotes(t *testing.T) {
	renderer := NewSlideRendererAdapter()

	t.Run("render simple notes", func(t *testing.T) {
		notes := renderer.renderNotes("Simple notes")
		assert.Equal(t, "<p>Simple notes</p>", notes)
	})

	t.Run("render empty notes", func(t *testing.T) {
		notes := renderer.renderNotes("")
		assert.Empty(t, notes)
	})

	t.Run("render multi-line notes", func(t *testing.T) {
		notes := renderer.renderNotes("Line 1\nLine 2")
		assert.Equal(t, "<p>Line 1\nLine 2</p>", notes)
	})
}
