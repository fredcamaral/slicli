package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// Mock implementations
type MockFileWatcher struct {
	mock.Mock
}

func (m *MockFileWatcher) Watch(ctx context.Context, path string) (<-chan ports.FileChangeEvent, error) {
	args := m.Called(ctx, path)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan ports.FileChangeEvent), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcher) Stop() error {
	args := m.Called()
	return args.Error(0)
}

type MockHTTPServer struct {
	mock.Mock
}

func (m *MockHTTPServer) Start(ctx context.Context, port int, host string) error {
	args := m.Called(ctx, port, host)
	return args.Error(0)
}

func (m *MockHTTPServer) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockHTTPServer) NotifyClients(event ports.UpdateEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockHTTPServer) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockBrowserLauncher struct {
	mock.Mock
}

func (m *MockBrowserLauncher) Launch(url string, noOpen bool) error {
	args := m.Called(url, noOpen)
	return args.Error(0)
}

func (m *MockBrowserLauncher) Detect() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

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

type MockRenderer struct {
	mock.Mock
}

func (m *MockRenderer) RenderPresentation(ctx context.Context, presentation *entities.Presentation) ([]byte, error) {
	args := m.Called(ctx, presentation)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockRenderer) RenderSlide(ctx context.Context, slide *entities.Slide) ([]byte, error) {
	args := m.Called(ctx, slide)
	return args.Get(0).([]byte), args.Error(1)
}

func TestNewLiveReloadService(t *testing.T) {
	watcher := &MockFileWatcher{}
	server := &MockHTTPServer{}
	browser := &MockBrowserLauncher{}
	presenter := &MockPresentationService{}
	renderer := &MockRenderer{}

	service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)
	assert.NotNil(t, service)
	assert.Equal(t, watcher, service.watcher)
	assert.Equal(t, server, service.server)
	assert.Equal(t, browser, service.browser)
	assert.Equal(t, presenter, service.presenter)
	assert.Equal(t, renderer, service.renderer)
	assert.False(t, service.watching)
}

func TestLiveReloadServiceStart(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		events := make(chan ports.FileChangeEvent)
		watcher.On("Watch", mock.Anything, "/test/file.md").Return((<-chan ports.FileChangeEvent)(events), nil)

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		require.NoError(t, err)
		assert.True(t, service.IsWatching())

		// Clean up
		close(events)
		_ = service.Stop()
		watcher.AssertExpectations(t)
	})

	t.Run("already watching", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		events := make(chan ports.FileChangeEvent)
		watcher.On("Watch", mock.Anything, "/test/file.md").Return((<-chan ports.FileChangeEvent)(events), nil)

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		require.NoError(t, err)

		// Try to start again
		err = service.Start(ctx, "/test/file.md")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already watching")

		// Clean up
		close(events)
		_ = service.Stop()
		watcher.AssertExpectations(t)
	})

	t.Run("watcher error", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		watcher.On("Watch", mock.Anything, "/test/file.md").Return(nil, errors.New("watch error"))

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "starting watcher")
		assert.False(t, service.IsWatching())

		watcher.AssertExpectations(t)
	})
}

func TestLiveReloadServiceStop(t *testing.T) {
	t.Run("stop when watching", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		events := make(chan ports.FileChangeEvent)
		watcher.On("Watch", mock.Anything, "/test/file.md").Return((<-chan ports.FileChangeEvent)(events), nil)

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		require.NoError(t, err)

		err = service.Stop()
		assert.NoError(t, err)
		assert.False(t, service.IsWatching())

		close(events)
		watcher.AssertExpectations(t)
	})

	t.Run("stop when not watching", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		err := service.Stop()
		assert.NoError(t, err)
	})
}

func TestLiveReloadServiceHandleEvents(t *testing.T) {
	t.Run("handle file change event", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		events := make(chan ports.FileChangeEvent, 1)
		watcher.On("Watch", mock.Anything, "/test/file.md").Return((<-chan ports.FileChangeEvent)(events), nil)

		// Set up presentation mocks
		presentation := &entities.Presentation{}
		presenter.On("LoadPresentation", mock.Anything, "/test/file.md").Return(presentation, nil)
		presenter.On("ApplyTheme", mock.Anything, presentation, "default").Return(nil)
		renderer.On("RenderPresentation", mock.Anything, presentation).Return([]byte("<html>test</html>"), nil)
		server.On("NotifyClients", mock.Anything).Return(nil)

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		require.NoError(t, err)

		// Send event
		events <- ports.FileChangeEvent{
			Path:      "/test/file.md",
			Type:      ports.Modified,
			Timestamp: time.Now(),
		}

		// Give handler time to process
		time.Sleep(100 * time.Millisecond)

		// Stop service
		_ = service.Stop()
		close(events)

		// Verify expectations
		presenter.AssertExpectations(t)
		renderer.AssertExpectations(t)
		server.AssertExpectations(t)
		watcher.AssertExpectations(t)
	})

	t.Run("handle parse error", func(t *testing.T) {
		watcher := &MockFileWatcher{}
		server := &MockHTTPServer{}
		browser := &MockBrowserLauncher{}
		presenter := &MockPresentationService{}
		renderer := &MockRenderer{}

		service := NewLiveReloadService(watcher, server, browser, presenter, renderer, nil)

		events := make(chan ports.FileChangeEvent, 1)
		watcher.On("Watch", mock.Anything, "/test/file.md").Return((<-chan ports.FileChangeEvent)(events), nil)

		// Set up presentation error
		presenter.On("LoadPresentation", mock.Anything, "/test/file.md").Return(nil, errors.New("parse error"))

		ctx := context.Background()
		err := service.Start(ctx, "/test/file.md")
		require.NoError(t, err)

		// Send event
		events <- ports.FileChangeEvent{
			Path:      "/test/file.md",
			Type:      ports.Modified,
			Timestamp: time.Now(),
		}

		// Give handler time to process
		time.Sleep(100 * time.Millisecond)

		// Stop service
		_ = service.Stop()
		close(events)

		// Should have called parse but not render or notify
		presenter.AssertExpectations(t)
		renderer.AssertNotCalled(t, "RenderPresentation")
		server.AssertNotCalled(t, "NotifyClients")
		watcher.AssertExpectations(t)
	})
}
