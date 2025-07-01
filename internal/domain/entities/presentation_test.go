package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresentation_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Presentation
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid presentation",
			setup: func() *Presentation {
				return &Presentation{
					Title: "Test Presentation",
					Theme: "default",
					Slides: []Slide{
						{Content: "# Slide 1", Index: 0},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "missing title",
			setup: func() *Presentation {
				return &Presentation{
					Theme: "default",
					Slides: []Slide{
						{Content: "# Slide 1", Index: 0},
					},
				}
			},
			wantErr: true,
			errMsg:  "presentation title is required",
		},
		{
			name: "no slides",
			setup: func() *Presentation {
				return &Presentation{
					Title:  "Test Presentation",
					Theme:  "default",
					Slides: []Slide{},
				}
			},
			wantErr: true,
			errMsg:  "presentation must have at least one slide",
		},
		{
			name: "invalid slide",
			setup: func() *Presentation {
				return &Presentation{
					Title: "Test Presentation",
					Theme: "default",
					Slides: []Slide{
						{Content: "", Index: 0}, // Empty content
					},
				}
			},
			wantErr: true,
			errMsg:  "slide 1 validation failed",
		},
		{
			name: "default theme applied when empty",
			setup: func() *Presentation {
				return &Presentation{
					Title: "Test Presentation",
					Theme: "", // Empty theme
					Slides: []Slide{
						{Content: "# Slide 1", Index: 0},
					},
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setup()
			err := p.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Check default theme was applied
				if tt.name == "default theme applied when empty" {
					assert.Equal(t, "default", p.Theme)
				}
			}
		})
	}
}

func TestPresentation_GetSlideByIndex(t *testing.T) {
	p := &Presentation{
		Title: "Test",
		Slides: []Slide{
			{Content: "Slide 1", Index: 0},
			{Content: "Slide 2", Index: 1},
			{Content: "Slide 3", Index: 2},
		},
	}

	tests := []struct {
		name    string
		index   int
		wantErr bool
		want    string
	}{
		{
			name:    "valid first index",
			index:   0,
			wantErr: false,
			want:    "Slide 1",
		},
		{
			name:    "valid middle index",
			index:   1,
			wantErr: false,
			want:    "Slide 2",
		},
		{
			name:    "valid last index",
			index:   2,
			wantErr: false,
			want:    "Slide 3",
		},
		{
			name:    "negative index",
			index:   -1,
			wantErr: true,
		},
		{
			name:    "index too large",
			index:   3,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide, err := p.GetSlideByIndex(tt.index)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, slide)
			} else {
				require.NoError(t, err)
				require.NotNil(t, slide)
				assert.Equal(t, tt.want, slide.Content)
			}
		})
	}
}

func TestPresentation_SlideCount(t *testing.T) {
	tests := []struct {
		name   string
		slides []Slide
		want   int
	}{
		{
			name:   "no slides",
			slides: []Slide{},
			want:   0,
		},
		{
			name:   "one slide",
			slides: []Slide{{Content: "test"}},
			want:   1,
		},
		{
			name:   "multiple slides",
			slides: []Slide{{Content: "1"}, {Content: "2"}, {Content: "3"}},
			want:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Presentation{
				Title:  "Test",
				Slides: tt.slides,
			}
			assert.Equal(t, tt.want, p.SlideCount())
		})
	}
}

func TestPresentation_CompleteStruct(t *testing.T) {
	now := time.Now()
	p := &Presentation{
		ID:     "test-123",
		Title:  "Complete Test",
		Theme:  "dark",
		Author: "Test Author",
		Date:   now,
		Metadata: map[string]interface{}{
			"custom": "value",
			"tags":   []string{"test", "demo"},
		},
		Slides: []Slide{
			{Content: "# First"},
			{Content: "# Second"},
		},
	}

	// Test all fields are properly set
	assert.Equal(t, "test-123", p.ID)
	assert.Equal(t, "Complete Test", p.Title)
	assert.Equal(t, "dark", p.Theme)
	assert.Equal(t, "Test Author", p.Author)
	assert.Equal(t, now, p.Date)
	assert.Equal(t, "value", p.Metadata["custom"])
	assert.Equal(t, []string{"test", "demo"}, p.Metadata["tags"])
	assert.Len(t, p.Slides, 2)
}
