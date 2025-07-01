package entities

import (
	"testing"
	"time"
)

func TestSpeakerNotes_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		notes    SpeakerNotes
		expected bool
	}{
		{
			name:     "empty content",
			notes:    SpeakerNotes{Content: ""},
			expected: true,
		},
		{
			name:     "whitespace only",
			notes:    SpeakerNotes{Content: "   \n\t  "},
			expected: true,
		},
		{
			name:     "has content",
			notes:    SpeakerNotes{Content: "This is a note"},
			expected: false,
		},
		{
			name:     "content with whitespace",
			notes:    SpeakerNotes{Content: "  This is a note  "},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.notes.IsEmpty(); got != tt.expected {
				t.Errorf("SpeakerNotes.IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPresenterState_Progress(t *testing.T) {
	tests := []struct {
		name     string
		state    PresenterState
		expected float64
	}{
		{
			name: "zero slides",
			state: PresenterState{
				CurrentSlide: 0,
				TotalSlides:  0,
			},
			expected: 0,
		},
		{
			name: "first slide of many",
			state: PresenterState{
				CurrentSlide: 0,
				TotalSlides:  10,
			},
			expected: 0,
		},
		{
			name: "middle slide",
			state: PresenterState{
				CurrentSlide: 5,
				TotalSlides:  10,
			},
			expected: 50,
		},
		{
			name: "last slide",
			state: PresenterState{
				CurrentSlide: 9,
				TotalSlides:  10,
			},
			expected: 90,
		},
		{
			name: "single slide",
			state: PresenterState{
				CurrentSlide: 0,
				TotalSlides:  1,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.Progress(); got != tt.expected {
				t.Errorf("PresenterState.Progress() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewSyncEvent(t *testing.T) {
	eventType := "navigation"
	data := map[string]interface{}{
		"action": "next",
		"slide":  5,
	}

	event := NewSyncEvent(eventType, data)

	if event.Type != eventType {
		t.Errorf("NewSyncEvent() Type = %v, want %v", event.Type, eventType)
	}

	if len(event.Data) != len(data) {
		t.Errorf("NewSyncEvent() Data length = %v, want %v", len(event.Data), len(data))
	}

	for key, value := range data {
		if event.Data[key] != value {
			t.Errorf("NewSyncEvent() Data[%s] = %v, want %v", key, event.Data[key], value)
		}
	}

	// Check that timestamp is recent (within last second)
	if time.Since(event.Timestamp) > time.Second {
		t.Errorf("NewSyncEvent() Timestamp is too old: %v", event.Timestamp)
	}
}
