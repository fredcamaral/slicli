package export

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPDFRenderer_FallbackGeneration(t *testing.T) {
	t.Run("generates PDF from HTML content", func(t *testing.T) {
		// Create a simple presentation
		presentation := &entities.Presentation{
			Title: "Test Presentation",
			Slides: []entities.Slide{
				{
					Title:   "First Slide",
					Content: "This is the first slide content.",
				},
				{
					Title:   "Second Slide",
					Content: "This is the second slide with some **markdown** content.",
				},
			},
		}

		// Create temporary directory for output
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "test-presentation.pdf")

		// Create renderer
		renderer := NewPDFRenderer()

		// Set up export options
		options := &ExportOptions{
			Format:     FormatPDF,
			OutputPath: outputPath,
		}

		// Generate PDF
		result, err := renderer.Render(context.Background(), presentation, options)

		// Verify result
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, string(FormatPDF), result.Format)
		assert.Equal(t, outputPath, result.OutputPath)
		assert.Equal(t, len(presentation.Slides), result.PageCount)

		// Verify PDF file was created
		_, err = os.Stat(outputPath)
		assert.NoError(t, err)

		// Verify file has content (PDF should be more than just headers)
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(100), "PDF file should have content")
	})

	t.Run("handles empty presentation", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title:  "Empty Presentation",
			Slides: []entities.Slide{},
		}

		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "empty-presentation.pdf")

		renderer := NewPDFRenderer()
		options := &ExportOptions{
			Format:     FormatPDF,
			OutputPath: outputPath,
		}

		result, err := renderer.Render(context.Background(), presentation, options)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)

		// Should still create a PDF file
		_, err = os.Stat(outputPath)
		assert.NoError(t, err)
	})
}

func TestPDFRenderer_HTMLParsing(t *testing.T) {
	t.Run("parses HTML slides correctly", func(t *testing.T) {
		renderer := NewPDFRenderer()

		htmlContent := `
		<html>
		<body>
			<div class="slide">
				<h1>First Slide Title</h1>
				<p>First slide content</p>
				<p>More content</p>
			</div>
			<div class="slide">
				<h2>Second Slide</h2>
				<ul>
					<li>Item 1</li>
					<li>Item 2</li>
				</ul>
				<pre><code>console.log("Hello World");</code></pre>
			</div>
		</body>
		</html>
		`

		slides, err := renderer.parseHTMLToSlides(htmlContent)

		require.NoError(t, err)
		assert.Len(t, slides, 2)

		// First slide
		assert.Equal(t, "First Slide Title", slides[0].Title)
		assert.Len(t, slides[0].Content, 2)
		assert.Contains(t, slides[0].Content[0], "First slide content")

		// Second slide
		assert.Equal(t, "Second Slide", slides[1].Title)
		assert.True(t, slides[1].IsCode, "Should detect code content")
		assert.Contains(t, slides[1].Content, "Item 1")
	})

	t.Run("handles malformed HTML gracefully", func(t *testing.T) {
		renderer := NewPDFRenderer()

		htmlContent := `<div><h1>Incomplete HTML`

		slides, err := renderer.parseHTMLToSlides(htmlContent)

		require.NoError(t, err)
		// Should create at least one default slide
		assert.GreaterOrEqual(t, len(slides), 1)
	})
}

func TestPDFRenderer_TextCleaning(t *testing.T) {
	t.Run("cleans HTML entities", func(t *testing.T) {
		renderer := NewPDFRenderer()

		input := "Hello&nbsp;world &amp; &lt;test&gt; &quot;quoted&quot;"
		expected := "Hello world & <test> \"quoted\""

		result := renderer.cleanTextForPDF(input)
		assert.Equal(t, expected, result)
	})

	t.Run("normalizes whitespace", func(t *testing.T) {
		renderer := NewPDFRenderer()

		input := "Hello    world\n\nwith   multiple   spaces"
		result := renderer.cleanTextForPDF(input)

		// Should normalize to single spaces
		assert.NotContains(t, result, "  ")
		assert.Contains(t, result, "Hello world")
	})
}

func TestPDFRenderer_LineSplitting(t *testing.T) {
	t.Run("splits long lines correctly", func(t *testing.T) {
		renderer := NewPDFRenderer()

		input := "This is a very long line that should be split into multiple lines for better PDF formatting"
		lines := renderer.splitLongLines(input, 30)

		assert.Greater(t, len(lines), 1)
		for _, line := range lines {
			assert.LessOrEqual(t, len(line), 30)
		}
	})

	t.Run("preserves short lines", func(t *testing.T) {
		renderer := NewPDFRenderer()

		input := "Short line"
		lines := renderer.splitLongLines(input, 50)

		assert.Len(t, lines, 1)
		assert.Equal(t, input, lines[0])
	})
}
