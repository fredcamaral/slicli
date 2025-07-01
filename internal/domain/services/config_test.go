package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// Mock implementations for testing

type MockConfigLoader struct {
	mock.Mock
}

func (m *MockConfigLoader) LoadGlobal(ctx context.Context) (*entities.Config, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Config), args.Error(1)
}

func (m *MockConfigLoader) LoadLocal(ctx context.Context, dir string) (*entities.Config, error) {
	args := m.Called(ctx, dir)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Config), args.Error(1)
}

func (m *MockConfigLoader) CreateDefaults(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockConfigLoader) GetGlobalPath() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConfigLoader) GetLocalPath(dir string) string {
	args := m.Called(dir)
	return args.String(0)
}

type MockConfigMerger struct {
	mock.Mock
}

func (m *MockConfigMerger) Merge(configs ...*entities.Config) *entities.Config {
	args := m.Called(configs)
	return args.Get(0).(*entities.Config)
}

func (m *MockConfigMerger) ApplyFlags(config *entities.Config, flags map[string]interface{}) *entities.Config {
	args := m.Called(config, flags)
	return args.Get(0).(*entities.Config)
}

func (m *MockConfigMerger) ApplyEnvVars(config *entities.Config) *entities.Config {
	args := m.Called(config)
	return args.Get(0).(*entities.Config)
}

func TestNewConfigService(t *testing.T) {
	t.Run("creates service with dependencies", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		service := NewConfigService(loader, merger)

		assert.NotNil(t, service)
		assert.Equal(t, loader, service.loader)
		assert.Equal(t, merger, service.merger)
	})
}

func TestConfigService_LoadConfig(t *testing.T) {
	t.Run("loads and merges config hierarchy successfully", func(t *testing.T) {
		// Setup mocks
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		// Create test configs
		defaultConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 3000},
			Theme:   entities.ThemeConfig{Name: "default"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		globalConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 4000},
			Theme:   entities.ThemeConfig{Name: "global-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		localConfig := &entities.Config{
			Server:  entities.ServerConfig{Port: 5000},
			Theme:   entities.ThemeConfig{Name: "local-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		mergedConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 5000},
			Theme:   entities.ThemeConfig{Name: "local-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		envConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "127.0.0.1", Port: 5000},
			Theme:   entities.ThemeConfig{Name: "local-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		finalConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "127.0.0.1", Port: 6000},
			Theme:   entities.ThemeConfig{Name: "local-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		flags := map[string]interface{}{
			"host": "127.0.0.1",
			"port": 6000,
		}

		// Setup expectations
		merger.On("Merge", mock.Anything).Return(defaultConfig).Once()
		loader.On("LoadGlobal", mock.Anything).Return(globalConfig, nil)
		loader.On("LoadLocal", mock.Anything, "/test/dir").Return(localConfig, nil)
		merger.On("Merge", mock.MatchedBy(func(configs []*entities.Config) bool {
			return len(configs) == 3
		})).Return(mergedConfig)
		merger.On("ApplyEnvVars", mergedConfig).Return(envConfig)
		merger.On("ApplyFlags", envConfig, flags).Return(finalConfig)

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		result, err := service.LoadConfig(ctx, "/test/dir", flags)

		require.NoError(t, err)
		assert.Equal(t, finalConfig, result)
		loader.AssertExpectations(t)
		merger.AssertExpectations(t)
	})

	t.Run("handles global config load error", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		loader.On("LoadGlobal", mock.Anything).Return(nil, errors.New("global config error"))
		merger.On("Merge", mock.Anything).Return(&entities.Config{})

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		_, err := service.LoadConfig(ctx, "/test/dir", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading global config")
	})

	t.Run("handles local config load error", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		loader.On("LoadGlobal", mock.Anything).Return(&entities.Config{}, nil)
		loader.On("LoadLocal", mock.Anything, "/test/dir").Return(nil, errors.New("local config error"))
		merger.On("Merge", mock.Anything).Return(&entities.Config{})

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		_, err := service.LoadConfig(ctx, "/test/dir", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "loading local config")
	})

	t.Run("handles validation error", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		invalidConfig := &entities.Config{
			Server: entities.ServerConfig{Port: -1}, // Invalid port
		}

		loader.On("LoadGlobal", mock.Anything).Return(&entities.Config{}, nil)
		loader.On("LoadLocal", mock.Anything, "/test/dir").Return(nil, nil)
		merger.On("Merge", mock.Anything).Return(&entities.Config{})
		merger.On("ApplyEnvVars", mock.Anything).Return(&entities.Config{})
		merger.On("ApplyFlags", mock.Anything, mock.Anything).Return(invalidConfig)

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		_, err := service.LoadConfig(ctx, "/test/dir", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "final config validation")
	})

	t.Run("handles nil local config", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		defaultConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 3000},
			Theme:   entities.ThemeConfig{Name: "default"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		globalConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 4000},
			Theme:   entities.ThemeConfig{Name: "global-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		finalConfig := &entities.Config{
			Server:  entities.ServerConfig{Host: "localhost", Port: 4000},
			Theme:   entities.ThemeConfig{Name: "global-theme"},
			Browser: entities.BrowserConfig{AutoOpen: true, Browser: "default"},
			Watcher: entities.WatcherConfig{IntervalMs: 200},
			Plugins: entities.PluginsConfig{Enabled: true},
		}

		// Setup expectations - local config returns nil (not found)
		merger.On("Merge", mock.Anything).Return(defaultConfig).Once()
		loader.On("LoadGlobal", mock.Anything).Return(globalConfig, nil)
		loader.On("LoadLocal", mock.Anything, "/test/dir").Return(nil, nil)
		merger.On("Merge", mock.MatchedBy(func(configs []*entities.Config) bool {
			return len(configs) == 2 // Only default and global
		})).Return(finalConfig)
		merger.On("ApplyEnvVars", finalConfig).Return(finalConfig)
		merger.On("ApplyFlags", finalConfig, map[string]interface{}(nil)).Return(finalConfig)

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		result, err := service.LoadConfig(ctx, "/test/dir", nil)

		require.NoError(t, err)
		assert.Equal(t, finalConfig, result)
	})
}

func TestConfigService_GetDefaultConfig(t *testing.T) {
	t.Run("returns default config from merger", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		expectedConfig := &entities.Config{
			Server: entities.ServerConfig{Host: "localhost", Port: 3000},
		}

		merger.On("Merge", mock.Anything).Return(expectedConfig)

		service := NewConfigService(loader, merger)
		result := service.GetDefaultConfig()

		assert.Equal(t, expectedConfig, result)
		merger.AssertExpectations(t)
	})
}

func TestConfigService_ValidateConfig(t *testing.T) {
	t.Run("validates valid config", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		validConfig := &entities.Config{
			Server: entities.ServerConfig{
				Host: "localhost",
				Port: 3000,
			},
			Theme: entities.ThemeConfig{
				Name: "default",
			},
			Browser: entities.BrowserConfig{
				AutoOpen: true,
				Browser:  "default",
			},
			Watcher: entities.WatcherConfig{
				IntervalMs: 200,
			},
			Plugins: entities.PluginsConfig{
				Enabled: true,
			},
		}

		service := NewConfigService(loader, merger)
		err := service.ValidateConfig(validConfig)

		assert.NoError(t, err)
	})

	t.Run("rejects nil config", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		service := NewConfigService(loader, merger)
		err := service.ValidateConfig(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("rejects invalid config", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		invalidConfig := &entities.Config{
			Server: entities.ServerConfig{
				Port: -1, // Invalid port
			},
		}

		service := NewConfigService(loader, merger)
		err := service.ValidateConfig(invalidConfig)

		assert.Error(t, err)
	})
}

func TestConfigService_CreateGlobalConfig(t *testing.T) {
	t.Run("creates global config successfully", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		globalPath := "/home/user/.config/slicli/config.toml"

		loader.On("GetGlobalPath").Return(globalPath)
		loader.On("CreateDefaults", mock.Anything, globalPath).Return(nil)

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		err := service.CreateGlobalConfig(ctx)

		assert.NoError(t, err)
		loader.AssertExpectations(t)
	})

	t.Run("handles creation error", func(t *testing.T) {
		loader := &MockConfigLoader{}
		merger := &MockConfigMerger{}

		globalPath := "/invalid/path/config.toml"
		creationError := errors.New("permission denied")

		loader.On("GetGlobalPath").Return(globalPath)
		loader.On("CreateDefaults", mock.Anything, globalPath).Return(creationError)

		service := NewConfigService(loader, merger)
		ctx := context.Background()

		err := service.CreateGlobalConfig(ctx)

		assert.Error(t, err)
		assert.Equal(t, creationError, err)
	})
}
