package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoldmarkParser_Parse(t *testing.T) {
	parser := NewGoldmarkParser()
	ctx := context.Background()

	t.Run("parse with frontmatter and slides", func(t *testing.T) {
		content := []byte(`---
title: Test Presentation
author: John Doe
theme: default
---

# First Slide

Some content with **bold** and *italic*

Note: This is a speaker note

---

## Second Slide

- Bullet point 1
- Bullet point 2

Note: Another speaker note`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		// Check frontmatter
		assert.NotNil(t, result.Frontmatter)
		assert.Equal(t, "Test Presentation", result.Frontmatter["title"])
		assert.Equal(t, "John Doe", result.Frontmatter["author"])
		assert.Equal(t, "default", result.Frontmatter["theme"])

		// Check slides
		assert.Len(t, result.Slides, 2)

		// First slide
		assert.Equal(t, 0, result.Slides[0].Index)
		assert.Contains(t, result.Slides[0].Content, "# First Slide")
		assert.Contains(t, result.Slides[0].Content, "**bold**")
		assert.Equal(t, "This is a speaker note", result.Slides[0].Notes)

		// Second slide
		assert.Equal(t, 1, result.Slides[1].Index)
		assert.Contains(t, result.Slides[1].Content, "## Second Slide")
		assert.Contains(t, result.Slides[1].Content, "- Bullet point 1")
		assert.Equal(t, "Another speaker note", result.Slides[1].Notes)
	})

	t.Run("parse without frontmatter", func(t *testing.T) {
		content := []byte(`# Title Slide

Content without frontmatter

---

## Another Slide`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Nil(t, result.Frontmatter)
		assert.Len(t, result.Slides, 2)
	})

	t.Run("parse single slide", func(t *testing.T) {
		content := []byte(`# Single Slide

Just one slide with no separators`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Len(t, result.Slides, 1)
		assert.Contains(t, result.Slides[0].Content, "# Single Slide")
	})

	t.Run("parse with code blocks", func(t *testing.T) {
		content := []byte("# Code Example\n\n```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```")

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Len(t, result.Slides, 1)
		assert.Contains(t, result.Slides[0].Content, "```go")
		assert.Contains(t, result.Slides[0].Content, "fmt.Println")
	})

	t.Run("parse with multiple notes", func(t *testing.T) {
		content := []byte(`# Slide

Content

Note: First note
Note: Second note`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Len(t, result.Slides, 1)
		assert.Equal(t, "First note\nSecond note", result.Slides[0].Notes)
	})

	t.Run("parse empty content", func(t *testing.T) {
		content := []byte("")

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Len(t, result.Slides, 1)
		assert.Equal(t, "", result.Slides[0].Content)
	})

	t.Run("parse with malformed frontmatter", func(t *testing.T) {
		content := []byte(`---
title: Test
invalid yaml [
---

# Slide`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		// Should treat entire content as slide when frontmatter is invalid
		assert.Nil(t, result.Frontmatter)
		assert.Len(t, result.Slides, 2) // Split on the second ---
		assert.Contains(t, result.Slides[0].Content, "title: Test")
		assert.Contains(t, result.Slides[1].Content, "# Slide")
	})

	t.Run("parse with GFM features", func(t *testing.T) {
		content := []byte(`# GFM Features

- [x] Completed task
- [ ] Incomplete task

| Header | Value |
|--------|-------|
| Row 1  | Data  |

~~strikethrough~~`)

		result, err := parser.Parse(ctx, content)
		require.NoError(t, err)

		assert.Len(t, result.Slides, 1)
		assert.Contains(t, result.Slides[0].Content, "[x] Completed task")
		assert.Contains(t, result.Slides[0].Content, "| Header | Value |")
		assert.Contains(t, result.Slides[0].Content, "~~strikethrough~~")
	})
}

func TestExtractFrontmatter(t *testing.T) {
	t.Run("valid frontmatter", func(t *testing.T) {
		content := []byte(`---
title: Test
key: value
number: 42
---
# Content`)

		fm, remaining := extractFrontmatter(content)

		assert.NotNil(t, fm)
		assert.Equal(t, "Test", fm["title"])
		assert.Equal(t, "value", fm["key"])
		assert.Equal(t, 42, fm["number"])
		assert.Equal(t, "# Content", string(remaining))
	})

	t.Run("no frontmatter", func(t *testing.T) {
		content := []byte("# Content without frontmatter")

		fm, remaining := extractFrontmatter(content)

		assert.Nil(t, fm)
		assert.Equal(t, content, remaining)
	})

	t.Run("unclosed frontmatter", func(t *testing.T) {
		content := []byte(`---
title: Test
# Content`)

		fm, remaining := extractFrontmatter(content)

		assert.Nil(t, fm)
		assert.Equal(t, content, remaining)
	})

	t.Run("empty frontmatter", func(t *testing.T) {
		content := []byte(`---
---
# Content`)

		fm, remaining := extractFrontmatter(content)

		assert.NotNil(t, fm)
		assert.Empty(t, fm)
		assert.Equal(t, "# Content", string(remaining))
	})
}

func TestSplitSlides(t *testing.T) {
	t.Run("multiple slides", func(t *testing.T) {
		content := []byte("# Slide 1\n---\n# Slide 2\n---\n# Slide 3")

		slides := splitSlides(content)

		assert.Len(t, slides, 3)
		assert.Equal(t, "# Slide 1", string(slides[0]))
		assert.Equal(t, "# Slide 2", string(slides[1]))
		assert.Equal(t, "# Slide 3", string(slides[2]))
	})

	t.Run("single slide", func(t *testing.T) {
		content := []byte("# Only one slide")

		slides := splitSlides(content)

		assert.Len(t, slides, 1)
		assert.Equal(t, "# Only one slide", string(slides[0]))
	})

	t.Run("slides with empty sections", func(t *testing.T) {
		content := []byte("# Slide 1\n---\n\n---\n# Slide 2")

		slides := splitSlides(content)

		// Empty slides should be filtered out
		assert.Len(t, slides, 2)
		assert.Equal(t, "# Slide 1", string(slides[0]))
		assert.Equal(t, "# Slide 2", string(slides[1]))
	})

	t.Run("different line endings", func(t *testing.T) {
		content := []byte("# Slide 1\r\n---\r\n# Slide 2")

		slides := splitSlides(content)

		assert.Len(t, slides, 2)
		assert.Equal(t, "# Slide 1", string(slides[0]))
		assert.Equal(t, "# Slide 2", string(slides[1]))
	})
}
