package entities

import (
	"strings"
	"time"
)

// SpeakerNotes represents notes associated with a slide
type SpeakerNotes struct {
	SlideID string `json:"slideId"`
	Content string `json:"content"`
	HTML    string `json:"html"`
}

// IsEmpty returns true if the notes have no content
func (n *SpeakerNotes) IsEmpty() bool {
	return strings.TrimSpace(n.Content) == ""
}

// PresenterState represents the current state of the presentation
type PresenterState struct {
	CurrentSlide   int           `json:"currentSlide"`
	TotalSlides    int           `json:"totalSlides"`
	ElapsedTime    time.Duration `json:"elapsedTime"`
	StartTime      time.Time     `json:"startTime"`
	IsPaused       bool          `json:"isPaused"`
	Notes          *SpeakerNotes `json:"notes,omitempty"`
	NextSlideTitle string        `json:"nextSlideTitle"`
}

// Progress returns the presentation progress as a percentage
func (p *PresenterState) Progress() float64 {
	if p.TotalSlides == 0 {
		return 0
	}
	return float64(p.CurrentSlide) / float64(p.TotalSlides) * 100
}

// SyncEvent represents a synchronization event between presenter and audience views
type SyncEvent struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewSyncEvent creates a new sync event
func NewSyncEvent(eventType string, data map[string]interface{}) SyncEvent {
	return SyncEvent{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
}
