package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// ThemeService manages presentation themes
type ThemeService struct {
	loader    ports.ThemeLoader
	cache     ports.ThemeCache
	processor ports.AssetProcessor
}

// NewThemeService creates a new theme service
func NewThemeService(loader ports.ThemeLoader, cache ports.ThemeCache, processor ports.AssetProcessor) *ThemeService {
	return &ThemeService{
		loader:    loader,
		cache:     cache,
		processor: processor,
	}
}

// GetTheme retrieves a theme by name
func (s *ThemeService) GetTheme(ctx context.Context, name string) (*entities.ThemeEngine, error) {
	// Check cache first
	if theme, found := s.cache.Get(name); found {
		return theme, nil
	}

	// Load from filesystem
	theme, err := s.loader.Load(ctx, name)
	if err != nil {
		// Try to fall back to default theme
		if name != "default" {
			defaultTheme, defaultErr := s.loader.Load(ctx, "default")
			if defaultErr == nil {
				s.cache.Set("default", defaultTheme)
				return defaultTheme, nil
			}
		}
		return nil, fmt.Errorf("loading theme '%s': %w", name, err)
	}

	// Process theme assets
	if err := s.processThemeAssets(theme); err != nil {
		return nil, fmt.Errorf("processing theme assets: %w", err)
	}

	// Cache the loaded theme
	s.cache.Set(name, theme)

	return theme, nil
}

// ListThemes lists all available themes
func (s *ThemeService) ListThemes(ctx context.Context) ([]entities.ThemeInfo, error) {
	return s.loader.List(ctx)
}

// RenderPresentation renders a complete presentation with a theme
func (s *ThemeService) RenderPresentation(theme *entities.ThemeEngine, presentation *entities.Presentation) ([]byte, error) {
	if theme == nil {
		return nil, errors.New("theme is required")
	}
	if presentation == nil {
		return nil, errors.New("presentation is required")
	}

	// Get the presentation template
	tmpl, ok := theme.Templates["presentation"]
	if !ok {
		return nil, errors.New("presentation template not found in theme")
	}

	// Prepare template data
	data := struct {
		Presentation *entities.Presentation
		Theme        string
		ThemeConfig  entities.ThemeEngineConfig
		SlideCount   int
	}{
		Presentation: presentation,
		Theme:        theme.Name,
		ThemeConfig:  theme.Config,
		SlideCount:   len(presentation.Slides),
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing presentation template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderSlide renders a single slide
func (s *ThemeService) RenderSlide(theme *entities.ThemeEngine, slide *entities.Slide, slideNumber int, totalSlides int) ([]byte, error) {
	if theme == nil {
		return nil, errors.New("theme is required")
	}
	if slide == nil {
		return nil, errors.New("slide is required")
	}

	// Get the slide template
	tmpl, ok := theme.Templates["slide"]
	if !ok {
		return nil, errors.New("slide template not found in theme")
	}

	// Prepare template data
	data := struct {
		Slide         *entities.Slide
		SlideNumber   int
		TotalSlides   int
		Theme         string
		ThemeConfig   entities.ThemeEngineConfig
		IsFirstSlide  bool
		IsLastSlide   bool
		NextSlide     int
		PreviousSlide int
	}{
		Slide:         slide,
		SlideNumber:   slideNumber,
		TotalSlides:   totalSlides,
		Theme:         theme.Name,
		ThemeConfig:   theme.Config,
		IsFirstSlide:  slideNumber == 1,
		IsLastSlide:   slideNumber == totalSlides,
		NextSlide:     slideNumber + 1,
		PreviousSlide: slideNumber - 1,
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing slide template: %w", err)
	}

	return buf.Bytes(), nil
}

// ServeAsset serves a theme asset
func (s *ThemeService) ServeAsset(theme *entities.ThemeEngine, path string) (*entities.ThemeAsset, error) {
	if theme == nil {
		return nil, errors.New("theme is required")
	}

	asset, ok := theme.Assets[path]
	if !ok {
		return nil, fmt.Errorf("asset not found: %s", path)
	}

	return asset, nil
}

// ReloadTheme reloads a theme (for development)
func (s *ThemeService) ReloadTheme(ctx context.Context, name string) error {
	// Remove from cache
	s.cache.Remove(name)

	// Reload the theme
	theme, err := s.loader.Reload(ctx, name)
	if err != nil {
		return fmt.Errorf("reloading theme '%s': %w", name, err)
	}

	// Process assets
	if err := s.processThemeAssets(theme); err != nil {
		return fmt.Errorf("processing theme assets: %w", err)
	}

	// Cache the reloaded theme
	s.cache.Set(name, theme)

	return nil
}

// processThemeAssets processes theme assets (CSS variables, etc.)
func (s *ThemeService) processThemeAssets(theme *entities.ThemeEngine) error {
	// Process CSS files with variable substitution
	for path, asset := range theme.Assets {
		switch asset.ContentType {
		case "text/css":
			processed, err := s.processor.ProcessCSS(asset.Content, theme.Config.Variables)
			if err != nil {
				return fmt.Errorf("processing CSS %s: %w", path, err)
			}
			asset.Content = processed
			// Recompute hash after processing
			asset.ComputeHash()
		case "application/javascript":
			processed, err := s.processor.ProcessJS(asset.Content, theme.Config.Variables)
			if err != nil {
				return fmt.Errorf("processing JS %s: %w", path, err)
			}
			asset.Content = processed
			// Recompute hash after processing
			asset.ComputeHash()
		}
	}

	return nil
}

// RenderNotes renders speaker notes (if template exists)
func (s *ThemeService) RenderNotes(theme *entities.ThemeEngine, slide *entities.Slide, slideNumber int) ([]byte, error) {
	if theme == nil {
		return nil, errors.New("theme is required")
	}

	// Get the notes template (optional)
	tmpl, ok := theme.Templates["notes"]
	if !ok {
		// Notes template is optional, return empty if not found
		return []byte{}, nil
	}

	// Prepare template data
	data := struct {
		Slide       *entities.Slide
		SlideNumber int
		Notes       string
		Theme       string
	}{
		Slide:       slide,
		SlideNumber: slideNumber,
		Notes:       slide.Notes,
		Theme:       theme.Name,
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing notes template: %w", err)
	}

	return buf.Bytes(), nil
}

// GetDefaultTheme returns the default theme name
func (s *ThemeService) GetDefaultTheme() string {
	return "default"
}

// ValidateTheme validates a theme's structure and requirements
func (s *ThemeService) ValidateTheme(ctx context.Context, name string) error {
	theme, err := s.loader.Load(ctx, name)
	if err != nil {
		return fmt.Errorf("loading theme for validation: %w", err)
	}

	return theme.Validate()
}

// Ensure ThemeService implements ports.ThemeService
var _ ports.ThemeService = (*ThemeService)(nil)
