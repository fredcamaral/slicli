package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// TOMLLoader implements the ConfigLoader interface using TOML files
type TOMLLoader struct {
	globalPath string
	localName  string
}

// NewTOMLLoader creates a new TOML configuration loader
func NewTOMLLoader() *TOMLLoader {
	homeDir, _ := os.UserHomeDir()
	globalPath := filepath.Join(homeDir, ".config", "slicli", "config.toml")

	return &TOMLLoader{
		globalPath: globalPath,
		localName:  "slicli.toml",
	}
}

// LoadGlobal loads the global configuration file
func (l *TOMLLoader) LoadGlobal(ctx context.Context) (*entities.Config, error) {
	// Check if global config exists
	if _, err := os.Stat(l.globalPath); os.IsNotExist(err) {
		// Create default config on first run
		if err := l.CreateDefaults(ctx, l.globalPath); err != nil {
			return nil, fmt.Errorf("creating defaults: %w", err)
		}
	}

	return l.loadConfig(l.globalPath)
}

// LoadLocal loads a local configuration file from the specified directory
func (l *TOMLLoader) LoadLocal(ctx context.Context, dir string) (*entities.Config, error) {
	localPath := filepath.Join(dir, l.localName)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return nil, nil // Local config is optional
	}

	return l.loadConfig(localPath)
}

// CreateDefaults creates a default configuration file at the specified path
func (l *TOMLLoader) CreateDefaults(ctx context.Context, path string) error {
	// Ensure directory exists
	if err := l.ensureConfigDir(path); err != nil {
		return err
	}

	// Write default config
	defaults := GetDefaultConfig()

	file, err := os.Create(path) // #nosec G304 - path is controlled (global config path)
	if err != nil {
		return fmt.Errorf("creating config file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	encoder := toml.NewEncoder(file)
	encoder.Indent = "  "

	if err := encoder.Encode(defaults); err != nil {
		return fmt.Errorf("encoding config to %s: %w", path, err)
	}

	return nil
}

// GetGlobalPath returns the path to the global configuration file
func (l *TOMLLoader) GetGlobalPath() string {
	return l.globalPath
}

// GetLocalPath returns the path to the local configuration file for a directory
func (l *TOMLLoader) GetLocalPath(dir string) string {
	return filepath.Join(dir, l.localName)
}

// loadConfig loads and validates a configuration file
func (l *TOMLLoader) loadConfig(path string) (*entities.Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 - path is from controlled sources (global/local config)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var config entities.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing TOML from %s: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %s: %w", path, err)
	}

	return &config, nil
}

// ensureConfigDir ensures the configuration directory exists
func (l *TOMLLoader) ensureConfigDir(path string) error {
	dir := filepath.Dir(path)

	// Create config directory with restricted permissions (0750 = owner and group only)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	return nil
}

// Ensure TOMLLoader implements ports.ConfigLoader
var _ ports.ConfigLoader = (*TOMLLoader)(nil)
