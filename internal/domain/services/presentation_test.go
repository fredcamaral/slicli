package services

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// Mock implementations
type MockPresentationRepository struct {
	mock.Mock
}

func (m *MockPresentationRepository) Load(ctx context.Context, path string) (*entities.Presentation, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Presentation), args.Error(1)
}

func (m *MockPresentationRepository) Watch(ctx context.Context, path string) (<-chan ports.RepositoryChangeEvent, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(<-chan ports.RepositoryChangeEvent), args.Error(1)
}

type MockThemeRepository struct {
	mock.Mock
}

func (m *MockThemeRepository) Get(ctx context.Context, name string) (*entities.Theme, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Theme), args.Error(1)
}

func (m *MockThemeRepository) List(ctx context.Context) ([]*entities.Theme, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.Theme), args.Error(1)
}

func (m *MockThemeRepository) Load(ctx context.Context, path string) (*entities.Theme, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Theme), args.Error(1)
}

type MockPresentationParser struct {
	mock.Mock
}

func (m *MockPresentationParser) Parse(content []byte) (*entities.Presentation, error) {
	args := m.Called(content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Presentation), args.Error(1)
}

type MockSlideRenderer struct {
	mock.Mock
}

func (m *MockSlideRenderer) RenderSlide(slide *entities.Slide) (*ports.RenderedSlide, error) {
	args := m.Called(slide)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.RenderedSlide), args.Error(1)
}

// Tests
func TestPresentationService_LoadPresentation(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "test*.md")
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(tmpFile.Name())
		}()
		require.NoError(t, tmpFile.Close())

		// Setup mocks
		repo := new(MockPresentationRepository)
		themeRepo := new(MockThemeRepository)
		parser := new(MockPresentationParser)
		renderer := new(MockSlideRenderer)

		presentation := &entities.Presentation{
			Title: "Test Presentation",
			Theme: "default",
			Slides: []entities.Slide{
				{Content: "# Slide 1", Index: 0},
				{Content: "# Slide 2", Index: 1},
			},
		}

		repo.On("Load", ctx, tmpFile.Name()).Return(presentation, nil)

		service := NewPresentationService(repo, themeRepo, parser, renderer)

		// Test
		result, err := service.LoadPresentation(ctx, tmpFile.Name())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "Test Presentation", result.Title)
		assert.Len(t, result.Slides, 2)
		assert.Equal(t, "Slide 1", result.Slides[0].Title)
		assert.Equal(t, "Slide 2", result.Slides[1].Title)

		repo.AssertExpectations(t)
	})

	t.Run("empty path", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.LoadPresentation(ctx, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "presentation path cannot be empty")
	})

	t.Run("file not found", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.LoadPresentation(ctx, "/nonexistent/file.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "presentation file not found")
	})

	t.Run("repository error", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "test*.md")
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(tmpFile.Name())
		}()
		require.NoError(t, tmpFile.Close())

		repo := new(MockPresentationRepository)
		repo.On("Load", ctx, tmpFile.Name()).Return(nil, errors.New("read error"))

		service := NewPresentationService(repo, nil, nil, nil)
		_, err = service.LoadPresentation(ctx, tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loading presentation")
	})

	t.Run("invalid presentation", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "test*.md")
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(tmpFile.Name())
		}()
		require.NoError(t, tmpFile.Close())

		repo := new(MockPresentationRepository)
		presentation := &entities.Presentation{
			// Missing title
			Slides: []entities.Slide{{Content: "test"}},
		}
		repo.On("Load", ctx, tmpFile.Name()).Return(presentation, nil)

		service := NewPresentationService(repo, nil, nil, nil)
		_, err = service.LoadPresentation(ctx, tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid presentation")
	})
}

func TestPresentationService_ParsePresentation(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		parser := new(MockPresentationParser)
		content := []byte("# Test\n\nContent")
		presentation := &entities.Presentation{
			Title: "Test",
			Theme: "default",
			Slides: []entities.Slide{
				{Content: "# Slide 1"},
				{Content: "# Slide 2"},
			},
		}

		parser.On("Parse", content).Return(presentation, nil)

		service := NewPresentationService(nil, nil, parser, nil)
		result, err := service.ParsePresentation(ctx, content)

		require.NoError(t, err)
		assert.Equal(t, "Test", result.Title)
		assert.Len(t, result.Slides, 2)
		// Check indices were set
		assert.Equal(t, 0, result.Slides[0].Index)
		assert.Equal(t, 1, result.Slides[1].Index)
		// Check titles were extracted
		assert.Equal(t, "Slide 1", result.Slides[0].Title)
		assert.Equal(t, "Slide 2", result.Slides[1].Title)

		parser.AssertExpectations(t)
	})

	t.Run("empty content", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.ParsePresentation(ctx, []byte{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "presentation content cannot be empty")
	})

	t.Run("parser error", func(t *testing.T) {
		parser := new(MockPresentationParser)
		parser.On("Parse", mock.Anything).Return(nil, errors.New("parse error"))

		service := NewPresentationService(nil, nil, parser, nil)
		_, err := service.ParsePresentation(ctx, []byte("content"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing presentation")
	})
}

func TestPresentationService_RenderSlides(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		renderer := new(MockSlideRenderer)
		presentation := &entities.Presentation{
			Title: "Test",
			Slides: []entities.Slide{
				{Content: "# Slide 1", Index: 0},
				{Content: "# Slide 2", Index: 1},
			},
		}

		renderer.On("RenderSlide", &presentation.Slides[0]).Return(&ports.RenderedSlide{
			Slide: &presentation.Slides[0],
			HTML:  "<h1>Slide 1</h1>",
		}, nil)
		renderer.On("RenderSlide", &presentation.Slides[1]).Return(&ports.RenderedSlide{
			Slide: &presentation.Slides[1],
			HTML:  "<h1>Slide 2</h1>",
		}, nil)

		service := NewPresentationService(nil, nil, nil, renderer)
		results, err := service.RenderSlides(ctx, presentation)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "<h1>Slide 1</h1>", results[0].HTML)
		assert.Equal(t, "<h1>Slide 2</h1>", results[1].HTML)

		renderer.AssertExpectations(t)
	})

	t.Run("nil presentation", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.RenderSlides(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "presentation cannot be nil")
	})

	t.Run("render error", func(t *testing.T) {
		renderer := new(MockSlideRenderer)
		presentation := &entities.Presentation{
			Title: "Test",
			Slides: []entities.Slide{
				{Content: "# Slide 1", Index: 0},
			},
		}

		renderer.On("RenderSlide", mock.Anything).Return(nil, errors.New("render error"))

		service := NewPresentationService(nil, nil, nil, renderer)
		_, err := service.RenderSlides(ctx, presentation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rendering slide 1")
	})
}

func TestPresentationService_ApplyTheme(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		themeRepo := new(MockThemeRepository)
		theme := &entities.Theme{
			Name:        "custom",
			DisplayName: "Custom Theme",
			Version:     "1.0.0",
		}
		themeRepo.On("Get", ctx, "custom").Return(theme, nil)

		presentation := &entities.Presentation{
			Title: "Test",
			Theme: "default",
		}

		service := NewPresentationService(nil, themeRepo, nil, nil)
		err := service.ApplyTheme(ctx, presentation, "custom")

		require.NoError(t, err)
		assert.Equal(t, "custom", presentation.Theme)

		themeRepo.AssertExpectations(t)
	})

	t.Run("default theme when empty", func(t *testing.T) {
		themeRepo := new(MockThemeRepository)
		theme := &entities.Theme{
			Name:    "default",
			Version: "1.0.0",
		}
		themeRepo.On("Get", ctx, "default").Return(theme, nil)

		presentation := &entities.Presentation{Title: "Test"}

		service := NewPresentationService(nil, themeRepo, nil, nil)
		err := service.ApplyTheme(ctx, presentation, "")

		require.NoError(t, err)
		assert.Equal(t, "default", presentation.Theme)
	})

	t.Run("nil presentation", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		err := service.ApplyTheme(ctx, nil, "theme")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "presentation cannot be nil")
	})

	t.Run("theme not found", func(t *testing.T) {
		themeRepo := new(MockThemeRepository)
		themeRepo.On("Get", ctx, "nonexistent").Return(nil, errors.New("theme not found"))

		presentation := &entities.Presentation{Title: "Test"}

		service := NewPresentationService(nil, themeRepo, nil, nil)
		err := service.ApplyTheme(ctx, presentation, "nonexistent")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "getting theme nonexistent")
	})
}

func TestPresentationService_LoadPresentationFromReader(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		parser := new(MockPresentationParser)
		content := []byte("# Test\n\nContent")
		presentation := &entities.Presentation{
			Title: "Test",
			Theme: "default",
			Slides: []entities.Slide{
				{Content: "# Test"},
			},
		}

		parser.On("Parse", content).Return(presentation, nil)

		reader := bytes.NewReader(content)
		service := NewPresentationService(nil, nil, parser, nil)
		result, err := service.LoadPresentationFromReader(ctx, reader)

		require.NoError(t, err)
		assert.Equal(t, "Test", result.Title)
		parser.AssertExpectations(t)
	})

	t.Run("nil reader", func(t *testing.T) {
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.LoadPresentationFromReader(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reader cannot be nil")
	})

	t.Run("read error", func(t *testing.T) {
		// Create a reader that fails
		reader := &failingReader{}
		service := NewPresentationService(nil, nil, nil, nil)
		_, err := service.LoadPresentationFromReader(ctx, reader)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading content")
	})
}

// Helper types
type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read failed")
}
