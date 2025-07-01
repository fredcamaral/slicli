package export

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/fogleman/gg"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/net/html"
)

// ImageRenderer implements export to image files (PNG/JPG)
type ImageRenderer struct {
	htmlRenderer      *HTMLRenderer
	browserAutomation *BrowserAutomation
}

// NewImageRenderer creates a new image renderer
func NewImageRenderer() *ImageRenderer {
	// Initialize with default browser config
	browserConfig := BrowserConfig{}
	browserAutomation, _ := NewBrowserAutomation(browserConfig)

	return &ImageRenderer{
		htmlRenderer:      NewHTMLRenderer(),
		browserAutomation: browserAutomation,
	}
}

// NewImageRendererWithBrowser creates a new image renderer with custom browser config
func NewImageRendererWithBrowser(browserConfig BrowserConfig) (*ImageRenderer, error) {
	browserAutomation, err := NewBrowserAutomation(browserConfig)
	if err != nil {
		return nil, fmt.Errorf("initializing browser automation: %w", err)
	}

	return &ImageRenderer{
		htmlRenderer:      NewHTMLRenderer(),
		browserAutomation: browserAutomation,
	}, nil
}

// Render exports the presentation to image files
func (r *ImageRenderer) Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	// Prepare output directory
	outputDir := options.OutputPath
	if outputDir != "" && outputDir[len(outputDir)-1] != '/' {
		outputDir += "/"
	}

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Create temporary HTML file for each slide
	tmpDir := filepath.Dir(outputDir)

	var generatedFiles []string
	var totalSize int64

	for i, slide := range presentation.Slides {
		// Create individual slide presentation
		singleSlidePresentation := &entities.Presentation{
			Title:    presentation.Title,
			Author:   presentation.Author,
			Date:     presentation.Date,
			Theme:    presentation.Theme,
			Slides:   []entities.Slide{slide},
			Metadata: presentation.Metadata,
		}

		// Create temporary HTML file for this slide
		tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("slicli-slide-%d-*.html", i))
		if err != nil {
			return nil, fmt.Errorf("creating temporary HTML file for slide %d: %w", i, err)
		}
		if err := tmpFile.Close(); err != nil {
			return nil, fmt.Errorf("closing temporary file for slide %d: %w", i, err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		// Generate HTML for this slide
		htmlOptions := &ExportOptions{
			Format:          FormatHTML,
			OutputPath:      tmpFile.Name(),
			Theme:           options.Theme,
			IncludeNotes:    false, // Don't include notes in image exports
			IncludeMetadata: options.IncludeMetadata,
			Metadata:        options.Metadata,
		}

		_, err = r.htmlRenderer.Render(ctx, singleSlidePresentation, htmlOptions)
		if err != nil {
			return nil, fmt.Errorf("generating HTML for slide %d: %w", i, err)
		}

		// Generate image filename
		imageFormat := "png"
		if options.Quality == "low" {
			imageFormat = "jpg"
		}

		imagePath := filepath.Join(outputDir, fmt.Sprintf("slide-%03d.%s", i+1, imageFormat))

		// Convert HTML to image
		err = r.convertHTMLToImage(tmpFile.Name(), imagePath, options)
		if err != nil {
			return nil, fmt.Errorf("converting slide %d to image: %w", i, err)
		}

		generatedFiles = append(generatedFiles, imagePath)

		// Add to total size
		if size, err := GetFileSize(imagePath); err == nil {
			totalSize += size
		}
	}

	return &ExportResult{
		Success:    true,
		Format:     string(FormatImages),
		OutputPath: outputDir,
		FileSize:   totalSize,
		PageCount:  len(presentation.Slides),
		Files:      generatedFiles,
	}, nil
}

// convertHTMLToImage converts an HTML file to an image using browser automation
func (r *ImageRenderer) convertHTMLToImage(htmlPath, outputPath string, options *ExportOptions) error {
	// Check if browser automation is available
	if r.browserAutomation == nil {
		return r.fallbackImageGeneration(htmlPath, outputPath, options)
	}

	ctx := context.Background()
	if err := r.browserAutomation.IsAvailable(ctx); err != nil {
		// Fallback to placeholder image generation if browser is not available
		return r.fallbackImageGeneration(htmlPath, outputPath, options)
	}

	// Determine image format
	imageFormat := "png"
	if options.Quality == "low" {
		imageFormat = "jpg"
	}

	// Convert export options to image options
	imageOptions := &ImageOptions{
		Quality: options.Quality,
		Format:  imageFormat,
	}

	// Use browser automation for image generation
	err := r.browserAutomation.ConvertHTMLToImage(ctx, htmlPath, outputPath, imageOptions)
	if err != nil {
		// Fallback to placeholder image generation if browser automation fails
		return r.fallbackImageGeneration(htmlPath, outputPath, options)
	}

	return nil
}

// fallbackImageGeneration creates a real image when browser automation is not available
func (r *ImageRenderer) fallbackImageGeneration(htmlPath, outputPath string, options *ExportOptions) error {
	return r.generateProperImage(htmlPath, outputPath, options)
}

// generateProperImage creates a real image from HTML content using gg graphics library
func (r *ImageRenderer) generateProperImage(htmlPath, outputPath string, options *ExportOptions) error {
	// Validate file path to prevent directory traversal
	if err := validateFilePath(htmlPath); err != nil {
		return fmt.Errorf("invalid HTML file path: %w", err)
	}

	// Read HTML content
	htmlContent, err := os.ReadFile(filepath.Clean(htmlPath)) // #nosec G304 - path validated above
	if err != nil {
		return fmt.Errorf("reading HTML file: %w", err)
	}

	// Parse HTML to extract content
	slideContent, err := r.parseHTMLToSlideContent(string(htmlContent))
	if err != nil {
		return fmt.Errorf("parsing HTML content: %w", err)
	}

	// Get image dimensions based on quality
	width, height := GetImageDimensions(options.Quality)

	// Create a new graphics context
	dc := gg.NewContext(width, height)

	// Set background color (white)
	dc.SetColor(color.White)
	dc.Clear()

	// Add slide content to image
	if err := r.addSlideContentToImage(dc, slideContent, width, height); err != nil {
		return fmt.Errorf("adding slide content to image: %w", err)
	}

	// Save image based on format
	imageFormat := strings.ToLower(filepath.Ext(outputPath))
	if imageFormat == "" {
		// Default to PNG if no extension
		imageFormat = ".png"
		outputPath += imageFormat
	}

	switch imageFormat {
	case ".jpg", ".jpeg":
		return r.saveAsJPEG(dc.Image(), outputPath, options)
	case ".png":
		return r.saveAsPNG(dc.Image(), outputPath)
	default:
		return fmt.Errorf("unsupported image format: %s", imageFormat)
	}
}

// SlideImageContent represents the content of a slide for image generation
type SlideImageContent struct {
	Title    string
	Content  []string
	IsCode   bool
	Language string
}

// parseHTMLToSlideContent extracts slide content from HTML for image generation
func (r *ImageRenderer) parseHTMLToSlideContent(htmlContent string) (*SlideImageContent, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	slideContent := &SlideImageContent{}

	// Recursively walk the HTML tree to extract content
	var walkNode func(*html.Node)
	walkNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3":
				title := r.extractTextContentFromNode(n)
				if title != "" && slideContent.Title == "" {
					slideContent.Title = title
				}
			case "p", "li":
				content := r.extractTextContentFromNode(n)
				if content != "" {
					slideContent.Content = append(slideContent.Content, content)
				}
			case "pre", "code":
				code := r.extractTextContentFromNode(n)
				if code != "" {
					slideContent.Content = append(slideContent.Content, code)
					slideContent.IsCode = true
				}
			}
		}

		// Process children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkNode(child)
		}
	}

	walkNode(doc)

	// Set default content if nothing was found
	if slideContent.Title == "" && len(slideContent.Content) == 0 {
		slideContent.Title = "Generated Slide"
		slideContent.Content = []string{"This slide was generated from HTML content by SLICLI."}
	}

	return slideContent, nil
}

// extractTextContentFromNode recursively extracts text content from HTML node
func (r *ImageRenderer) extractTextContentFromNode(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var text strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		childText := r.extractTextContentFromNode(child)
		if childText != "" {
			if text.Len() > 0 {
				text.WriteString(" ")
			}
			text.WriteString(childText)
		}
	}

	return strings.TrimSpace(text.String())
}

// addSlideContentToImage adds slide content to the graphics context
func (r *ImageRenderer) addSlideContentToImage(dc *gg.Context, slideContent *SlideImageContent, width, height int) error {
	// Calculate margins and positioning
	marginX := float64(width) * 0.1  // 10% margin
	marginY := float64(height) * 0.1 // 10% margin
	contentWidth := float64(width) - 2*marginX

	// Set up fonts
	titleFontSize := float64(width) / 25 // Responsive font size
	contentFontSize := float64(width) / 35

	currentY := marginY

	// Add title
	if slideContent.Title != "" {
		dc.SetColor(color.RGBA{45, 55, 72, 255}) // Dark gray
		// Load embedded Go font
		if err := r.loadGoFont(dc, titleFontSize); err != nil {
			return fmt.Errorf("loading title font: %w", err)
		}

		// Wrap title text if needed
		titleLines := r.wrapText(slideContent.Title, contentWidth, dc)
		for _, line := range titleLines {
			dc.DrawStringAnchored(line, marginX+contentWidth/2, currentY, 0.5, 0)
			currentY += titleFontSize * 1.2
		}
		currentY += titleFontSize * 0.5 // Extra spacing after title
	}

	// Add content
	dc.SetColor(color.RGBA{74, 85, 104, 255}) // Medium gray
	// Load content font
	if err := r.loadGoFont(dc, contentFontSize); err != nil {
		return fmt.Errorf("loading content font: %w", err)
	}

	for _, content := range slideContent.Content {
		if slideContent.IsCode {
			// Use different color for code
			dc.SetColor(color.RGBA{113, 128, 150, 255}) // Light gray for code
			// Use slightly smaller font for code
			if err := r.loadGoFont(dc, contentFontSize*0.9); err != nil {
				return fmt.Errorf("loading code font: %w", err)
			}
		} else {
			dc.SetColor(color.RGBA{74, 85, 104, 255})
			if err := r.loadGoFont(dc, contentFontSize); err != nil {
				return fmt.Errorf("loading content font: %w", err)
			}
		}

		// Wrap content text
		contentLines := r.wrapText(content, contentWidth, dc)
		for _, line := range contentLines {
			if currentY+contentFontSize > float64(height)-marginY {
				break // Prevent overflow
			}
			dc.DrawString(line, marginX, currentY)
			currentY += contentFontSize * 1.4
		}
		currentY += contentFontSize * 0.3 // Spacing between content blocks
	}

	return nil
}

// wrapText wraps text to fit within the specified width
func (r *ImageRenderer) wrapText(text string, maxWidth float64, dc *gg.Context) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		testLine := currentLine.String()
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		width, _ := dc.MeasureString(testLine)
		if width > maxWidth && currentLine.Len() > 0 {
			// Current line is too long, save it and start new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			// Add word to current line
			if currentLine.Len() > 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}
	}

	// Add the last line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// loadGoFont loads the embedded Go font with the specified size
func (r *ImageRenderer) loadGoFont(dc *gg.Context, fontSize float64) error {
	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return fmt.Errorf("parsing embedded font: %w", err)
	}

	face := truetype.NewFace(font, &truetype.Options{
		Size: fontSize,
	})

	dc.SetFontFace(face)
	return nil
}

// saveAsPNG saves the image as PNG format
func (r *ImageRenderer) saveAsPNG(img image.Image, outputPath string) error {
	// #nosec G304 - outputPath is validated by caller and represents user's intended export path
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating PNG file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}

	return nil
}

// saveAsJPEG saves the image as JPEG format
func (r *ImageRenderer) saveAsJPEG(img image.Image, outputPath string, options *ExportOptions) error {
	// #nosec G304 - outputPath is validated by caller and represents user's intended export path
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating JPEG file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Set JPEG quality based on export quality
	quality := 90 // default
	switch options.Quality {
	case "low":
		quality = 70
	case "high":
		quality = 95
	}

	jpegOptions := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(file, img, jpegOptions); err != nil {
		return fmt.Errorf("encoding JPEG: %w", err)
	}

	return nil
}

// Supports returns true if this renderer supports the given format
func (r *ImageRenderer) Supports(format ExportFormat) bool {
	return format == FormatImages
}

// GetMimeType returns the MIME type for image exports
func (r *ImageRenderer) GetMimeType() string {
	return "image/png"
}

// GetImageDimensions returns the dimensions based on quality setting
func GetImageDimensions(quality string) (width, height int) {
	switch quality {
	case "low":
		return 1280, 720
	case "high":
		return 2560, 1440
	default: // medium
		return 1920, 1080
	}
}

// findSubstring is a simple substring search function (kept for potential future use)
// func findSubstring(s, substr string) int {
//	if len(substr) == 0 {
//		return 0
//	}
//	if len(substr) > len(s) {
//		return -1
//	}
//	for i := 0; i <= len(s)-len(substr); i++ {
//		if s[i:i+len(substr)] == substr {
//			return i
//		}
//	}
//	return -1
// }

// IsBrowserAvailable checks if browser automation is available for image generation
func (r *ImageRenderer) IsBrowserAvailable() bool {
	if r.browserAutomation == nil {
		return false
	}

	ctx := context.Background()
	return r.browserAutomation.IsAvailable(ctx) == nil
}

// GetBrowserInfo returns information about the browser used for image generation
func (r *ImageRenderer) GetBrowserInfo() (string, error) {
	if r.browserAutomation == nil {
		return "", errors.New("browser automation not initialized")
	}

	ctx := context.Background()
	version, err := r.browserAutomation.GetChromeVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("getting browser version: %w", err)
	}

	return fmt.Sprintf("Chrome/Chromium %s at %s", version, r.browserAutomation.executablePath), nil
}

// Note: This renderer uses Chrome/Chromium headless for production-quality image generation.
// When browser automation is not available, it falls back to gg graphics library for real image generation.
