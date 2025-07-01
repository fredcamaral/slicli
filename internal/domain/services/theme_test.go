package services

import (
	"context"
	"errors"
	"html/template"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type MockThemeLoader struct {
	mock.Mock
}

func (m *MockThemeLoader) Load(ctx context.Context, name string) (*entities.ThemeEngine, error) {
	args := m.Called(ctx, name)
	if theme := args.Get(0); theme != nil {
		return theme.(*entities.ThemeEngine), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockThemeLoader) List(ctx context.Context) ([]entities.ThemeInfo, error) {
	args := m.Called(ctx)
	if info := args.Get(0); info != nil {
		return info.([]entities.ThemeInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockThemeLoader) Exists(ctx context.Context, name string) bool {
	args := m.Called(ctx, name)
	return args.Bool(0)
}

func (m *MockThemeLoader) Reload(ctx context.Context, name string) (*entities.ThemeEngine, error) {
	args := m.Called(ctx, name)
	if theme := args.Get(0); theme != nil {
		return theme.(*entities.ThemeEngine), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockThemeCache struct {
	mock.Mock
}

func (m *MockThemeCache) Get(name string) (*entities.ThemeEngine, bool) {
	args := m.Called(name)
	if theme := args.Get(0); theme != nil {
		return theme.(*entities.ThemeEngine), args.Bool(1)
	}
	return nil, args.Bool(1)
}

func (m *MockThemeCache) Set(name string, theme *entities.ThemeEngine) {
	m.Called(name, theme)
}

func (m *MockThemeCache) Remove(name string) {
	m.Called(name)
}

func (m *MockThemeCache) Clear() {
	m.Called()
}

func (m *MockThemeCache) Stats() entities.CacheStats {
	args := m.Called()
	return args.Get(0).(entities.CacheStats)
}

type MockAssetProcessor struct {
	mock.Mock
}

func (m *MockAssetProcessor) Process(content []byte, contentType string, variables map[string]string) ([]byte, error) {
	args := m.Called(content, contentType, variables)
	if result := args.Get(0); result != nil {
		return result.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAssetProcessor) ProcessCSS(content []byte, variables map[string]string) ([]byte, error) {
	args := m.Called(content, variables)
	if result := args.Get(0); result != nil {
		return result.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAssetProcessor) ProcessJS(content []byte, variables map[string]string) ([]byte, error) {
	args := m.Called(content, variables)
	if result := args.Get(0); result != nil {
		return result.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAssetProcessor) MinifyCSS(content []byte) ([]byte, error) {
	args := m.Called(content)
	if result := args.Get(0); result != nil {
		return result.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAssetProcessor) MinifyJS(content []byte) ([]byte, error) {
	args := m.Called(content)
	if result := args.Get(0); result != nil {
		return result.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

// Helper functions
func createMockTheme(name string) *entities.ThemeEngine {
	// Create templates with actual parsing
	presentationTmpl, _ := template.New("presentation").Parse(`<html><body>{{.Presentation.Title}}</body></html>`)
	slideTmpl, _ := template.New("slide").Parse(`<div class="slide">{{.Slide.Content}}</div>`)

	return &entities.ThemeEngine{
		Name: name,
		Path: "/themes/" + name,
		Templates: map[string]*template.Template{
			"presentation": presentationTmpl,
			"slide":        slideTmpl,
		},
		Assets: map[string]*entities.ThemeAsset{
			"css/main.css": {
				Path:        "/themes/" + name + "/assets/css/main.css",
				Content:     []byte("body { color: var(--primary-color); }"),
				ContentType: "text/css",
				Hash:        "abc123",
			},
		},
		Config: entities.ThemeEngineConfig{
			Variables: map[string]string{
				"primary-color": "#000",
			},
		},
		LoadedAt: time.Now(),
	}
}

func TestThemeService_LoadTheme_FromCache(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	mockCache.On("Get", "test").Return(theme, true)

	result, err := service.GetTheme(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, theme, result)

	mockCache.AssertExpectations(t)
	mockLoader.AssertNotCalled(t, "Load")
}

func TestThemeService_LoadTheme_FromLoader(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	mockCache.On("Get", "test").Return(nil, false)
	mockLoader.On("Load", ctx, "test").Return(theme, nil)

	// Mock asset processing
	mockProcessor.On("ProcessCSS",
		[]byte("body { color: var(--primary-color); }"),
		theme.Config.Variables,
	).Return([]byte("body { color: #000; }"), nil)

	mockCache.On("Set", "test", theme).Return()

	result, err := service.GetTheme(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, theme, result)

	mockCache.AssertExpectations(t)
	mockLoader.AssertExpectations(t)
}

func TestThemeService_LoadTheme_LoaderError(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	mockCache.On("Get", "test").Return(nil, false)
	mockLoader.On("Load", ctx, "test").Return(nil, errors.New("theme not found"))
	mockLoader.On("Load", ctx, "default").Return(nil, errors.New("default theme not found"))

	_, err := service.GetTheme(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "theme not found")

	mockCache.AssertExpectations(t)
	mockLoader.AssertExpectations(t)
}

func TestThemeService_RenderPresentation(t *testing.T) {
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	presentation := &entities.Presentation{
		Title:  "Test Presentation",
		Author: "Test Author",
		Slides: []entities.Slide{
			{ID: "1", Content: "Slide 1"},
			{ID: "2", Content: "Slide 2"},
		},
	}

	html, err := service.RenderPresentation(theme, presentation)
	require.NoError(t, err)
	assert.Contains(t, string(html), "Test Presentation")
}

func TestThemeService_RenderSlide(t *testing.T) {
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	slide := &entities.Slide{
		ID:      "1",
		Content: "# Test Slide",
		HTML:    "<h1>Test Slide</h1>",
	}

	html, err := service.RenderSlide(theme, slide, 1, 10)
	require.NoError(t, err)
	assert.Contains(t, string(html), "Test Slide")
}

func TestThemeService_ServeAsset(t *testing.T) {
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	// Create theme with already processed asset
	theme := createMockTheme("test")
	theme.Assets["css/main.css"].Content = []byte("body { color: #000; }")

	asset, err := service.ServeAsset(theme, "css/main.css")
	require.NoError(t, err)
	assert.Equal(t, []byte("body { color: #000; }"), asset.Content)
	assert.Equal(t, "text/css", asset.ContentType)
	assert.NotEmpty(t, asset.Hash)
}

func TestThemeService_ServeAsset_NotFound(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	mockCache.On("Get", "test").Return(theme, true)

	theme, err := service.GetTheme(ctx, "test")
	require.NoError(t, err)

	_, err = service.ServeAsset(theme, "css/nonexistent.css")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	mockCache.AssertExpectations(t)
}

func TestThemeService_ListThemes(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	themes := []entities.ThemeInfo{
		{Name: "default", DisplayName: "Default Theme"},
		{Name: "dark", DisplayName: "Dark Theme"},
	}

	mockLoader.On("List", ctx).Return(themes, nil)

	result, err := service.ListThemes(ctx)
	require.NoError(t, err)
	assert.Equal(t, themes, result)

	mockLoader.AssertExpectations(t)
}

func TestThemeService_ReloadTheme(t *testing.T) {
	ctx := context.Background()
	mockLoader := new(MockThemeLoader)
	mockCache := new(MockThemeCache)
	mockProcessor := new(MockAssetProcessor)

	service := NewThemeService(mockLoader, mockCache, mockProcessor)

	theme := createMockTheme("test")
	mockCache.On("Remove", "test").Return()
	mockLoader.On("Reload", ctx, "test").Return(theme, nil)

	// Mock asset processing
	mockProcessor.On("ProcessCSS",
		[]byte("body { color: var(--primary-color); }"),
		theme.Config.Variables,
	).Return([]byte("body { color: #000; }"), nil)

	mockCache.On("Set", "test", theme).Return()

	err := service.ReloadTheme(ctx, "test")
	require.NoError(t, err)

	mockCache.AssertExpectations(t)
	mockLoader.AssertExpectations(t)
}
