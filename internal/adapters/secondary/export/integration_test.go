package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/test/builders"
)

// Integration tests that require actual Chrome/Chromium installation
// These tests are skipped if Chrome is not available

func findAvailableChrome() string {
	// Common Chrome/Chromium paths on different systems
	chromePaths := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/opt/google/chrome/chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
	}

	for _, path := range chromePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func requireChrome(t *testing.T) string {
	chromePath := findAvailableChrome()
	if chromePath == "" {
		t.Skip("Chrome/Chromium not found - skipping integration test")
	}
	return chromePath
}

func TestBrowserAutomation_RealChrome_PDFGeneration(t *testing.T) {
	chromePath := requireChrome(t)

	config := BrowserConfig{
		ExecutablePath: chromePath,
		TempDir:        os.TempDir(),
		Timeout:        30 * time.Second,
	}

	ba, err := NewBrowserAutomation(config)
	require.NoError(t, err)
	defer func() { _ = ba.Cleanup() }()

	// Create test HTML content
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Presentation</title>
    <style>
        body { font-family: Arial, sans-serif; padding: 20px; }
        .slide { page-break-after: always; margin-bottom: 20px; }
        h1 { color: #333; }
    </style>
</head>
<body>
    <div class="slide">
        <h1>Slide 1: Introduction</h1>
        <p>This is the first slide of our test presentation.</p>
        <ul>
            <li>Point 1</li>
            <li>Point 2</li>
            <li>Point 3</li>
        </ul>
    </div>
    <div class="slide">
        <h1>Slide 2: Content</h1>
        <p>This is the second slide with some content.</p>
        <p>Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>
    </div>
</body>
</html>`

	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "test-presentation-*.html")
	require.NoError(t, err)
	defer func() { _ = os.Remove(htmlFile.Name()) }()

	_, err = htmlFile.WriteString(htmlContent)
	require.NoError(t, err)
	_ = htmlFile.Close()

	// Create output PDF path
	outputDir := filepath.Join(os.TempDir(), "export-integration-test")
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(outputDir) }()

	outputPath := filepath.Join(outputDir, "test-output.pdf")

	// Test PDF generation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options := &PDFOptions{
		PageSize:     "A4",
		Landscape:    false,
		MarginTop:    "1in",
		MarginBottom: "1in",
		MarginLeft:   "1in",
		MarginRight:  "1in",
	}

	err = ba.ConvertHTMLToPDF(ctx, htmlFile.Name(), outputPath, options)
	require.NoError(t, err)

	// Verify PDF was created
	stat, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.True(t, stat.Size() > 1000, "PDF file should be larger than 1KB")

	// Verify PDF content (basic check for PDF header)
	pdfContent, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(pdfContent), "%PDF"), "File should start with PDF header")
}

func TestBrowserAutomation_RealChrome_ImageGeneration(t *testing.T) {
	chromePath := requireChrome(t)

	config := BrowserConfig{
		ExecutablePath: chromePath,
		TempDir:        os.TempDir(),
		Timeout:        30 * time.Second,
	}

	ba, err := NewBrowserAutomation(config)
	require.NoError(t, err)
	defer func() { _ = ba.Cleanup() }()

	// Create test HTML content for screenshot
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Slide</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 0; 
            padding: 20px; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            height: 100vh;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
        }
        h1 { font-size: 3em; margin-bottom: 20px; }
        p { font-size: 1.5em; text-align: center; }
    </style>
</head>
<body>
    <h1>Integration Test Slide</h1>
    <p>This slide is generated for testing browser automation with real Chrome</p>
    <p>Current time: ` + time.Now().Format("2006-01-02 15:04:05") + `</p>
</body>
</html>`

	// Create temporary HTML file
	htmlFile, err := os.CreateTemp("", "test-slide-*.html")
	require.NoError(t, err)
	defer func() { _ = os.Remove(htmlFile.Name()) }()

	_, err = htmlFile.WriteString(htmlContent)
	require.NoError(t, err)
	_ = htmlFile.Close()

	// Create output image path
	outputDir := filepath.Join(os.TempDir(), "export-integration-test")
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(outputDir) }()

	outputPath := filepath.Join(outputDir, "test-slide.png")

	// Test image generation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options := &ImageOptions{
		Width:   1920,
		Height:  1080,
		Quality: "high",
	}

	err = ba.ConvertHTMLToImage(ctx, htmlFile.Name(), outputPath, options)
	require.NoError(t, err)

	// Verify image was created
	stat, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.True(t, stat.Size() > 5000, "Image file should be larger than 5KB")

	// Verify PNG content (basic check for PNG header)
	imageContent, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, imageContent[:4], "File should start with PNG header")
}

func TestExportService_RealChrome_EndToEndExport(t *testing.T) {
	chromePath := requireChrome(t)

	// Create export service with real Chrome
	tempDir := filepath.Join(os.TempDir(), "export-e2e-test")
	service, err := NewService(tempDir)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Override the default PDF renderer with one that uses real Chrome
	browserConfig := BrowserConfig{
		ExecutablePath: chromePath,
		TempDir:        tempDir,
		Timeout:        30 * time.Second,
	}

	browserAutomation, err := NewBrowserAutomation(browserConfig)
	require.NoError(t, err)
	defer func() { _ = browserAutomation.Cleanup() }()

	pdfRenderer := &PDFRenderer{
		browserAutomation: browserAutomation,
	}
	service.RegisterRenderer(FormatPDF, pdfRenderer)

	// Create a test presentation
	presentation := builders.NewPresentationBuilder().
		WithTitle("End-to-End Export Test").
		WithSlides([]entities.Slide{
			{
				ID:      "slide-1",
				Title:   "Welcome",
				Content: "# Welcome to SliCLI\n\nThis is a test presentation for end-to-end export testing.",
				HTML:    "<h1>Welcome to SliCLI</h1><p>This is a test presentation for end-to-end export testing.</p>",
			},
			{
				ID:      "slide-2",
				Title:   "Features",
				Content: "## Key Features\n\n- Fast rendering\n- Multiple export formats\n- Plugin support",
				HTML:    "<h2>Key Features</h2><ul><li>Fast rendering</li><li>Multiple export formats</li><li>Plugin support</li></ul>",
			},
		}).
		Build()

	// Test PDF export
	t.Run("PDF Export", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "test-presentation.pdf")

		options := &ExportOptions{
			Format:      FormatPDF,
			OutputPath:  outputPath,
			Quality:     "high",
			PageSize:    "A4",
			Orientation: "portrait",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := service.Export(ctx, presentation, options)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, string(FormatPDF), result.Format)
		assert.NotZero(t, result.FileSize)
		assert.NotZero(t, result.PageCount)

		// Verify PDF file exists and has content
		stat, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.True(t, stat.Size() > 2000, "PDF should be larger than 2KB for 2 slides")

		// Verify it's a valid PDF
		pdfContent, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(string(pdfContent), "%PDF"), "Should be valid PDF")
	})

	// Test Images export
	t.Run("Images Export", func(t *testing.T) {
		outputPath := filepath.Join(tempDir, "test-presentation-images.zip")

		options := &ExportOptions{
			Format:     FormatImages,
			OutputPath: outputPath,
			Quality:    "high",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := service.Export(ctx, presentation, options)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, string(FormatImages), result.Format)
		assert.NotZero(t, result.FileSize)

		// Verify ZIP file exists
		stat, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.True(t, stat.Size() > 1000, "Images ZIP should be larger than 1KB")
	})
}

func TestExportService_RealChrome_ErrorHandling(t *testing.T) {
	_ = requireChrome(t) // Use underscore to indicate intentionally unused

	tempDir := filepath.Join(os.TempDir(), "export-error-test")
	service, err := NewService(tempDir)
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test with malformed HTML
	t.Run("Malformed HTML", func(t *testing.T) {
		presentation := builders.NewPresentationBuilder().
			WithTitle("Error Test").
			WithSlides([]entities.Slide{
				{
					ID:      "slide-1",
					Title:   "Broken",
					Content: "# Broken Slide",
					HTML:    "<html><body><h1>Unclosed tag<h1><p>Missing closing tags",
				},
			}).
			Build()

		outputPath := filepath.Join(tempDir, "error-test.pdf")
		options := &ExportOptions{
			Format:     FormatPDF,
			OutputPath: outputPath,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := service.Export(ctx, presentation, options)
		// Should handle malformed HTML gracefully
		if err != nil {
			assert.NotNil(t, result)
			assert.False(t, result.Success)
		} else {
			// Chrome is quite forgiving with malformed HTML
			assert.True(t, result.Success)
		}
	})

	// Test timeout handling
	t.Run("Timeout Handling", func(t *testing.T) {
		// Create presentation with complex content that might take time
		presentation := builders.NewPresentationBuilder().
			WithTitle("Timeout Test").
			WithSlides([]entities.Slide{
				{
					ID:    "slide-1",
					Title: "Complex",
					HTML: `<html><body>
						<script>
							// Simulate slow loading
							for(let i = 0; i < 1000000; i++) {
								document.body.innerHTML += '<div>Loading...</div>';
							}
						</script>
					</body></html>`,
				},
			}).
			Build()

		outputPath := filepath.Join(tempDir, "timeout-test.pdf")
		options := &ExportOptions{
			Format:     FormatPDF,
			OutputPath: outputPath,
		}

		// Use very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		result, err := service.Export(ctx, presentation, options)
		// Should timeout gracefully
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "timeout")
	})
}

func TestBrowserAutomation_RealChrome_ProcessManagement(t *testing.T) {
	chromePath := requireChrome(t)

	config := BrowserConfig{
		ExecutablePath: chromePath,
		TempDir:        os.TempDir(),
		Timeout:        30 * time.Second,
	}

	ba, err := NewBrowserAutomation(config)
	require.NoError(t, err)

	// Test process tracking
	initialProcesses := ba.GetActiveProcessCount()

	// Create simple HTML file
	htmlContent := `<html><body><h1>Process Test</h1></body></html>`
	htmlFile, err := os.CreateTemp("", "process-test-*.html")
	require.NoError(t, err)
	defer func() { _ = os.Remove(htmlFile.Name()) }()

	_, err = htmlFile.WriteString(htmlContent)
	require.NoError(t, err)
	_ = htmlFile.Close()

	outputPath := filepath.Join(os.TempDir(), "process-test.pdf")
	defer func() { _ = os.Remove(outputPath) }()

	// Execute conversion
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = ba.ConvertHTMLToPDF(ctx, htmlFile.Name(), outputPath, nil)
	require.NoError(t, err)

	// Process should complete and be cleaned up
	finalProcesses := ba.GetActiveProcessCount()
	assert.Equal(t, initialProcesses, finalProcesses, "Processes should be cleaned up after completion")

	// Test cleanup
	err = ba.Cleanup()
	require.NoError(t, err)

	// All processes should be terminated
	cleanupProcesses := ba.GetActiveProcessCount()
	assert.Equal(t, 0, cleanupProcesses, "All processes should be terminated after cleanup")
}

// Benchmark tests with real Chrome
func BenchmarkBrowserAutomation_RealChrome_PDFGeneration(b *testing.B) {
	chromePath := findAvailableChrome()
	if chromePath == "" {
		b.Skip("Chrome/Chromium not found - skipping benchmark")
	}

	config := BrowserConfig{
		ExecutablePath: chromePath,
		TempDir:        os.TempDir(),
		Timeout:        30 * time.Second,
	}

	ba, err := NewBrowserAutomation(config)
	require.NoError(b, err)
	defer func() { _ = ba.Cleanup() }()

	// Create test HTML
	htmlContent := `<!DOCTYPE html>
<html><body>
	<h1>Benchmark Test</h1>
	<p>This is a simple slide for benchmarking PDF generation performance.</p>
</body></html>`

	htmlFile, err := os.CreateTemp("", "benchmark-*.html")
	require.NoError(b, err)
	defer func() { _ = os.Remove(htmlFile.Name()) }()

	_, err = htmlFile.WriteString(htmlContent)
	require.NoError(b, err)
	_ = htmlFile.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("benchmark-%d.pdf", i))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := ba.ConvertHTMLToPDF(ctx, htmlFile.Name(), outputPath, nil)
		cancel()

		require.NoError(b, err)
		_ = os.Remove(outputPath) // Cleanup
	}
}
