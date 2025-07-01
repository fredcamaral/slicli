package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlide_Validate(t *testing.T) {
	tests := []struct {
		name    string
		slide   Slide
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid slide",
			slide: Slide{
				Content: "# Hello World",
				Index:   0,
			},
			wantErr: false,
		},
		{
			name: "empty content",
			slide: Slide{
				Content: "",
				Index:   0,
			},
			wantErr: true,
			errMsg:  "slide content cannot be empty",
		},
		{
			name: "whitespace only content",
			slide: Slide{
				Content: "   \n\t  ",
				Index:   0,
			},
			wantErr: true,
			errMsg:  "slide content cannot be empty",
		},
		{
			name: "negative index",
			slide: Slide{
				Content: "Valid content",
				Index:   -1,
			},
			wantErr: true,
			errMsg:  "slide index must be non-negative",
		},
		{
			name: "valid slide with notes",
			slide: Slide{
				Content: "# Title\n\nContent",
				Index:   0,
				Notes:   "Speaker notes here",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.slide.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSlide_ExtractTitle(t *testing.T) {
	tests := []struct {
		name  string
		slide Slide
		want  string
	}{
		{
			name: "h1 at start",
			slide: Slide{
				Content: "# Main Title\n\nSome content",
				Index:   0,
			},
			want: "Main Title",
		},
		{
			name: "h1 with leading whitespace",
			slide: Slide{
				Content: "  # Spaced Title\n\nContent",
				Index:   0,
			},
			want: "Spaced Title",
		},
		{
			name: "h1 after content",
			slide: Slide{
				Content: "Some intro\n\n# Title Here\n\nMore content",
				Index:   1,
			},
			want: "Title Here",
		},
		{
			name: "no h1 heading",
			slide: Slide{
				Content: "## Subtitle\n\nNo main heading",
				Index:   2,
			},
			want: "Slide 3",
		},
		{
			name: "empty content",
			slide: Slide{
				Content: "",
				Index:   4,
			},
			want: "Slide 5",
		},
		{
			name: "multiple h1 headings",
			slide: Slide{
				Content: "# First\n\n# Second\n\nContent",
				Index:   0,
			},
			want: "First", // Should take the first one
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := tt.slide.ExtractTitle()
			assert.Equal(t, tt.want, title)
		})
	}
}

func TestSlide_HasNotes(t *testing.T) {
	tests := []struct {
		name  string
		notes string
		want  bool
	}{
		{
			name:  "has notes",
			notes: "Speaker notes content",
			want:  true,
		},
		{
			name:  "empty notes",
			notes: "",
			want:  false,
		},
		{
			name:  "whitespace only notes",
			notes: "   \n\t  ",
			want:  false,
		},
		{
			name:  "notes with content",
			notes: "  Important points to mention  ",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Slide{
				Content: "# Test",
				Notes:   tt.notes,
			}
			assert.Equal(t, tt.want, s.HasNotes())
		})
	}
}

func TestSlide_ContentWithoutNotes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "content with note lines",
			content: `# Title

This is content

Note: This is a speaker note

More content here`,
			want: `# Title

This is content


More content here`,
		},
		{
			name: "multiple note lines",
			content: `# Slide

Content

Note: First note
Note: Second note

Final content`,
			want: `# Slide

Content


Final content`,
		},
		{
			name: "note with leading spaces",
			content: `# Title

   Note: Indented note

Content`,
			want: `# Title


Content`,
		},
		{
			name: "no notes",
			content: `# Title

Just regular content
No notes here`,
			want: `# Title

Just regular content
No notes here`,
		},
		{
			name: "note in middle of line",
			content: `# Title

This line contains Note: but not at start`,
			want: `# Title

This line contains Note: but not at start`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Slide{Content: tt.content}
			result := s.ContentWithoutNotes()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestSlide_CompleteStruct(t *testing.T) {
	s := &Slide{
		ID:    "slide-123",
		Index: 2,
		Title: "Test Slide",
		Content: `# Test Slide

This is the content

Note: Speaker note here`,
		HTML:  "<h1>Test Slide</h1><p>This is the content</p>",
		Notes: "Speaker note here",
		Metadata: map[string]interface{}{
			"transition": "fade",
			"duration":   5,
		},
	}

	// Test all fields
	assert.Equal(t, "slide-123", s.ID)
	assert.Equal(t, 2, s.Index)
	assert.Equal(t, "Test Slide", s.Title)
	assert.Contains(t, s.Content, "This is the content")
	assert.Contains(t, s.HTML, "<h1>Test Slide</h1>")
	assert.Equal(t, "Speaker note here", s.Notes)
	assert.Equal(t, "fade", s.Metadata["transition"])
	assert.Equal(t, 5, s.Metadata["duration"])
}
