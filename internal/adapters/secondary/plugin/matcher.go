package plugin

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// RuleMatcher implements plugin matching based on rules.
type RuleMatcher struct {
	mu    sync.RWMutex
	rules map[string][]ports.MatchRule
}

// NewRuleMatcher creates a new rule-based matcher.
func NewRuleMatcher() *RuleMatcher {
	return &RuleMatcher{
		rules: make(map[string][]ports.MatchRule),
	}
}

// Match returns plugins that should process the given content.
func (m *RuleMatcher) Match(content string, language string, metadata map[string]interface{}) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type match struct {
		plugin   string
		priority int
	}

	var matches []match
	seen := make(map[string]bool)

	// Check each plugin's rules
	for pluginName, rules := range m.rules {
		for _, rule := range rules {
			if m.matchesRule(rule, content, language, metadata) {
				if !seen[pluginName] {
					matches = append(matches, match{
						plugin:   pluginName,
						priority: rule.Priority,
					})
					seen[pluginName] = true
				}
			}
		}
	}

	// Sort by priority (higher priority first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].priority > matches[j].priority
	})

	// Extract plugin names
	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.plugin
	}

	return result
}

// MatchByType returns plugins of a specific type that match the content.
func (m *RuleMatcher) MatchByType(content string, pluginType entities.PluginType) []string {
	// This would require integration with the registry to filter by type
	// For now, just use regular match
	return m.Match(content, "", nil)
}

// AddRule adds a matching rule.
func (m *RuleMatcher) AddRule(pluginName string, rule ports.MatchRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = m.generateRuleID(pluginName)
	}

	m.rules[pluginName] = append(m.rules[pluginName], rule)
}

// RemoveRule removes a matching rule.
func (m *RuleMatcher) RemoveRule(pluginName string, ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules, exists := m.rules[pluginName]
	if !exists {
		return
	}

	// Filter out the rule
	var newRules []ports.MatchRule
	for _, rule := range rules {
		if rule.ID != ruleID {
			newRules = append(newRules, rule)
		}
	}

	if len(newRules) == 0 {
		delete(m.rules, pluginName)
	} else {
		m.rules[pluginName] = newRules
	}
}

// matchesRule checks if content matches a rule.
func (m *RuleMatcher) matchesRule(rule ports.MatchRule, content string, language string, metadata map[string]interface{}) bool {
	// Check language
	if rule.Language != "" && rule.Language != language {
		return false
	}

	// Check file extension from metadata
	if rule.FileExt != "" {
		if fileExt, ok := metadata["file_ext"].(string); ok {
			if !strings.HasSuffix(fileExt, rule.FileExt) {
				return false
			}
		}
	}

	// Check content type from metadata
	if rule.ContentType != "" {
		if contentType, ok := metadata["content_type"].(string); ok {
			if contentType != rule.ContentType {
				return false
			}
		}
	}

	// Check pattern
	if rule.Pattern != "" {
		matched, err := regexp.MatchString(rule.Pattern, content)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// generateRuleID generates a unique rule ID.
func (m *RuleMatcher) generateRuleID(pluginName string) string {
	// Simple ID generation
	count := len(m.rules[pluginName])
	return pluginName + "-rule-" + string(rune(count+1))
}

// ConfigurableMatcher extends RuleMatcher with configuration-based rules.
type ConfigurableMatcher struct {
	*RuleMatcher
	registry ports.PluginRegistry
}

// NewConfigurableMatcher creates a new configurable matcher.
func NewConfigurableMatcher(registry ports.PluginRegistry) *ConfigurableMatcher {
	return &ConfigurableMatcher{
		RuleMatcher: NewRuleMatcher(),
		registry:    registry,
	}
}

// LoadFromConfig loads matching rules from plugin configurations.
func (c *ConfigurableMatcher) LoadFromConfig(configs map[string]entities.PluginConfig) {
	for pluginName, config := range configs {
		// Add rules based on file extensions
		for _, ext := range config.FileExtensions {
			c.AddRule(pluginName, ports.MatchRule{
				Priority: config.Priority,
				FileExt:  ext,
			})
		}

		// Add rules based on content patterns
		for _, pattern := range config.ContentPatterns {
			c.AddRule(pluginName, ports.MatchRule{
				Priority: config.Priority,
				Pattern:  pattern,
			})
		}
	}
}

// MatchWithConfig matches content using both rules and plugin configurations.
func (c *ConfigurableMatcher) MatchWithConfig(content string, language string, fileExt string) []string {
	metadata := map[string]interface{}{
		"language": language,
		"file_ext": fileExt,
	}

	// First try rule-based matching
	matches := c.Match(content, language, metadata)
	if len(matches) > 0 {
		return matches
	}

	// Fallback to checking all processor plugins
	if c.registry != nil {
		processors := c.registry.GetByType(entities.PluginTypeProcessor)
		for _, p := range processors {
			matches = append(matches, p.Name())
		}
	}

	return matches
}
