package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// ConfigService implements the configuration service business logic
type ConfigService struct {
	loader ports.ConfigLoader
	merger ports.ConfigMerger
}

// NewConfigService creates a new configuration service
func NewConfigService(loader ports.ConfigLoader, merger ports.ConfigMerger) *ConfigService {
	return &ConfigService{
		loader: loader,
		merger: merger,
	}
}

// LoadConfig loads the complete configuration with hierarchy and overrides
func (s *ConfigService) LoadConfig(ctx context.Context, workingDir string, flags map[string]interface{}) (*entities.Config, error) {
	// Start with defaults
	defaultConfig := s.GetDefaultConfig()

	// Load global config (creates if not exists)
	globalConfig, err := s.loader.LoadGlobal(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}

	// Load local config (optional)
	localConfig, err := s.loader.LoadLocal(ctx, workingDir)
	if err != nil {
		return nil, fmt.Errorf("loading local config: %w", err)
	}

	// Merge configurations in order of precedence: defaults → global → local
	var configs []*entities.Config
	configs = append(configs, defaultConfig)
	if globalConfig != nil {
		configs = append(configs, globalConfig)
	}
	if localConfig != nil {
		configs = append(configs, localConfig)
	}

	mergedConfig := s.merger.Merge(configs...)

	// Apply environment variable overrides
	envConfig := s.merger.ApplyEnvVars(mergedConfig)

	// Apply CLI flag overrides (highest precedence)
	finalConfig := s.merger.ApplyFlags(envConfig, flags)

	// Final validation
	if err := s.ValidateConfig(finalConfig); err != nil {
		return nil, fmt.Errorf("final config validation: %w", err)
	}

	return finalConfig, nil
}

// GetDefaultConfig returns the default configuration
func (s *ConfigService) GetDefaultConfig() *entities.Config {
	// We could import the defaults package, but to avoid circular imports,
	// we'll delegate to the merger which should have access to defaults
	return s.merger.Merge() // Merge with no arguments returns defaults
}

// ValidateConfig validates a configuration
func (s *ConfigService) ValidateConfig(config *entities.Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	return config.Validate()
}

// CreateGlobalConfig creates the global configuration file with defaults
func (s *ConfigService) CreateGlobalConfig(ctx context.Context) error {
	globalPath := s.loader.GetGlobalPath()
	return s.loader.CreateDefaults(ctx, globalPath)
}

// Ensure ConfigService implements ports.ConfigService
var _ ports.ConfigService = (*ConfigService)(nil)
