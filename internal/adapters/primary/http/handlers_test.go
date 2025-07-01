package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// getTestServerConfig returns a test server configuration
func getTestServerConfig() *entities.ServerConfig {
	return &entities.ServerConfig{
		Host:            "localhost",
		Port:            8080,
		ReadTimeout:     30,
		WriteTimeout:    30,
		ShutdownTimeout: 5,
		CORSOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		},
	}
}

func TestHandlePresentation(t *testing.T) {
	t.Run("successful presentation render", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		presentation := &entities.Presentation{
			Title:  "Test Presentation",
			Author: "Test Author",
			Theme:  "default",
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Slides: []entities.Slide{
				{Index: 0, Title: "Slide 1", HTML: "<h1>Slide 1</h1>"},
			},
		}

		html := []byte("<html><body>Test</body></html>")

		// Set the presentation on the server
		server.SetPresentation(presentation)
		renderer.On("RenderPresentation", mock.Anything, presentation).Return(html, nil)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handlePresentation(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Equal(t, html, body)

		presenter.AssertExpectations(t)
		renderer.AssertExpectations(t)
	})

	t.Run("presentation not loaded", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		// Don't set any presentation - it will create a default one
		defaultPresentation := &entities.Presentation{
			Title: "No Presentation Loaded",
			Theme: "default",
			Slides: []entities.Slide{
				{Index: 0, Title: "No presentation loaded", HTML: "<h1>No presentation loaded</h1><p>Please specify a presentation file.</p>"},
			},
		}

		html := []byte("<html><body>No presentation</body></html>")
		renderer.On("RenderPresentation", mock.Anything, mock.MatchedBy(func(p *entities.Presentation) bool {
			return p.Title == defaultPresentation.Title
		})).Return(html, nil)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handlePresentation(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, html, body)

		renderer.AssertExpectations(t)
	})

	t.Run("renderer error", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		presentation := &entities.Presentation{Title: "Test"}

		// Set the presentation on the server
		server.SetPresentation(presentation)
		renderer.On("RenderPresentation", mock.Anything, presentation).Return(nil, errors.New("render error"))

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handlePresentation(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		presenter.AssertExpectations(t)
		renderer.AssertExpectations(t)
	})

	t.Run("404 for non-root path", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		req := httptest.NewRequest("GET", "/unknown", nil)
		w := httptest.NewRecorder()

		server.handlePresentation(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandleSlides(t *testing.T) {
	t.Run("successful slides response", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		presentation := &entities.Presentation{
			Title:  "Test Presentation",
			Author: "Test Author",
			Theme:  "default",
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Slides: []entities.Slide{
				{Index: 0, Title: "Slide 1", HTML: "<h1>Slide 1</h1>", Notes: "Notes 1"},
				{Index: 1, Title: "Slide 2", HTML: "<h1>Slide 2</h1>"},
			},
		}

		// Set the presentation on the server
		server.SetPresentation(presentation)

		req := httptest.NewRequest("GET", "/api/slides", nil)
		w := httptest.NewRecorder()

		server.handleSlides(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var slidesResp SlidesResponse
		err := json.Unmarshal(body, &slidesResp)
		require.NoError(t, err)

		assert.Equal(t, "Test Presentation", slidesResp.Title)
		assert.Equal(t, "Test Author", slidesResp.Author)
		assert.Equal(t, "2024-01-01", slidesResp.Date)
		assert.Equal(t, "default", slidesResp.Theme)
		assert.Len(t, slidesResp.Slides, 2)
		assert.Equal(t, "Notes 1", slidesResp.Slides[0].Notes)

		presenter.AssertExpectations(t)
	})

	t.Run("method not allowed", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		req := httptest.NewRequest("POST", "/api/slides", nil)
		w := httptest.NewRecorder()

		server.handleSlides(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("no presentation loaded", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		// Don't set any presentation - it will return a default

		req := httptest.NewRequest("GET", "/api/slides", nil)
		w := httptest.NewRecorder()

		server.handleSlides(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var slidesResp SlidesResponse
		err := json.Unmarshal(body, &slidesResp)
		require.NoError(t, err)
		assert.Equal(t, "No Presentation Loaded", slidesResp.Title)

		presenter.AssertExpectations(t)
	})
}

func TestHandleConfig(t *testing.T) {
	t.Run("successful config response", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		req := httptest.NewRequest("GET", "/api/config", nil)
		w := httptest.NewRecorder()

		server.handleConfig(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var config ConfigResponse
		err := json.Unmarshal(body, &config)
		require.NoError(t, err)

		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "default", config.Theme)
		assert.Equal(t, "/ws", config.WebSocketURL)
		assert.True(t, config.LiveReload)
		assert.Contains(t, config.SupportedThemes, "default")
	})

	t.Run("method not allowed", func(t *testing.T) {
		presenter := new(MockPresentationService)
		renderer := new(MockRenderer)
		server := NewServer(presenter, renderer, getTestServerConfig())

		req := httptest.NewRequest("DELETE", "/api/config", nil)
		w := httptest.NewRecorder()

		server.handleConfig(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestPresentationToResponse(t *testing.T) {
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	t.Run("full presentation", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title:  "Test",
			Author: "Author",
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Theme:  "dark",
			Slides: []entities.Slide{
				{Index: 0, Title: "Slide 1", HTML: "<h1>Test</h1>", Notes: "Notes"},
			},
		}

		response := server.presentationToResponse(presentation)

		assert.Equal(t, "Test", response.Title)
		assert.Equal(t, "Author", response.Author)
		assert.Equal(t, "2024-01-01", response.Date)
		assert.Equal(t, "dark", response.Theme)
		assert.Len(t, response.Slides, 1)
		assert.Equal(t, "Notes", response.Slides[0].Notes)
	})

	t.Run("zero date", func(t *testing.T) {
		presentation := &entities.Presentation{
			Title: "Test",
			Date:  time.Time{},
		}

		response := server.presentationToResponse(presentation)

		assert.Equal(t, "", response.Date)
	})
}
