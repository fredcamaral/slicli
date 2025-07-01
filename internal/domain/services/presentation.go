package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// PresentationService implements the business logic for presentations
type PresentationService struct {
	repo      ports.PresentationRepository
	themeRepo ports.ThemeRepository
	parser    ports.PresentationParser
	renderer  ports.SlideRenderer
}

// NewPresentationService creates a new presentation service instance
func NewPresentationService(
	repo ports.PresentationRepository,
	themeRepo ports.ThemeRepository,
	parser ports.PresentationParser,
	renderer ports.SlideRenderer,
) *PresentationService {
	return &PresentationService{
		repo:      repo,
		themeRepo: themeRepo,
		parser:    parser,
		renderer:  renderer,
	}
}

// LoadPresentation loads a presentation from a file path
func (s *PresentationService) LoadPresentation(ctx context.Context, path string) (*entities.Presentation, error) {
	if path == "" {
		return nil, errors.New("presentation path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("presentation file not found: %s", path)
		}
		return nil, fmt.Errorf("checking presentation file: %w", err)
	}

	// Load presentation through repository
	presentation, err := s.repo.Load(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("loading presentation: %w", err)
	}

	// Validate the loaded presentation
	if err := presentation.Validate(); err != nil {
		return nil, fmt.Errorf("invalid presentation: %w", err)
	}

	// Set slide titles
	for i := range presentation.Slides {
		presentation.Slides[i].Title = presentation.Slides[i].ExtractTitle()
	}

	return presentation, nil
}

// ParsePresentation parses markdown content into a presentation
func (s *PresentationService) ParsePresentation(ctx context.Context, content []byte) (*entities.Presentation, error) {
	if len(content) == 0 {
		return nil, errors.New("presentation content cannot be empty")
	}

	presentation, err := s.parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parsing presentation: %w", err)
	}

	// Validate the parsed presentation
	if err := presentation.Validate(); err != nil {
		return nil, fmt.Errorf("invalid presentation: %w", err)
	}

	// Set slide titles and indices
	for i := range presentation.Slides {
		presentation.Slides[i].Index = i
		presentation.Slides[i].Title = presentation.Slides[i].ExtractTitle()
	}

	return presentation, nil
}

// RenderSlides renders all slides in a presentation
func (s *PresentationService) RenderSlides(ctx context.Context, presentation *entities.Presentation) ([]ports.RenderedSlide, error) {
	if presentation == nil {
		return nil, errors.New("presentation cannot be nil")
	}

	rendered := make([]ports.RenderedSlide, 0, len(presentation.Slides))

	for i := range presentation.Slides {
		slide := &presentation.Slides[i]

		renderedSlide, err := s.renderer.RenderSlide(slide)
		if err != nil {
			return nil, fmt.Errorf("rendering slide %d: %w", i+1, err)
		}

		rendered = append(rendered, *renderedSlide)
	}

	return rendered, nil
}

// WatchPresentation watches a presentation file for changes
func (s *PresentationService) WatchPresentation(ctx context.Context, path string) (<-chan ports.FileChangeEvent, error) {
	if path == "" {
		return nil, errors.New("presentation path cannot be empty")
	}

	repoEvents, err := s.repo.Watch(ctx, path)
	if err != nil {
		return nil, err
	}

	// Convert repository events to file change events
	fileEvents := make(chan ports.FileChangeEvent)
	go func() {
		defer close(fileEvents)
		for repoEvent := range repoEvents {
			// Map repository event to file change event
			var changeType ports.ChangeType
			switch repoEvent.Operation {
			case "create":
				changeType = ports.Created
			case "update":
				changeType = ports.Modified
			case "delete":
				changeType = ports.Deleted
			default:
				changeType = ports.Modified
			}

			fileEvent := ports.FileChangeEvent{
				Path:      repoEvent.Path,
				Type:      changeType,
				Timestamp: time.Now(),
			}

			select {
			case fileEvents <- fileEvent:
			case <-ctx.Done():
				return
			}
		}
	}()

	return fileEvents, nil
}

// ApplyTheme applies a theme to a presentation
func (s *PresentationService) ApplyTheme(ctx context.Context, presentation *entities.Presentation, themeName string) error {
	if presentation == nil {
		return errors.New("presentation cannot be nil")
	}

	if themeName == "" {
		themeName = "default"
	}

	// Get the theme
	theme, err := s.themeRepo.Get(ctx, themeName)
	if err != nil {
		return fmt.Errorf("getting theme %s: %w", themeName, err)
	}

	// Validate theme
	if err := theme.Validate(); err != nil {
		return fmt.Errorf("invalid theme %s: %w", themeName, err)
	}

	// Apply theme to presentation
	presentation.Theme = theme.Name

	return nil
}

// LoadPresentationFromReader loads a presentation from an io.Reader
func (s *PresentationService) LoadPresentationFromReader(ctx context.Context, reader io.Reader) (*entities.Presentation, error) {
	if reader == nil {
		return nil, errors.New("reader cannot be nil")
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading content: %w", err)
	}

	return s.ParsePresentation(ctx, content)
}
