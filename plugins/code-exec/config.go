package main

import (
	"fmt"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// extractConfig extracts and validates execution configuration from plugin options
func (p *CodeExecPlugin) extractConfig(options map[string]interface{}) entities.ExecutionConfig {
	config := entities.GetDefaultExecutionConfig()

	// Extract language from options
	if lang, ok := options["language"].(string); ok {
		config.Language = lang
	}

	// Extract timeout
	if timeout, ok := options["timeout"].(string); ok {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = duration
		}
	} else if timeoutSec, ok := options["timeout"].(float64); ok {
		config.Timeout = time.Duration(timeoutSec) * time.Second
	}

	// Extract max output size
	if maxOutput, ok := options["max_output"].(float64); ok {
		config.MaxOutputSize = int(maxOutput)
	} else if maxOutputStr, ok := options["max_output"].(string); ok {
		if size, err := parseSize(maxOutputStr); err == nil {
			config.MaxOutputSize = size
		}
	}

	// Extract max memory
	if maxMem, ok := options["max_memory"].(float64); ok {
		config.MaxMemory = int64(maxMem)
	} else if maxMemStr, ok := options["max_memory"].(string); ok {
		if size, err := parseSize(maxMemStr); err == nil {
			config.MaxMemory = int64(size)
		}
	}

	// Extract trust settings
	if trusted, ok := options["trusted"].(bool); ok {
		config.TrustedMode = trusted
	}

	// Extract network permission
	if allowNet, ok := options["allow_network"].(bool); ok {
		config.AllowNetwork = allowNet
	}

	// Extract file write permission
	if allowWrite, ok := options["allow_file_write"].(bool); ok {
		config.AllowFileWrite = allowWrite
	}

	// Extract environment variables
	if env, ok := options["environment"].([]interface{}); ok {
		for _, e := range env {
			if envStr, ok := e.(string); ok {
				config.Environment = append(config.Environment, envStr)
			}
		}
	} else if envMap, ok := options["environment"].(map[string]interface{}); ok {
		for key, value := range envMap {
			if valueStr, ok := value.(string); ok {
				config.Environment = append(config.Environment, fmt.Sprintf("%s=%s", key, valueStr))
			}
		}
	}

	// Apply global plugin configuration overrides
	p.applyGlobalConfig(&config)

	return config
}

// applyGlobalConfig applies global plugin configuration
func (p *CodeExecPlugin) applyGlobalConfig(config *entities.ExecutionConfig) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Apply global timeout override
	if globalTimeout, ok := p.config["global_timeout"].(string); ok {
		if duration, err := time.ParseDuration(globalTimeout); err == nil {
			config.Timeout = duration
		}
	}

	// Apply global trusted mode override
	if globalTrusted, ok := p.config["global_trusted"].(bool); ok {
		config.TrustedMode = globalTrusted
	}

	// Apply global memory limit
	if globalMemLimit, ok := p.config["global_memory_limit"].(string); ok {
		if size, err := parseSize(globalMemLimit); err == nil {
			config.MaxMemory = int64(size)
		}
	}

	// Apply global output limit
	if globalOutputLimit, ok := p.config["global_output_limit"].(string); ok {
		if size, err := parseSize(globalOutputLimit); err == nil {
			config.MaxOutputSize = size
		}
	}

	// Disable execution entirely if configured
	if disabled, ok := p.config["execution_disabled"].(bool); ok && disabled {
		config.TrustedMode = false
	}
}

// parseSize parses size strings like "100MB", "1GB", "512KB"
func parseSize(sizeStr string) (int, error) {
	// Simple size parser - in production you might want to use a more robust one
	var size float64
	var unit string

	n, err := fmt.Sscanf(sizeStr, "%f%s", &size, &unit)
	if err != nil || n != 2 {
		// Try without unit (assume bytes)
		if n, err := fmt.Sscanf(sizeStr, "%f", &size); err == nil && n == 1 {
			return int(size), nil
		}
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	switch unit {
	case "B", "b", "bytes":
		return int(size), nil
	case "KB", "kb", "K", "k":
		return int(size * 1024), nil
	case "MB", "mb", "M", "m":
		return int(size * 1024 * 1024), nil
	case "GB", "gb", "G", "g":
		return int(size * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}
