package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// PresentationSyncService manages synchronization between presenter and audience views
type PresentationSyncService struct {
	state        *entities.PresenterState
	clients      map[string]chan entities.SyncEvent
	mu           sync.RWMutex
	ticker       *time.Ticker
	presentation *entities.Presentation
	notesService ports.NotesService
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewPresentationSyncService creates a new presentation sync service
func NewPresentationSyncService(presentation *entities.Presentation, notesService ports.NotesService) *PresentationSyncService {
	ctx, cancel := context.WithCancel(context.Background())

	s := &PresentationSyncService{
		state: &entities.PresenterState{
			CurrentSlide: 0,
			TotalSlides:  len(presentation.Slides),
			StartTime:    time.Now(),
			IsPaused:     false,
		},
		clients:      make(map[string]chan entities.SyncEvent),
		presentation: presentation,
		notesService: notesService,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Initialize with first slide info
	s.updateSlideInfo()

	// Start timer
	s.startTimer()

	return s
}

// Subscribe adds a client to receive sync events
func (s *PresentationSyncService) Subscribe(clientID string) <-chan entities.SyncEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan entities.SyncEvent, 10)
	s.clients[clientID] = ch

	return ch
}

// Unsubscribe removes a client from sync events
func (s *PresentationSyncService) Unsubscribe(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.clients[clientID]; exists {
		close(ch)
		delete(s.clients, clientID)
	}
}

// Broadcast sends an event to all connected clients
func (s *PresentationSyncService) Broadcast(event entities.SyncEvent) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Update state based on event
	if err := s.updateState(event); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	// Broadcast to all clients
	for clientID, ch := range s.clients {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Client too slow, log and continue
			fmt.Printf("Warning: Client %s is slow, skipping event\n", clientID)
		}
	}

	return nil
}

// GetState returns the current presenter state
func (s *PresentationSyncService) GetState() *entities.PresenterState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	stateCopy := *s.state
	if s.state.Notes != nil {
		notesCopy := *s.state.Notes
		stateCopy.Notes = &notesCopy
	}

	// Update elapsed time if not paused
	if !s.state.IsPaused {
		stateCopy.ElapsedTime = time.Since(s.state.StartTime)
	}

	return &stateCopy
}

// updateState updates the internal state based on sync events
func (s *PresentationSyncService) updateState(event entities.SyncEvent) error {
	switch event.Type {
	case "navigation":
		return s.handleNavigation(event.Data)
	case "timer":
		return s.handleTimer(event.Data)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// handleNavigation processes navigation events
func (s *PresentationSyncService) handleNavigation(data map[string]interface{}) error {
	action, ok := data["action"].(string)
	if !ok {
		return errors.New("invalid action in navigation event")
	}

	switch action {
	case "next":
		if s.state.CurrentSlide < s.state.TotalSlides-1 {
			s.state.CurrentSlide++
		}
	case "prev":
		if s.state.CurrentSlide > 0 {
			s.state.CurrentSlide--
		}
	case "goto":
		if slide, ok := data["slide"].(float64); ok {
			slideNum := int(slide)
			if slideNum >= 0 && slideNum < s.state.TotalSlides {
				s.state.CurrentSlide = slideNum
			}
		}
	case "first":
		s.state.CurrentSlide = 0
	case "last":
		s.state.CurrentSlide = s.state.TotalSlides - 1
	default:
		return fmt.Errorf("unknown navigation action: %s", action)
	}

	// Update slide-specific information
	s.updateSlideInfo()

	return nil
}

// handleTimer processes timer events
func (s *PresentationSyncService) handleTimer(data map[string]interface{}) error {
	action, ok := data["action"].(string)
	if !ok {
		return errors.New("invalid action in timer event")
	}

	switch action {
	case "pause":
		if !s.state.IsPaused {
			s.state.IsPaused = true
			s.state.ElapsedTime = time.Since(s.state.StartTime)
		}
	case "resume":
		if s.state.IsPaused {
			s.state.StartTime = time.Now().Add(-s.state.ElapsedTime)
			s.state.IsPaused = false
		}
	case "reset":
		s.state.StartTime = time.Now()
		s.state.ElapsedTime = 0
		s.state.IsPaused = false
	default:
		return fmt.Errorf("unknown timer action: %s", action)
	}

	return nil
}

// updateSlideInfo updates notes and next slide information
func (s *PresentationSyncService) updateSlideInfo() {
	if s.state.CurrentSlide >= 0 && s.state.CurrentSlide < len(s.presentation.Slides) {
		// Get notes for current slide
		if s.notesService != nil {
			slideID := fmt.Sprintf("slide-%d", s.state.CurrentSlide)
			notes, err := s.notesService.GetNotes(slideID)
			if err == nil && !notes.IsEmpty() {
				s.state.Notes = notes
			} else {
				s.state.Notes = nil
			}
		}

		// Get title of next slide
		nextIndex := s.state.CurrentSlide + 1
		if nextIndex < len(s.presentation.Slides) {
			s.state.NextSlideTitle = s.presentation.Slides[nextIndex].Title
		} else {
			s.state.NextSlideTitle = "End of presentation"
		}
	}
}

// startTimer starts the presentation timer
func (s *PresentationSyncService) startTimer() {
	s.ticker = time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.ticker.C:
				// Timer tick - elapsed time is calculated in GetState()
				// This could be used for periodic updates if needed
			}
		}
	}()
}

// Stop stops the sync service
func (s *PresentationSyncService) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all client channels
	for clientID, ch := range s.clients {
		close(ch)
		delete(s.clients, clientID)
	}
}
