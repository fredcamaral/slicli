package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	wrapped := loggingMiddleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic and should log
	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("normal operation", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("normal response"))
		})

		wrapped := recoveryMiddleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("panic recovery", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		wrapped := recoveryMiddleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// Should not panic
		wrapped.ServeHTTP(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	wrapped := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
	}

	t.Run("write header", func(t *testing.T) {
		wrapped.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, wrapped.status)
	})

	t.Run("write data", func(t *testing.T) {
		data := []byte("test data")
		n, err := wrapped.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, len(data), wrapped.size)
	})

	t.Run("multiple writes", func(t *testing.T) {
		wrapped.size = 0 // Reset

		data1 := []byte("first ")
		data2 := []byte("second")

		n1, err := wrapped.Write(data1)
		assert.NoError(t, err)
		assert.Equal(t, len(data1), n1)

		n2, err := wrapped.Write(data2)
		assert.NoError(t, err)
		assert.Equal(t, len(data2), n2)

		assert.Equal(t, len(data1)+len(data2), wrapped.size)
	})
}
