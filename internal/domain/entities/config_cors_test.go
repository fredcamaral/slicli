package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConfig_CORSOrigins(t *testing.T) {
	t.Run("valid CORS origins", func(t *testing.T) {
		config := ServerConfig{
			Host: "localhost",
			Port: 8080,
			CORSOrigins: []string{
				"http://localhost:3000",
				"https://example.com",
				"http://127.0.0.1:8080",
			},
		}

		err := config.Validate()
		require.NoError(t, err)
	})

	t.Run("wildcard origin allowed", func(t *testing.T) {
		config := ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{"*"},
		}

		err := config.Validate()
		require.NoError(t, err)
	})

	t.Run("invalid CORS origin - no protocol", func(t *testing.T) {
		config := ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{"example.com"},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CORS origin format")
	})

	t.Run("invalid CORS origin - empty string", func(t *testing.T) {
		config := ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: []string{""},
		}

		err := config.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CORS origin cannot be empty")
	})

	t.Run("GetCORSOrigins with defaults", func(t *testing.T) {
		config := ServerConfig{
			Host: "localhost",
			Port: 8080,
			// No CORSOrigins set
		}

		origins := config.GetCORSOrigins()
		expected := []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}

		assert.Equal(t, expected, origins)
	})

	t.Run("GetCORSOrigins with custom origins", func(t *testing.T) {
		customOrigins := []string{
			"https://example.com",
			"http://localhost:9000",
		}

		config := ServerConfig{
			Host:        "localhost",
			Port:        8080,
			CORSOrigins: customOrigins,
		}

		origins := config.GetCORSOrigins()
		assert.Equal(t, customOrigins, origins)
	})
}
