package parser

import (
	"bufio"
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
)

// NotesExtractor handles extraction of speaker notes from markdown content
type NotesExtractor struct {
	notePrefix string
	noteRegex  *regexp.Regexp
}

// NewNotesExtractor creates a new notes extractor
func NewNotesExtractor() *NotesExtractor {
	return &NotesExtractor{
		notePrefix: "Note:",
		noteRegex:  regexp.MustCompile(`^Note:\s*(.*)$`),
	}
}

// ExtractNotes separates speaker notes from main content
func (e *NotesExtractor) ExtractNotes(content string) (mainContent string, notes string) {
	var contentLines, noteLines []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, e.notePrefix) {
			// Extract note content after the prefix
			noteContent := strings.TrimPrefix(trimmed, e.notePrefix)
			noteContent = strings.TrimSpace(noteContent)
			if noteContent != "" {
				noteLines = append(noteLines, noteContent)
			}
		} else {
			contentLines = append(contentLines, line)
		}
	}

	mainContent = strings.Join(contentLines, "\n")
	notes = strings.Join(noteLines, "\n\n")

	return mainContent, notes
}

// ConvertNotesToHTML converts markdown notes to HTML
func (e *NotesExtractor) ConvertNotesToHTML(notes string) string {
	if strings.TrimSpace(notes) == "" {
		return ""
	}

	// Use goldmark to convert markdown to HTML
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(notes), &buf); err != nil {
		// If conversion fails, return escaped text
		return "<p>" + html.EscapeString(notes) + "</p>"
	}

	return buf.String()
}

// ExtractSlideTitle extracts the first heading from slide content
func (e *NotesExtractor) ExtractSlideTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			// Remove # symbols and trim
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return html.EscapeString(title)
			}
		}
	}

	return "Untitled Slide"
}
