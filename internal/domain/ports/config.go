package ports

import (
	"context"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// ConfigLoader defines the interface for loading configuration files
type ConfigLoader interface {
	// LoadGlobal loads the global configuration file
	LoadGlobal(ctx context.Context) (*entities.Config, error)

	// LoadLocal loads a local configuration file from the specified directory
	LoadLocal(ctx context.Context, dir string) (*entities.Config, error)

	// CreateDefaults creates a default configuration file at the specified path
	CreateDefaults(ctx context.Context, path string) error

	// GetGlobalPath returns the path to the global configuration file
	GetGlobalPath() string

	// GetLocalPath returns the path to the local configuration file for a directory
	GetLocalPath(dir string) string
}

// ConfigMerger defines the interface for merging configurations
type ConfigMerger interface {
	// Merge merges multiple configurations with later configs taking precedence
	Merge(configs ...*entities.Config) *entities.Config

	// ApplyFlags applies CLI flag overrides to a configuration
	ApplyFlags(config *entities.Config, flags map[string]interface{}) *entities.Config

	// ApplyEnvVars applies environment variable overrides to a configuration
	ApplyEnvVars(config *entities.Config) *entities.Config
}

// ConfigService defines the interface for the configuration service
type ConfigService interface {
	// LoadConfig loads the complete configuration with hierarchy and overrides
	LoadConfig(ctx context.Context, workingDir string, flags map[string]interface{}) (*entities.Config, error)

	// GetDefaultConfig returns the default configuration
	GetDefaultConfig() *entities.Config

	// ValidateConfig validates a configuration
	ValidateConfig(config *entities.Config) error

	// CreateGlobalConfig creates the global configuration file with defaults
	CreateGlobalConfig(ctx context.Context) error
}
