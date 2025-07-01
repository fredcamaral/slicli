package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

func TestServeCommand(t *testing.T) {
	t.Run("valid arguments", func(t *testing.T) {
		// Test validation logic only - don't actually start server
		if err := validateServeArgs([]string{"test.md"}); err != nil {
			t.Errorf("Expected no error for valid args, got: %v", err)
		}
	})

	t.Run("missing file argument", func(t *testing.T) {
		cmd := &cobra.Command{Use: serveCmd.Use, Args: serveCmd.Args, RunE: serveCmd.RunE}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s)")
	})

	t.Run("multiple arguments", func(t *testing.T) {
		err := validateServeArgs([]string{"test1.md", "test2.md"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s)")
	})

	t.Run("empty arguments", func(t *testing.T) {
		err := validateServeArgs([]string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "accepts 1 arg(s)")
	})

	t.Run("with custom flags", func(t *testing.T) {
		// Test validation only - don't actually start server
		err := validateServeArgs([]string{"test.md"})
		require.NoError(t, err)
	})
}

func TestValidateServeConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 3000,
			},
		}
		err := validateServeConfig(config)
		require.NoError(t, err)
	})

	t.Run("invalid port - zero", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 0,
			},
		}
		err := validateServeConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("invalid port - too high", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 99999,
			},
		}
		err := validateServeConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("invalid host", func(t *testing.T) {
		config := &entities.Config{
			Server: entities.ServerConfig{
				Host: "invalid host!",
				Port: 3000,
			},
		}
		err := validateServeConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid host")
	})
}

func TestGetServerURL(t *testing.T) {
	tests := []struct {
		name     string
		hostVal  string
		portVal  int
		expected string
	}{
		{
			name:     "default values",
			hostVal:  "localhost",
			portVal:  3000,
			expected: "http://localhost:3000",
		},
		{
			name:     "custom host and port",
			hostVal:  "127.0.0.1",
			portVal:  8080,
			expected: "http://127.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldHost := host
			oldPort := port

			host = tt.hostVal
			port = tt.portVal

			result := getServerURL()
			assert.Equal(t, tt.expected, result)

			host = oldHost
			port = oldPort
		})
	}
}
