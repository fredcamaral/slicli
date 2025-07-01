package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// LiveReloadService coordinates file watching with WebSocket notifications
type LiveReloadService struct {
	watcher          ports.FileWatcher
	server           ports.HTTPServer
	browser          ports.BrowserLauncher
	presenter        ports.PresentationService
	renderer         ports.Renderer
	logger           *slog.Logger
	mu               sync.Mutex
	watching         bool
	watchCancel      context.CancelFunc
	presentationPath string
}

// NewLiveReloadService creates a new live reload service
func NewLiveReloadService(
	watcher ports.FileWatcher,
	server ports.HTTPServer,
	browser ports.BrowserLauncher,
	presenter ports.PresentationService,
	renderer ports.Renderer,
	logger *slog.Logger,
) *LiveReloadService {
	if logger == nil {
		logger = slog.Default()
	}

	return &LiveReloadService{
		watcher:   watcher,
		server:    server,
		browser:   browser,
		presenter: presenter,
		renderer:  renderer,
		logger:    logger.With("service", "live_reload"),
	}
}

// Start starts the live reload service
func (s *LiveReloadService) Start(ctx context.Context, filePath string) error {
	s.mu.Lock()
	if s.watching {
		s.mu.Unlock()
		return errors.New("already watching")
	}
	s.watching = true
	s.presentationPath = filePath
	s.mu.Unlock()

	// Create a cancellable context for the watcher
	watchCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.watchCancel = cancel
	s.mu.Unlock()

	events, err := s.watcher.Watch(watchCtx, filePath)
	if err != nil {
		s.mu.Lock()
		s.watching = false
		s.watchCancel = nil
		s.mu.Unlock()
		return fmt.Errorf("starting watcher: %w", err)
	}

	go s.handleEvents(watchCtx, events)

	return nil
}

// Stop stops the live reload service
func (s *LiveReloadService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.watching {
		return nil
	}

	if s.watchCancel != nil {
		s.watchCancel()
		s.watchCancel = nil
	}

	s.watching = false
	return nil
}

// IsWatching returns whether the service is currently watching
func (s *LiveReloadService) IsWatching() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watching
}

// handleEvents handles file change events
func (s *LiveReloadService) handleEvents(ctx context.Context, events <-chan ports.FileChangeEvent) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-events:
			if !ok {
				return
			}

			s.logger.Info("File changed detected",
				slog.String("path", event.Path),
				slog.String("type", event.Type.String()),
				slog.Time("timestamp", event.Timestamp),
			)

			// Reload the presentation
			if err := s.reloadPresentation(); err != nil {
				s.logger.Error("Failed to reload presentation",
					slog.String("error", err.Error()),
					slog.String("path", event.Path),
					slog.String("change_type", event.Type.String()),
				)
				continue
			}

			// Notify all connected clients
			updateEvent := ports.UpdateEvent{
				Type:      "reload",
				Timestamp: event.Timestamp,
				Data: map[string]interface{}{
					"file": event.Path,
					"type": event.Type.String(),
				},
			}

			if err := s.server.NotifyClients(updateEvent); err != nil {
				s.logger.Warn("Failed to notify WebSocket clients",
					slog.String("error", err.Error()),
					slog.String("event_type", "reload"),
					slog.String("file", event.Path),
				)
			} else {
				s.logger.Debug("WebSocket clients notified successfully",
					slog.String("event_type", "reload"),
					slog.String("file", event.Path),
				)
			}
		}
	}
}

// reloadPresentation reloads the presentation from disk
func (s *LiveReloadService) reloadPresentation() error {
	s.mu.Lock()
	path := s.presentationPath
	s.mu.Unlock()

	if path == "" {
		return errors.New("no presentation path set")
	}

	ctx := context.Background()

	// Load the presentation from disk
	presentation, err := s.presenter.LoadPresentation(ctx, path)
	if err != nil {
		return fmt.Errorf("loading presentation: %w", err)
	}

	// Apply theme if needed (using default theme for now)
	if err := s.presenter.ApplyTheme(ctx, presentation, "default"); err != nil {
		return fmt.Errorf("applying theme: %w", err)
	}

	// Render the presentation
	html, err := s.renderer.RenderPresentation(ctx, presentation)
	if err != nil {
		return fmt.Errorf("rendering presentation: %w", err)
	}

	// Update the server's presentation
	// Note: This assumes the server has a SetPresentation method
	// which we'll need to add to the HTTPServer interface or handle differently
	s.logger.Info("Presentation reloaded successfully",
		slog.Int("html_size_bytes", len(html)),
		slog.String("presentation_path", path),
	)

	return nil
}
