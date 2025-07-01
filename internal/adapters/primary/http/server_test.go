package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// MockPresentationService is a mock for PresentationService
type MockPresentationService struct {
	mock.Mock
}

func (m *MockPresentationService) LoadPresentation(ctx context.Context, path string) (*entities.Presentation, error) {
	args := m.Called(ctx, path)
	if p := args.Get(0); p != nil {
		return p.(*entities.Presentation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPresentationService) ParsePresentation(ctx context.Context, content []byte) (*entities.Presentation, error) {
	args := m.Called(ctx, content)
	if p := args.Get(0); p != nil {
		return p.(*entities.Presentation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPresentationService) RenderSlides(ctx context.Context, presentation *entities.Presentation) ([]ports.RenderedSlide, error) {
	args := m.Called(ctx, presentation)
	if slides := args.Get(0); slides != nil {
		return slides.([]ports.RenderedSlide), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPresentationService) WatchPresentation(ctx context.Context, path string) (<-chan ports.FileChangeEvent, error) {
	args := m.Called(ctx, path)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan ports.FileChangeEvent), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPresentationService) ApplyTheme(ctx context.Context, presentation *entities.Presentation, themeName string) error {
	args := m.Called(ctx, presentation, themeName)
	return args.Error(0)
}

// MockRenderer is a mock for Renderer
type MockRenderer struct {
	mock.Mock
}

func (m *MockRenderer) RenderPresentation(ctx context.Context, p *entities.Presentation) ([]byte, error) {
	args := m.Called(ctx, p)
	if b := args.Get(0); b != nil {
		return b.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRenderer) RenderSlide(ctx context.Context, s *entities.Slide) ([]byte, error) {
	args := m.Called(ctx, s)
	if b := args.Get(0); b != nil {
		return b.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestServerLifecycle(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	ctx := context.Background()

	t.Run("start server", func(t *testing.T) {
		err := server.Start(ctx, 0, "localhost")
		require.NoError(t, err)
		assert.True(t, server.IsRunning())
	})

	t.Run("server already running", func(t *testing.T) {
		err := server.Start(ctx, 0, "localhost")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})

	t.Run("stop server", func(t *testing.T) {
		err := server.Stop(ctx)
		require.NoError(t, err)
		assert.False(t, server.IsRunning())
	})

	t.Run("server not running", func(t *testing.T) {
		err := server.Stop(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestNotifyClients(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	ctx := context.Background()

	t.Run("notify when server not running", func(t *testing.T) {
		event := ports.UpdateEvent{
			Type:      ports.EventTypeReload,
			Timestamp: time.Now(),
		}
		err := server.NotifyClients(event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("notify when server running", func(t *testing.T) {
		err := server.Start(ctx, 0, "localhost")
		require.NoError(t, err)
		defer func() { _ = server.Stop(ctx) }()

		event := ports.UpdateEvent{
			Type:      ports.EventTypeReload,
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "test"},
		}
		err = server.NotifyClients(event)
		assert.NoError(t, err)
	})
}

func TestServerHTTPEndpoints(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	// Create test server directly
	mux := server.setupRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	t.Run("config endpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/config")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})

	t.Run("config endpoint method not allowed", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/config", "text/plain", nil)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestBroadcastMethods(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	ctx := context.Background()

	// Start server
	err := server.Start(ctx, 0, "localhost")
	require.NoError(t, err)
	defer func() { _ = server.Stop(ctx) }()

	t.Run("broadcast reload", func(t *testing.T) {
		// Should not panic
		server.BroadcastReload()
	})

	t.Run("broadcast file change", func(t *testing.T) {
		// Should not panic
		server.BroadcastFileChange("test.md")
	})
}

func TestServerConfigValidation(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)

	t.Run("panics with nil config", func(t *testing.T) {
		assert.Panics(t, func() {
			NewServer(presenter, renderer, nil)
		})
	})

	t.Run("accepts valid config", func(t *testing.T) {
		config := getTestServerConfig()
		server := NewServer(presenter, renderer, config)
		assert.NotNil(t, server)
	})
}
