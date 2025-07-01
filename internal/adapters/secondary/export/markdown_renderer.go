package export

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// MarkdownRenderer implements export to markdown format
type MarkdownRenderer struct{}

// NewMarkdownRenderer creates a new markdown renderer
func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{}
}

// Render exports the presentation to markdown format
func (r *MarkdownRenderer) Render(ctx context.Context, presentation *entities.Presentation, options *ExportOptions) (*ExportResult, error) {
	var content strings.Builder

	// Write frontmatter
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("title: \"%s\"\n", presentation.Title))
	if presentation.Author != "" {
		content.WriteString(fmt.Sprintf("author: \"%s\"\n", presentation.Author))
	}
	if !presentation.Date.IsZero() {
		content.WriteString(fmt.Sprintf("date: \"%s\"\n", presentation.Date.Format("2006-01-02")))
	}
	if presentation.Theme != "" {
		content.WriteString(fmt.Sprintf("theme: \"%s\"\n", presentation.Theme))
	}
	content.WriteString("exported: \"" + time.Now().Format("2006-01-02 15:04:05") + "\"\n")
	content.WriteString("generator: \"slicli\"\n")
	if options.IncludeMetadata && len(options.Metadata) > 0 {
		for key, value := range options.Metadata {
			content.WriteString(fmt.Sprintf("%s: \"%v\"\n", key, value))
		}
	}
	content.WriteString("---\n\n")

	// Write presentation title
	content.WriteString(fmt.Sprintf("# %s\n\n", presentation.Title))

	// Write metadata if present
	if presentation.Author != "" || !presentation.Date.IsZero() {
		content.WriteString("**Presentation Details:**\n")
		if presentation.Author != "" {
			content.WriteString(fmt.Sprintf("- **Author:** %s\n", presentation.Author))
		}
		if !presentation.Date.IsZero() {
			content.WriteString(fmt.Sprintf("- **Date:** %s\n", presentation.Date.Format("January 2, 2006")))
		}
		content.WriteString(fmt.Sprintf("- **Slides:** %d\n", len(presentation.Slides)))
		content.WriteString("\n---\n\n")
	}

	// Write slides
	for i, slide := range presentation.Slides {
		// Slide header
		content.WriteString(fmt.Sprintf("## Slide %d\n\n", i+1))

		// Convert HTML back to markdown (simplified)
		slideMarkdown := r.htmlToMarkdown(slide.HTML)
		content.WriteString(slideMarkdown)
		content.WriteString("\n\n")

		// Add speaker notes if included and present
		if options.IncludeNotes && slide.Notes != "" {
			content.WriteString("### Speaker Notes\n\n")
			content.WriteString("> " + strings.ReplaceAll(slide.Notes, "\n", "\n> "))
			content.WriteString("\n\n")
		}

		// Add slide separator except for last slide
		if i < len(presentation.Slides)-1 {
			content.WriteString("---\n\n")
		}
	}

	// Write footer
	content.WriteString("\n---\n\n*Exported from slicli on " + time.Now().Format("January 2, 2006 at 3:04 PM") + "*\n")

	// Write to file
	err := os.WriteFile(options.OutputPath, []byte(content.String()), 0600)
	if err != nil {
		return nil, fmt.Errorf("writing markdown file: %w", err)
	}

	// Get file size
	fileSize, _ := GetFileSize(options.OutputPath)

	return &ExportResult{
		Success:    true,
		Format:     string(FormatMarkdown),
		OutputPath: options.OutputPath,
		FileSize:   fileSize,
		PageCount:  len(presentation.Slides),
	}, nil
}

// htmlToMarkdown converts HTML content back to markdown (simplified conversion)
func (r *MarkdownRenderer) htmlToMarkdown(html string) string {
	content := html

	// Replace HTML headers with markdown headers
	content = strings.ReplaceAll(content, "<h1>", "# ")
	content = strings.ReplaceAll(content, "</h1>", "\n")
	content = strings.ReplaceAll(content, "<h2>", "## ")
	content = strings.ReplaceAll(content, "</h2>", "\n")
	content = strings.ReplaceAll(content, "<h3>", "### ")
	content = strings.ReplaceAll(content, "</h3>", "\n")
	content = strings.ReplaceAll(content, "<h4>", "#### ")
	content = strings.ReplaceAll(content, "</h4>", "\n")
	content = strings.ReplaceAll(content, "<h5>", "##### ")
	content = strings.ReplaceAll(content, "</h5>", "\n")
	content = strings.ReplaceAll(content, "<h6>", "###### ")
	content = strings.ReplaceAll(content, "</h6>", "\n")

	// Replace paragraphs
	content = strings.ReplaceAll(content, "<p>", "")
	content = strings.ReplaceAll(content, "</p>", "\n\n")

	// Replace line breaks
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br />", "\n")

	// Replace emphasis
	content = strings.ReplaceAll(content, "<em>", "*")
	content = strings.ReplaceAll(content, "</em>", "*")
	content = strings.ReplaceAll(content, "<i>", "*")
	content = strings.ReplaceAll(content, "</i>", "*")

	// Replace strong
	content = strings.ReplaceAll(content, "<strong>", "**")
	content = strings.ReplaceAll(content, "</strong>", "**")
	content = strings.ReplaceAll(content, "<b>", "**")
	content = strings.ReplaceAll(content, "</b>", "**")

	// Replace code
	content = strings.ReplaceAll(content, "<code>", "`")
	content = strings.ReplaceAll(content, "</code>", "`")

	// Replace preformatted blocks
	content = strings.ReplaceAll(content, "<pre>", "```\n")
	content = strings.ReplaceAll(content, "</pre>", "\n```")

	// Replace lists (simplified)
	content = strings.ReplaceAll(content, "<ul>", "")
	content = strings.ReplaceAll(content, "</ul>", "\n")
	content = strings.ReplaceAll(content, "<ol>", "")
	content = strings.ReplaceAll(content, "</ol>", "\n")
	content = strings.ReplaceAll(content, "<li>", "- ")
	content = strings.ReplaceAll(content, "</li>", "\n")

	// Replace blockquotes
	content = strings.ReplaceAll(content, "<blockquote>", "> ")
	content = strings.ReplaceAll(content, "</blockquote>", "\n")

	// Replace horizontal rules
	content = strings.ReplaceAll(content, "<hr>", "---")
	content = strings.ReplaceAll(content, "<hr/>", "---")
	content = strings.ReplaceAll(content, "<hr />", "---")

	// Handle links (simplified)
	content = r.convertLinks(content)

	// Handle images (simplified)
	content = r.convertImages(content)

	// Clean up extra whitespace
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	content = strings.TrimSpace(content)

	return content
}

// convertLinks converts HTML links to markdown format
func (r *MarkdownRenderer) convertLinks(content string) string {
	// Simple regex replacement would be better, but for now use basic string manipulation
	// This is a simplified implementation
	result := content

	// Look for <a href="url">text</a> patterns and convert to [text](url)
	for {
		start := strings.Index(result, "<a href=\"")
		if start == -1 {
			break
		}

		urlStart := start + 9
		urlEnd := strings.Index(result[urlStart:], "\"")
		if urlEnd == -1 {
			break
		}
		urlEnd += urlStart

		textStart := strings.Index(result[urlEnd:], ">")
		if textStart == -1 {
			break
		}
		textStart += urlEnd + 1

		textEnd := strings.Index(result[textStart:], "</a>")
		if textEnd == -1 {
			break
		}
		textEnd += textStart

		url := result[urlStart:urlEnd]
		text := result[textStart:textEnd]

		markdown := fmt.Sprintf("[%s](%s)", text, url)
		result = result[:start] + markdown + result[textEnd+4:]
	}

	return result
}

// convertImages converts HTML images to markdown format
func (r *MarkdownRenderer) convertImages(content string) string {
	// Simple conversion for <img src="url" alt="text"> to ![text](url)
	result := content

	for {
		start := strings.Index(result, "<img")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		end += start + 1

		imgTag := result[start:end]

		// Extract src
		srcStart := strings.Index(imgTag, "src=\"")
		if srcStart == -1 {
			result = result[:start] + result[end:]
			continue
		}
		srcStart += 5
		srcEnd := strings.Index(imgTag[srcStart:], "\"")
		if srcEnd == -1 {
			result = result[:start] + result[end:]
			continue
		}
		srcEnd += srcStart
		src := imgTag[srcStart:srcEnd]

		// Extract alt
		alt := ""
		altStart := strings.Index(imgTag, "alt=\"")
		if altStart != -1 {
			altStart += 5
			altEnd := strings.Index(imgTag[altStart:], "\"")
			if altEnd != -1 {
				altEnd += altStart
				alt = imgTag[altStart:altEnd]
			}
		}

		markdown := fmt.Sprintf("![%s](%s)", alt, src)
		result = result[:start] + markdown + result[end:]
	}

	return result
}

// Supports returns true if this renderer supports the given format
func (r *MarkdownRenderer) Supports(format ExportFormat) bool {
	return format == FormatMarkdown
}

// GetMimeType returns the MIME type for markdown exports
func (r *MarkdownRenderer) GetMimeType() string {
	return "text/markdown"
}
