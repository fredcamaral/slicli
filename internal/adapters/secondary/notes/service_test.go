package notes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

func TestNewService(t *testing.T) {
	service := NewService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.notes)
	assert.NotNil(t, service.markdownMD)
	assert.Empty(t, service.notes)
}

func TestService_GetNotes(t *testing.T) {
	service := NewService()

	t.Run("get non-existent notes", func(t *testing.T) {
		notes, err := service.GetNotes("slide-1")

		assert.NoError(t, err)
		assert.NotNil(t, notes)
		assert.Equal(t, "slide-1", notes.SlideID)
		assert.Empty(t, notes.Content)
		assert.Empty(t, notes.HTML)
	})

	t.Run("get existing notes", func(t *testing.T) {
		// First set some notes
		existingNotes := &entities.SpeakerNotes{
			SlideID: "slide-2",
			Content: "Test notes content",
			HTML:    "<p>Test notes content</p>",
		}

		err := service.SetNotes("slide-2", existingNotes)
		require.NoError(t, err)

		// Now get them back
		notes, err := service.GetNotes("slide-2")

		assert.NoError(t, err)
		assert.NotNil(t, notes)
		assert.Equal(t, "slide-2", notes.SlideID)
		assert.Equal(t, "Test notes content", notes.Content)
		assert.Contains(t, notes.HTML, "Test notes content")
	})
}

func TestService_SetNotes(t *testing.T) {
	service := NewService()

	t.Run("set valid notes", func(t *testing.T) {
		notes := &entities.SpeakerNotes{
			SlideID: "slide-1",
			Content: "# Test Notes\nThis is **bold** text.",
		}

		err := service.SetNotes("slide-1", notes)

		assert.NoError(t, err)

		// Verify the notes were stored
		storedNotes, err := service.GetNotes("slide-1")
		require.NoError(t, err)
		assert.Equal(t, "slide-1", storedNotes.SlideID)
		assert.Equal(t, "# Test Notes\nThis is **bold** text.", storedNotes.Content)
		assert.Contains(t, storedNotes.HTML, "<h1>")
		assert.Contains(t, storedNotes.HTML, "<strong>bold</strong>")
	})

	t.Run("set notes with different slide ID", func(t *testing.T) {
		notes := &entities.SpeakerNotes{
			SlideID: "wrong-id",
			Content: "Test content",
		}

		err := service.SetNotes("correct-id", notes)

		assert.NoError(t, err)

		// Verify the slide ID was corrected
		storedNotes, err := service.GetNotes("correct-id")
		require.NoError(t, err)
		assert.Equal(t, "correct-id", storedNotes.SlideID)
	})

	t.Run("set nil notes", func(t *testing.T) {
		err := service.SetNotes("slide-1", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notes cannot be nil")
	})

	t.Run("set empty notes", func(t *testing.T) {
		notes := &entities.SpeakerNotes{
			SlideID: "slide-empty",
			Content: "",
		}

		err := service.SetNotes("slide-empty", notes)

		assert.NoError(t, err)

		storedNotes, err := service.GetNotes("slide-empty")
		require.NoError(t, err)
		assert.Empty(t, storedNotes.Content)
		assert.Empty(t, storedNotes.HTML)
	})
}

func TestService_ExtractNotes(t *testing.T) {
	service := NewService()

	t.Run("extract simple notes", func(t *testing.T) {
		content := `# Slide Title

Some slide content.

<!-- NOTES: -->
This is a speaker note.
<!-- END NOTES -->

More slide content.`

		mainContent, notesContent := service.ExtractNotes(content)

		assert.Contains(t, mainContent, "# Slide Title")
		assert.Contains(t, mainContent, "Some slide content")
		assert.Contains(t, mainContent, "More slide content")
		assert.NotContains(t, mainContent, "NOTES:")
		assert.NotContains(t, mainContent, "This is a speaker note")

		assert.Equal(t, "This is a speaker note.", notesContent)
	})

	t.Run("extract notes without end tag", func(t *testing.T) {
		content := `# Slide Title

<!-- NOTES: -->
This note continues to the end of the content.`

		mainContent, notesContent := service.ExtractNotes(content)

		assert.Contains(t, mainContent, "# Slide Title")
		assert.NotContains(t, mainContent, "NOTES:")
		assert.Equal(t, "This note continues to the end of the content.", notesContent)
	})

	t.Run("extract multiple notes sections", func(t *testing.T) {
		content := `# Slide Title

<!-- NOTES: -->
First note section.
<!-- END NOTES -->

Some content.

<!-- NOTES: -->
Second note section.
<!-- END NOTES -->`

		mainContent, notesContent := service.ExtractNotes(content)

		assert.Contains(t, mainContent, "# Slide Title")
		assert.Contains(t, mainContent, "Some content")
		assert.NotContains(t, mainContent, "NOTES:")

		assert.Contains(t, notesContent, "First note section.")
		assert.Contains(t, notesContent, "Second note section.")
		assert.Contains(t, notesContent, "\n\n") // Should be joined with double newlines
	})

	t.Run("extract notes with whitespace variations", func(t *testing.T) {
		content := `<!-- NOTES:    -->
   Note with whitespace   
<!--   END NOTES   -->`

		_, notesContent := service.ExtractNotes(content)

		assert.Equal(t, "Note with whitespace", notesContent)
	})

	t.Run("no notes in content", func(t *testing.T) {
		content := `# Regular slide content
With no notes at all.`

		mainContent, notesContent := service.ExtractNotes(content)

		assert.Equal(t, content, mainContent)
		assert.Empty(t, notesContent)
	})

	t.Run("empty notes section", func(t *testing.T) {
		content := `# Slide Title

<!-- NOTES: -->
<!-- END NOTES -->

Content after.`

		mainContent, notesContent := service.ExtractNotes(content)

		assert.Contains(t, mainContent, "# Slide Title")
		assert.Contains(t, mainContent, "Content after")
		assert.Empty(t, notesContent)
	})

	t.Run("notes with markdown content", func(t *testing.T) {
		content := `# Slide Title

<!-- NOTES: -->
## Important Points
- First point
- **Bold point**
- *Italic point*

[Link example](https://example.com)
<!-- END NOTES -->`

		_, notesContent := service.ExtractNotes(content)

		assert.Contains(t, notesContent, "## Important Points")
		assert.Contains(t, notesContent, "- First point")
		assert.Contains(t, notesContent, "**Bold point**")
		assert.Contains(t, notesContent, "*Italic point*")
		assert.Contains(t, notesContent, "[Link example](https://example.com)")
	})
}

func TestService_ConvertNotesToHTML(t *testing.T) {
	service := NewService()

	t.Run("convert markdown to HTML", func(t *testing.T) {
		markdown := `# Heading 1
## Heading 2

This is a **bold** word and this is *italic*.

- List item 1
- List item 2

[Link](https://example.com)`

		html := service.ConvertNotesToHTML(markdown)

		assert.Contains(t, html, "<h1>Heading 1</h1>")
		assert.Contains(t, html, "<h2>Heading 2</h2>")
		assert.Contains(t, html, "<strong>bold</strong>")
		assert.Contains(t, html, "<em>italic</em>")
		assert.Contains(t, html, "<ul>")
		assert.Contains(t, html, "<li>List item 1</li>")
		assert.Contains(t, html, "<li>List item 2</li>")
		assert.Contains(t, html, `<a href="https://example.com">Link</a>`)
	})

	t.Run("convert plain text", func(t *testing.T) {
		plainText := "Just plain text with no markdown."

		html := service.ConvertNotesToHTML(plainText)

		assert.Contains(t, html, "<p>Just plain text with no markdown.</p>")
	})

	t.Run("convert empty string", func(t *testing.T) {
		html := service.ConvertNotesToHTML("")

		assert.Empty(t, html)
	})

	t.Run("convert whitespace only", func(t *testing.T) {
		html := service.ConvertNotesToHTML("   \n\t   ")

		assert.Empty(t, html)
	})

	t.Run("convert with line breaks", func(t *testing.T) {
		text := "Line 1\nLine 2\nLine 3"

		html := service.ConvertNotesToHTML(text)

		// Goldmark converts single newlines to spaces within paragraphs
		assert.Contains(t, html, "<p>")
		assert.Contains(t, html, "Line 1")
		assert.Contains(t, html, "Line 2")
		assert.Contains(t, html, "Line 3")
	})

	t.Run("convert code blocks", func(t *testing.T) {
		markdown := "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"

		html := service.ConvertNotesToHTML(markdown)

		assert.Contains(t, html, "<pre>")
		assert.Contains(t, html, "<code")
		assert.Contains(t, html, "func main()")
	})

	t.Run("convert tables", func(t *testing.T) {
		markdown := `| Column 1 | Column 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |`

		html := service.ConvertNotesToHTML(markdown)

		assert.Contains(t, html, "<table>")
		assert.Contains(t, html, "<thead>")
		assert.Contains(t, html, "<tbody>")
		assert.Contains(t, html, "<th>Column 1</th>")
		assert.Contains(t, html, "<td>Cell 1</td>")
	})
}

func TestService_ConcurrentAccess(t *testing.T) {
	service := NewService()

	// Test concurrent access to service methods
	done := make(chan bool, 3)

	// Goroutine 1: Set notes
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			notes := &entities.SpeakerNotes{
				SlideID: "concurrent-slide",
				Content: "Concurrent content",
			}
			_ = service.SetNotes("concurrent-slide", notes)
		}
	}()

	// Goroutine 2: Get notes
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_, err := service.GetNotes("concurrent-slide")
			assert.NoError(t, err)
		}
	}()

	// Goroutine 3: Extract notes
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			content := `<!-- NOTES: -->Test note<!-- END NOTES -->`
			_, _ = service.ExtractNotes(content)
		}
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Test passes if no race conditions or panics occurred
}

func TestService_EdgeCases(t *testing.T) {
	service := NewService()

	t.Run("very long slide ID", func(t *testing.T) {
		longID := strings.Repeat("a", 1000)
		notes := &entities.SpeakerNotes{
			SlideID: "short",
			Content: "Test content",
		}

		err := service.SetNotes(longID, notes)
		assert.NoError(t, err)

		retrievedNotes, err := service.GetNotes(longID)
		assert.NoError(t, err)
		assert.Equal(t, longID, retrievedNotes.SlideID)
	})

	t.Run("very long notes content", func(t *testing.T) {
		longContent := strings.Repeat("This is a very long note. ", 1000)
		notes := &entities.SpeakerNotes{
			SlideID: "slide-long-content",
			Content: longContent,
		}

		err := service.SetNotes("slide-long-content", notes)
		assert.NoError(t, err)

		retrievedNotes, err := service.GetNotes("slide-long-content")
		assert.NoError(t, err)
		assert.Equal(t, longContent, retrievedNotes.Content)
		assert.NotEmpty(t, retrievedNotes.HTML)
	})

	t.Run("special characters in slide ID", func(t *testing.T) {
		specialID := "slide-with-@#$%^&*()_+-=[]{}|;':\",./<>?"
		notes := &entities.SpeakerNotes{
			SlideID: "normal",
			Content: "Test content",
		}

		err := service.SetNotes(specialID, notes)
		assert.NoError(t, err)

		retrievedNotes, err := service.GetNotes(specialID)
		assert.NoError(t, err)
		assert.Equal(t, specialID, retrievedNotes.SlideID)
	})

	t.Run("unicode content", func(t *testing.T) {
		unicodeContent := "📝 Notes with emojis 🎯 and unicode: 中文 العربية 🌟"
		notes := &entities.SpeakerNotes{
			SlideID: "unicode-slide",
			Content: unicodeContent,
		}

		err := service.SetNotes("unicode-slide", notes)
		assert.NoError(t, err)

		retrievedNotes, err := service.GetNotes("unicode-slide")
		assert.NoError(t, err)
		assert.Equal(t, unicodeContent, retrievedNotes.Content)
		assert.Contains(t, retrievedNotes.HTML, "📝")
		assert.Contains(t, retrievedNotes.HTML, "中文")
	})
}

func TestService_IntegrationWorkflow(t *testing.T) {
	service := NewService()

	// Simulate a complete workflow of extracting and setting notes
	slideContent := `# Presentation Title

This is the main slide content.

<!-- NOTES: -->
## Speaker Notes
- Remember to mention the key points
- **Emphasize** the important parts
- Time limit: 5 minutes

### Next Steps
1. Show demo
2. Answer questions
<!-- END NOTES -->

More slide content here.`

	// Step 1: Extract notes from slide content
	mainContent, notesContent := service.ExtractNotes(slideContent)

	assert.Contains(t, mainContent, "# Presentation Title")
	assert.Contains(t, mainContent, "This is the main slide content")
	assert.Contains(t, mainContent, "More slide content here")
	assert.NotContains(t, mainContent, "Speaker Notes")

	assert.Contains(t, notesContent, "## Speaker Notes")
	assert.Contains(t, notesContent, "Remember to mention")
	assert.Contains(t, notesContent, "**Emphasize**")
	assert.Contains(t, notesContent, "### Next Steps")

	// Step 2: Create speaker notes entity
	speakerNotes := &entities.SpeakerNotes{
		SlideID: "slide-1",
		Content: notesContent,
	}

	// Step 3: Set the notes in the service
	err := service.SetNotes("slide-1", speakerNotes)
	assert.NoError(t, err)

	// Step 4: Retrieve and verify the notes
	retrievedNotes, err := service.GetNotes("slide-1")
	assert.NoError(t, err)

	assert.Equal(t, "slide-1", retrievedNotes.SlideID)
	assert.Equal(t, notesContent, retrievedNotes.Content)

	// Verify HTML conversion
	assert.Contains(t, retrievedNotes.HTML, "<h2>Speaker Notes</h2>")
	assert.Contains(t, retrievedNotes.HTML, "<strong>Emphasize</strong>")
	assert.Contains(t, retrievedNotes.HTML, "<h3>Next Steps</h3>")
	assert.Contains(t, retrievedNotes.HTML, "<ol>")
	assert.Contains(t, retrievedNotes.HTML, "<li>Show demo</li>")

	// Step 5: Verify we can handle multiple slides
	err = service.SetNotes("slide-2", &entities.SpeakerNotes{
		SlideID: "slide-2",
		Content: "Different notes for slide 2",
	})
	assert.NoError(t, err)

	// Both slides should have their notes preserved
	notes1, _ := service.GetNotes("slide-1")
	notes2, _ := service.GetNotes("slide-2")

	assert.Contains(t, notes1.Content, "Speaker Notes")
	assert.Contains(t, notes2.Content, "Different notes for slide 2")
	assert.NotEqual(t, notes1.Content, notes2.Content)
}
