package notes

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// Service implements the NotesService interface
type Service struct {
	notes      map[string]*entities.SpeakerNotes
	mu         sync.RWMutex
	markdownMD goldmark.Markdown
}

// NewService creates a new notes service
func NewService() *Service {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	return &Service{
		notes:      make(map[string]*entities.SpeakerNotes),
		markdownMD: md,
	}
}

// GetNotes retrieves speaker notes for a specific slide
func (s *Service) GetNotes(slideID string) (*entities.SpeakerNotes, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notes, exists := s.notes[slideID]
	if !exists {
		// Return empty notes if not found
		return &entities.SpeakerNotes{
			SlideID: slideID,
			Content: "",
			HTML:    "",
		}, nil
	}

	return notes, nil
}

// SetNotes sets speaker notes for a specific slide
func (s *Service) SetNotes(slideID string, notes *entities.SpeakerNotes) error {
	if notes == nil {
		return errors.New("notes cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure slide ID matches
	notes.SlideID = slideID

	// Convert markdown to HTML
	notes.HTML = s.ConvertNotesToHTML(notes.Content)

	s.notes[slideID] = notes
	return nil
}

// ExtractNotes extracts notes from slide content
// Notes are marked with <!-- NOTES: --> comments
func (s *Service) ExtractNotes(content string) (mainContent string, notesContent string) {
	// Regular expression to match notes sections
	notesRegex := regexp.MustCompile(`(?s)<!--\s*NOTES:\s*-->(.*?)(?:<!--\s*END\s*NOTES\s*-->|$)`)

	// Find all notes sections
	matches := notesRegex.FindAllStringSubmatch(content, -1)

	var notes []string
	for _, match := range matches {
		if len(match) > 1 {
			// Trim whitespace and add to notes
			noteContent := strings.TrimSpace(match[1])
			if noteContent != "" {
				notes = append(notes, noteContent)
			}
		}
	}

	// Remove notes from main content
	mainContent = notesRegex.ReplaceAllString(content, "")

	// Join all notes with double newlines
	notesContent = strings.Join(notes, "\n\n")

	return strings.TrimSpace(mainContent), strings.TrimSpace(notesContent)
}

// ConvertNotesToHTML converts markdown notes to HTML
func (s *Service) ConvertNotesToHTML(notes string) string {
	if strings.TrimSpace(notes) == "" {
		return ""
	}

	var buf strings.Builder
	if err := s.markdownMD.Convert([]byte(notes), &buf); err != nil {
		// If markdown conversion fails, return as plain text wrapped in <p>
		return fmt.Sprintf("<p>%s</p>", strings.ReplaceAll(notes, "\n", "<br>"))
	}

	return buf.String()
}
