package parser

import (
	"strings"
	"testing"
)

func TestNotesExtractor_ExtractNotes(t *testing.T) {
	extractor := NewNotesExtractor()

	tests := []struct {
		name          string
		content       string
		expectedMain  string
		expectedNotes string
	}{
		{
			name: "no notes",
			content: `# Slide Title

Some content here
- Bullet point
- Another point`,
			expectedMain: `# Slide Title

Some content here
- Bullet point
- Another point`,
			expectedNotes: "",
		},
		{
			name: "single note",
			content: `# Slide Title

Some content here

Note: This is a speaker note`,
			expectedMain: `# Slide Title

Some content here
`,
			expectedNotes: "This is a speaker note",
		},
		{
			name: "multiple notes",
			content: `# Slide Title

Some content here

Note: First note
More content

Note: Second note`,
			expectedMain: `# Slide Title

Some content here

More content
`,
			expectedNotes: "First note\n\nSecond note",
		},
		{
			name: "notes with whitespace",
			content: `# Slide Title

Note:   This note has leading spaces
Note:	This note has a tab
Note: Normal note`,
			expectedMain: `# Slide Title
`,
			expectedNotes: "This note has leading spaces\n\nThis note has a tab\n\nNormal note",
		},
		{
			name: "empty note lines",
			content: `# Slide Title

Note:
Note: Valid note
Note:   `,
			expectedMain: `# Slide Title
`,
			expectedNotes: "Valid note",
		},
		{
			name: "mixed content and notes",
			content: `# Introduction

Welcome to the presentation

Note: Remember to speak slowly

## Key Points

- Point 1
- Point 2

Note: Emphasize point 2

That's all!`,
			expectedMain: `# Introduction

Welcome to the presentation


## Key Points

- Point 1
- Point 2


That's all!`,
			expectedNotes: "Remember to speak slowly\n\nEmphasize point 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMain, gotNotes := extractor.ExtractNotes(tt.content)

			if gotMain != tt.expectedMain {
				t.Errorf("ExtractNotes() main content = %q, want %q", gotMain, tt.expectedMain)
			}

			if gotNotes != tt.expectedNotes {
				t.Errorf("ExtractNotes() notes = %q, want %q", gotNotes, tt.expectedNotes)
			}
		})
	}
}

func TestNotesExtractor_ConvertNotesToHTML(t *testing.T) {
	extractor := NewNotesExtractor()

	tests := []struct {
		name     string
		notes    string
		expected string
	}{
		{
			name:     "empty notes",
			notes:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			notes:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "simple text",
			notes:    "This is a simple note",
			expected: "<p>This is a simple note</p>\n",
		},
		{
			name:     "markdown formatting",
			notes:    "This is **bold** and *italic*",
			expected: "<p>This is <strong>bold</strong> and <em>italic</em></p>\n",
		},
		{
			name: "multiple paragraphs",
			notes: `First paragraph

Second paragraph`,
			expected: "<p>First paragraph</p>\n<p>Second paragraph</p>\n",
		},
		{
			name: "list items",
			notes: `Remember to:
- Speak clearly
- Make eye contact
- Use gestures`,
			expected: "<p>Remember to:</p>\n<ul>\n<li>Speak clearly</li>\n<li>Make eye contact</li>\n<li>Use gestures</li>\n</ul>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.ConvertNotesToHTML(tt.notes)
			if got != tt.expected {
				t.Errorf("ConvertNotesToHTML() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNotesExtractor_ExtractSlideTitle(t *testing.T) {
	extractor := NewNotesExtractor()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "no heading",
			content:  "Just some text without a heading",
			expected: "Untitled Slide",
		},
		{
			name: "h1 heading",
			content: `# Introduction

Some content here`,
			expected: "Introduction",
		},
		{
			name: "h2 heading",
			content: `## Key Points

Some content here`,
			expected: "Key Points",
		},
		{
			name: "multiple headings",
			content: `# Main Title

## Subtitle

Some content`,
			expected: "Main Title",
		},
		{
			name: "heading with extra spaces",
			content: `###   Title with Spaces   

Content`,
			expected: "Title with Spaces",
		},
		{
			name: "heading with special characters",
			content: `# Title & Symbols <test>

Content`,
			expected: "Title &amp; Symbols &lt;test&gt;",
		},
		{
			name: "empty heading",
			content: `#

Content`,
			expected: "Untitled Slide",
		},
		{
			name: "heading after content",
			content: `Some content first

# Heading Later`,
			expected: "Heading Later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.ExtractSlideTitle(tt.content)
			if got != tt.expected {
				t.Errorf("ExtractSlideTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNotesExtractor_Integration(t *testing.T) {
	extractor := NewNotesExtractor()

	content := `# Welcome to Our Presentation

This is the introduction slide with some content.

Note: Welcome everyone and introduce yourself

## Agenda

- Topic 1
- Topic 2  
- Topic 3

Note: Explain that we'll cover these three main topics

Remember to engage the audience!`

	// Test extraction
	mainContent, notes := extractor.ExtractNotes(content)

	// Verify main content doesn't contain notes
	if strings.Contains(mainContent, "Note:") {
		t.Error("Main content should not contain Note: lines")
	}

	// Verify notes were extracted
	expectedNotes := "Welcome everyone and introduce yourself\n\nExplain that we'll cover these three main topics"
	if notes != expectedNotes {
		t.Errorf("Extracted notes = %q, want %q", notes, expectedNotes)
	}

	// Test HTML conversion
	html := extractor.ConvertNotesToHTML(notes)
	if !strings.Contains(html, "<p>Welcome everyone") {
		t.Error("HTML should contain converted notes")
	}

	// Test title extraction
	title := extractor.ExtractSlideTitle(mainContent)
	if title != "Welcome to Our Presentation" {
		t.Errorf("Extracted title = %q, want %q", title, "Welcome to Our Presentation")
	}
}
