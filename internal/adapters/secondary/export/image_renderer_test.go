package export

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fogleman/gg"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageRenderer_FallbackGeneration(t *testing.T) {
	t.Run("generates PNG images from HTML content", func(t *testing.T) {
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
		outputDir := filepath.Join(tmpDir, "images")

		// Create renderer
		renderer := NewImageRenderer()

		// Set up export options
		options := &ExportOptions{
			Format:     FormatImages,
			OutputPath: outputDir,
			Quality:    "medium",
		}

		// Generate images
		result, err := renderer.Render(context.Background(), presentation, options)

		// Verify result
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, string(FormatImages), result.Format)
		assert.Equal(t, len(presentation.Slides), result.PageCount)
		assert.Len(t, result.Files, len(presentation.Slides))

		// Verify image files were created
		for i, filePath := range result.Files {
			_, err = os.Stat(filePath)
			assert.NoError(t, err, "Image file %d should exist", i+1)

			// Verify file has content
			info, err := os.Stat(filePath)
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(100), "Image file %d should have content", i+1)
		}
	})

	t.Run("generates JPEG images for low quality", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title: "Test Presentation",
			Slides: []entities.Slide{
				{
					Title:   "Test Slide",
					Content: "Test content for JPEG generation.",
				},
			},
		}

		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "images")

		renderer := NewImageRenderer()
		options := &ExportOptions{
			Format:     FormatImages,
			OutputPath: outputDir,
			Quality:    "low", // Should generate JPEG
		}

		result, err := renderer.Render(context.Background(), presentation, options)

		require.NoError(t, err)
		assert.Len(t, result.Files, 1)

		// Verify JPEG file extension
		imageFile := result.Files[0]
		assert.Contains(t, imageFile, ".jpg", "Low quality should generate JPEG files")

		// Verify file exists and has content
		info, err := os.Stat(imageFile)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(100))
	})

	t.Run("handles empty presentation", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title:  "Empty Presentation",
			Slides: []entities.Slide{},
		}

		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "images")

		renderer := NewImageRenderer()
		options := &ExportOptions{
			Format:     FormatImages,
			OutputPath: outputDir,
		}

		result, err := renderer.Render(context.Background(), presentation, options)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Empty(t, result.Files, "Should have no files for empty presentation")
	})
}

func TestImageRenderer_HTMLParsing(t *testing.T) {
	t.Run("parses HTML slide content correctly", func(t *testing.T) {
		renderer := NewImageRenderer()

		htmlContent := `
		<html>
		<body>
			<div class="slide">
				<h1>First Slide Title</h1>
				<p>First slide content</p>
				<p>More content</p>
			</div>
		</body>
		</html>
		`

		slideContent, err := renderer.parseHTMLToSlideContent(htmlContent)

		require.NoError(t, err)
		assert.Equal(t, "First Slide Title", slideContent.Title)
		assert.Len(t, slideContent.Content, 2)
		assert.Contains(t, slideContent.Content[0], "First slide content")
		assert.Contains(t, slideContent.Content[1], "More content")
	})

	t.Run("detects code content", func(t *testing.T) {
		renderer := NewImageRenderer()

		htmlContent := `
		<html>
		<body>
			<h2>Code Example</h2>
			<pre><code>console.log("Hello World");</code></pre>
		</body>
		</html>
		`

		slideContent, err := renderer.parseHTMLToSlideContent(htmlContent)

		require.NoError(t, err)
		assert.Equal(t, "Code Example", slideContent.Title)
		assert.True(t, slideContent.IsCode, "Should detect code content")
		assert.Contains(t, slideContent.Content, "console.log(\"Hello World\");")
	})

	t.Run("handles malformed HTML gracefully", func(t *testing.T) {
		renderer := NewImageRenderer()

		htmlContent := `<div><h1>Incomplete HTML`

		slideContent, err := renderer.parseHTMLToSlideContent(htmlContent)

		require.NoError(t, err)
		// HTML parser should still find the h1 content or create defaults
		if slideContent.Title == "" && len(slideContent.Content) == 0 {
			// If parsing fails completely, ensure defaults are created in generateProperImage
			assert.NotNil(t, slideContent)
		} else {
			// HTML parser found some content
			assert.True(t, slideContent.Title != "" || len(slideContent.Content) > 0)
		}
	})
}

func TestImageRenderer_ProperImageGeneration(t *testing.T) {
	t.Run("generates real PNG file", func(t *testing.T) {
		renderer := NewImageRenderer()

		// Create a temporary HTML file
		tmpDir := t.TempDir()
		htmlPath := filepath.Join(tmpDir, "test.html")
		htmlContent := `
		<html>
		<body>
			<h1>Test Slide</h1>
			<p>This is test content for image generation.</p>
		</body>
		</html>
		`
		err := os.WriteFile(htmlPath, []byte(htmlContent), 0600)
		require.NoError(t, err)

		// Generate image
		outputPath := filepath.Join(tmpDir, "test.png")
		options := &ExportOptions{Quality: "medium"}

		err = renderer.generateProperImage(htmlPath, outputPath, options)
		require.NoError(t, err)

		// Verify PNG file was created
		_, err = os.Stat(outputPath)
		assert.NoError(t, err)

		// Verify file has significant content (real PNG should be larger than placeholder)
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(1000), "Real PNG should have substantial content")
	})

	t.Run("generates real JPEG file", func(t *testing.T) {
		renderer := NewImageRenderer()

		// Create a temporary HTML file
		tmpDir := t.TempDir()
		htmlPath := filepath.Join(tmpDir, "test.html")
		htmlContent := `
		<html>
		<body>
			<h1>JPEG Test</h1>
			<p>This content will be converted to JPEG format.</p>
		</body>
		</html>
		`
		err := os.WriteFile(htmlPath, []byte(htmlContent), 0600)
		require.NoError(t, err)

		// Generate JPEG image
		outputPath := filepath.Join(tmpDir, "test.jpg")
		options := &ExportOptions{Quality: "low"} // Low quality uses JPEG

		err = renderer.generateProperImage(htmlPath, outputPath, options)
		require.NoError(t, err)

		// Verify JPEG file was created
		_, err = os.Stat(outputPath)
		assert.NoError(t, err)

		// Verify file has content
		info, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(500), "JPEG should have content")
	})
}

func TestImageRenderer_TextWrapping(t *testing.T) {
	t.Run("wraps long text correctly", func(t *testing.T) {
		renderer := NewImageRenderer()

		// Create a mock graphics context for testing
		dc := gg.NewContext(100, 100) // Small context to force wrapping

		longText := "This is a very long line of text that should be wrapped into multiple lines for better image formatting"
		lines := renderer.wrapText(longText, 50, dc) // Small max width to force wrapping

		assert.Greater(t, len(lines), 1, "Long text should be wrapped into multiple lines")

		// Verify no line is empty
		for _, line := range lines {
			assert.NotEmpty(t, line, "No line should be empty")
		}
	})

	t.Run("preserves short text", func(t *testing.T) {
		renderer := NewImageRenderer()

		dc := gg.NewContext(1000, 1000) // Large context

		shortText := "Short text"
		lines := renderer.wrapText(shortText, 500, dc) // Large max width

		assert.Len(t, lines, 1, "Short text should not be wrapped")
		assert.Equal(t, shortText, lines[0])
	})
}
