package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/jung-kurt/gofpdf/v2"
	"golang.org/x/net/html"
)

// PDFRenderer implements export to PDF using browser automation
type PDFRenderer struct {
	htmlRenderer      *HTMLRenderer
	browserAutomation *BrowserAutomation
}

// NewPDFRenderer creates a new PDF renderer
func NewPDFRenderer() *PDFRenderer {
	// Initialize with default browser config
	browserConfig := BrowserConfig{}
	browserAutomation, _ := NewBrowserAutomation(browserConfig)

	return &PDFRenderer{
		htmlRenderer:      NewHTMLRenderer(),
		browserAutomation: browserAutomation,
	}
}

// NewPDFRendererWithBrowser creates a new PDF renderer with custom browser config
func NewPDFRendererWithBrowser(browserConfig BrowserConfig) (*PDFRenderer, error) {
	browserAutomation, err := NewBrowserAutomation(browserConfig)
	if err != nil {
		return nil, fmt.Errorf("initializing browser automation: %w", err)
	}

	return &PDFRenderer{
		htmlRenderer:      NewHTMLRenderer(),
		browserAutomation: browserAutomation,
	}, nil
}

// Render exports the presentation to PDF
func (r *PDFRenderer) Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	// Create temporary HTML file
	tmpDir := "/tmp"
	if dir := options.OutputPath; dir != "" {
		if i := len(dir) - 1; i >= 0 && dir[i] == '/' {
			tmpDir = dir[:i]
		} else {
			for i := len(dir) - 1; i >= 0; i-- {
				if dir[i] == '/' {
					tmpDir = dir[:i]
					break
				}
			}
		}
	}
	tmpFile, err := os.CreateTemp(tmpDir, "slicli-pdf-*.html")
	if err != nil {
		return nil, fmt.Errorf("creating temporary HTML file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temporary file: %w", err)
	}

	// Prepare HTML export options
	htmlOptions := &ExportOptions{
		Format:          FormatHTML,
		OutputPath:      tmpFile.Name(),
		Theme:           options.Theme,
		IncludeNotes:    options.IncludeNotes,
		IncludeMetadata: options.IncludeMetadata,
		Metadata:        options.Metadata,
	}

	// Generate HTML first
	_, err = r.htmlRenderer.Render(ctx, presentation, htmlOptions)
	if err != nil {
		return nil, fmt.Errorf("generating HTML for PDF conversion: %w", err)
	}

	// Convert HTML to PDF using browser automation or external tool
	err = r.convertHTMLToPDF(tmpFile.Name(), options.OutputPath, options)
	if err != nil {
		return nil, fmt.Errorf("converting HTML to PDF: %w", err)
	}

	// Get file size
	fileSize, _ := GetFileSize(options.OutputPath)

	return &ExportResult{
		Success:    true,
		Format:     string(FormatPDF),
		OutputPath: options.OutputPath,
		FileSize:   fileSize,
		PageCount:  len(presentation.Slides),
	}, nil
}

// convertHTMLToPDF converts an HTML file to PDF using browser automation
func (r *PDFRenderer) convertHTMLToPDF(htmlPath, outputPath string, options *ExportOptions) error {
	// Check if browser automation is available
	if r.browserAutomation == nil {
		return r.fallbackPDFGeneration(htmlPath, outputPath, options)
	}

	ctx := context.Background()
	if err := r.browserAutomation.IsAvailable(ctx); err != nil {
		// Fallback to simple PDF generation if browser is not available
		return r.fallbackPDFGeneration(htmlPath, outputPath, options)
	}

	// Convert export options to PDF options
	pdfOptions := &PDFOptions{
		PageSize:     options.PageSize,
		Landscape:    options.Orientation == "landscape",
		PrintHeaders: false,
		PrintFooters: false,
	}

	// Use browser automation for PDF generation
	err := r.browserAutomation.ConvertHTMLToPDF(ctx, htmlPath, outputPath, pdfOptions)
	if err != nil {
		// Fallback to simple PDF generation if browser automation fails
		return r.fallbackPDFGeneration(htmlPath, outputPath, options)
	}

	return nil
}

// fallbackPDFGeneration creates a proper PDF when browser automation is not available
func (r *PDFRenderer) fallbackPDFGeneration(htmlPath, outputPath string, options *ExportOptions) error {
	return r.generateProperPDF(htmlPath, outputPath, options)
}

// generateProperPDF creates a proper PDF from HTML content using gofpdf
func (r *PDFRenderer) generateProperPDF(htmlPath, outputPath string, options *ExportOptions) error {
	// Validate file path to prevent directory traversal
	if err := validateHTMLPath(htmlPath); err != nil {
		return fmt.Errorf("invalid HTML file path: %w", err)
	}

	// Read HTML content
	htmlContent, err := os.ReadFile(filepath.Clean(htmlPath)) // #nosec G304 - path validated above
	if err != nil {
		return fmt.Errorf("reading HTML file: %w", err)
	}

	// Parse HTML to extract content
	slides, err := r.parseHTMLToSlides(string(htmlContent))
	if err != nil {
		return fmt.Errorf("parsing HTML content: %w", err)
	}

	// Create PDF using gofpdf
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Set margins
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)

	// Process each slide
	for i, slide := range slides {
		if i > 0 || pdf.PageCount() == 0 {
			pdf.AddPage()
		}

		// Set font for content
		pdf.SetFont("Helvetica", "", 12)

		// Add slide content
		if err := r.addSlideContentToPDF(pdf, slide); err != nil {
			return fmt.Errorf("adding slide %d to PDF: %w", i+1, err)
		}
	}

	// Save PDF to output path
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("saving PDF to %s: %w", outputPath, err)
	}

	return nil
}

// SlideContent represents the content of a single slide
type SlideContent struct {
	Title    string
	Content  []string
	IsCode   bool
	Language string
}

// parseHTMLToSlides extracts slide content from HTML
func (r *PDFRenderer) parseHTMLToSlides(htmlContent string) ([]SlideContent, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	var slides []SlideContent
	var currentSlide SlideContent

	// Recursively walk the HTML tree to extract content
	var walkNode func(*html.Node)
	walkNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "div":
				// Check if this is a slide div
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, "slide") {
						// Save previous slide if it has content
						if currentSlide.Title != "" || len(currentSlide.Content) > 0 {
							slides = append(slides, currentSlide)
						}
						// Start new slide
						currentSlide = SlideContent{}
						break
					}
				}
			case "h1", "h2", "h3":
				title := r.extractTextContent(n)
				if title != "" {
					currentSlide.Title = title
				}
			case "p", "li":
				content := r.extractTextContent(n)
				if content != "" {
					currentSlide.Content = append(currentSlide.Content, content)
				}
			case "pre", "code":
				code := r.extractTextContent(n)
				if code != "" {
					currentSlide.Content = append(currentSlide.Content, code)
					currentSlide.IsCode = true
				}
			}
		}

		// Process children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkNode(child)
		}
	}

	walkNode(doc)

	// Add the last slide if it has content
	if currentSlide.Title != "" || len(currentSlide.Content) > 0 {
		slides = append(slides, currentSlide)
	}

	// If no slides were found, create a default slide
	if len(slides) == 0 {
		slides = append(slides, SlideContent{
			Title:   "Generated PDF",
			Content: []string{"This PDF was generated from HTML content by SLICLI."},
		})
	}

	return slides, nil
}

// extractTextContent recursively extracts text content from HTML node
func (r *PDFRenderer) extractTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}

	var text strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		childText := r.extractTextContent(child)
		if childText != "" {
			if text.Len() > 0 {
				text.WriteString(" ")
			}
			text.WriteString(childText)
		}
	}

	return strings.TrimSpace(text.String())
}

// addSlideContentToPDF adds a slide's content to the PDF
func (r *PDFRenderer) addSlideContentToPDF(pdf *gofpdf.Fpdf, slide SlideContent) error {
	// Add title
	if slide.Title != "" {
		pdf.SetFont("Helvetica", "B", 16)
		pdf.Cell(0, 10, slide.Title)
		pdf.Ln(15)
	}

	// Add content
	pdf.SetFont("Helvetica", "", 12)

	for _, content := range slide.Content {
		// Clean up content for PDF
		cleanContent := r.cleanTextForPDF(content)

		if slide.IsCode {
			// Use monospace style for code
			pdf.SetFont("Courier", "", 10)
			// Split long lines
			lines := r.splitLongLines(cleanContent, 80)
			for _, line := range lines {
				pdf.Cell(0, 6, line)
				pdf.Ln(6)
			}
			pdf.SetFont("Helvetica", "", 12)
		} else {
			// Regular text content
			// Split long lines for better formatting
			lines := r.splitLongLines(cleanContent, 90)
			for _, line := range lines {
				pdf.Cell(0, 8, line)
				pdf.Ln(8)
			}
		}
		pdf.Ln(4) // Extra spacing between content blocks
	}

	return nil
}

// cleanTextForPDF removes problematic characters for PDF generation
func (r *PDFRenderer) cleanTextForPDF(text string) string {
	// Remove HTML entities and normalize text
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")

	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// splitLongLines splits text into lines that fit within the specified character limit
func (r *PDFRenderer) splitLongLines(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)

	var currentLine strings.Builder
	for _, word := range words {
		// If adding this word would exceed the limit, start a new line
		if currentLine.Len() > 0 && currentLine.Len()+len(word)+1 > maxChars {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}

		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	// Add the last line if it has content
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// Supports returns true if this renderer supports the given format
func (r *PDFRenderer) Supports(format ExportFormat) bool {
	return format == FormatPDF
}

// GetMimeType returns the MIME type for PDF exports
func (r *PDFRenderer) GetMimeType() string {
	return "application/pdf"
}

// validateHTMLPath validates an HTML file path to prevent directory traversal attacks
func validateHTMLPath(path string) error {
	return validateFilePath(path)
}

// IsBrowserAvailable checks if browser automation is available for PDF generation
func (r *PDFRenderer) IsBrowserAvailable() bool {
	if r.browserAutomation == nil {
		return false
	}

	ctx := context.Background()
	return r.browserAutomation.IsAvailable(ctx) == nil
}

// GetBrowserInfo returns information about the browser used for PDF generation
func (r *PDFRenderer) GetBrowserInfo() (string, error) {
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

// Note: This is a simplified PDF implementation for demonstration.
// For production use, you should integrate with proper PDF generation libraries like:
// - Chrome/Chromium headless for HTML-to-PDF conversion
// - wkhtmltopdf
// - Go PDF libraries like gofpdf, unidoc, etc.
// - Browser automation tools like Playwright or Puppeteer
