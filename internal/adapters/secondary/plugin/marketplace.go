package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// MarketplacePlugin represents a plugin available in the marketplace
type MarketplacePlugin struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Version      string                  `json:"version"`
	Author       string                  `json:"author"`
	Description  string                  `json:"description"`
	Category     entities.PluginType     `json:"category"`
	Tags         []string                `json:"tags"`
	Price        MarketplacePrice        `json:"price"`
	Rating       float64                 `json:"rating"`
	Downloads    int64                   `json:"downloads"`
	Repository   string                  `json:"repository"`
	Homepage     string                  `json:"homepage"`
	License      string                  `json:"license"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
	Dependencies []PluginDependency      `json:"dependencies"`
	Platforms    []string                `json:"platforms"`
	Status       MarketplacePluginStatus `json:"status"`
	Featured     bool                    `json:"featured"`
	Premium      bool                    `json:"premium"`
}

// MarketplacePrice represents pricing information (always free for open source)
type MarketplacePrice struct {
	Type     PriceType `json:"type"`     // always free
	Amount   float64   `json:"amount"`   // always 0.00
	Currency string    `json:"currency"` // USD
	Interval string    `json:"interval"` // not applicable
}

// PriceType represents different pricing models
type PriceType string

const (
	PriceTypeFree PriceType = "free" // Only free pricing for open source
)

// MarketplacePluginStatus represents plugin status in marketplace
type MarketplacePluginStatus string

const (
	StatusPending    MarketplacePluginStatus = "pending"
	StatusApproved   MarketplacePluginStatus = "approved"
	StatusRejected   MarketplacePluginStatus = "rejected"
	StatusDeprecated MarketplacePluginStatus = "deprecated"
)

// PluginDependency represents a plugin dependency
type PluginDependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"` // marketplace, local, url
}

// MarketplaceClient handles interaction with plugin marketplace
type MarketplaceClient struct {
	mu           sync.RWMutex
	baseURL      string
	apiKey       string
	httpClient   *http.Client
	cache        map[string]*MarketplacePlugin
	cacheExpiry  time.Time
	userLicenses map[string][]string // user_id -> list of licensed plugin IDs
}

// MarketplaceConfig configures the marketplace client
type MarketplaceConfig struct {
	BaseURL  string
	APIKey   string
	Timeout  time.Duration
	CacheTTL time.Duration
	UserID   string
}

// NewMarketplaceClient creates a new marketplace client
func NewMarketplaceClient(config MarketplaceConfig) *MarketplaceClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 10 * time.Minute
	}

	return &MarketplaceClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache:        make(map[string]*MarketplacePlugin),
		userLicenses: make(map[string][]string),
	}
}

// ListPlugins retrieves available plugins from marketplace
func (mc *MarketplaceClient) ListPlugins(category entities.PluginType, premium bool) ([]*MarketplacePlugin, error) {
	// Check cache first
	if mc.isCacheValid() {
		return mc.filterFromCache(category, premium), nil
	}

	// Try to fetch from marketplace API
	url := mc.baseURL + "/api/v1/plugins"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		// Fallback to built-in plugins if request creation fails
		return mc.getBuiltinPlugins(category, premium), nil
	}

	// Add query parameters
	q := req.URL.Query()
	if category != "" {
		q.Add("category", string(category))
	}
	if premium {
		q.Add("premium", "true")
	}
	req.URL.RawQuery = q.Encode()

	// Add authentication
	if mc.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+mc.apiKey)
	}
	req.Header.Set("User-Agent", "slicli/1.0")

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		// Fallback to built-in plugins if marketplace is unavailable
		return mc.getBuiltinPlugins(category, premium), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Fallback to built-in plugins if marketplace returns error
		return mc.getBuiltinPlugins(category, premium), nil
	}

	var plugins []*MarketplacePlugin
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		// Fallback to built-in plugins if response parsing fails
		return mc.getBuiltinPlugins(category, premium), nil
	}

	// Update cache
	mc.updateCache(plugins)

	return plugins, nil
}

// SearchPlugins searches for plugins by name or tags
func (mc *MarketplaceClient) SearchPlugins(query string) ([]*MarketplacePlugin, error) {
	plugins, err := mc.ListPlugins("", false)
	if err != nil {
		return nil, err
	}

	var results []*MarketplacePlugin
	query = strings.ToLower(query)

	for _, plugin := range plugins {
		// Search in name, description, and tags
		if strings.Contains(strings.ToLower(plugin.Name), query) ||
			strings.Contains(strings.ToLower(plugin.Description), query) ||
			mc.containsTag(plugin.Tags, query) {
			results = append(results, plugin)
		}
	}

	// Sort by relevance (exact name matches first, then by rating)
	sort.Slice(results, func(i, j int) bool {
		iExact := strings.ToLower(results[i].Name) == query
		jExact := strings.ToLower(results[j].Name) == query

		if iExact && !jExact {
			return true
		}
		if !iExact && jExact {
			return false
		}

		// Both exact or both partial - sort by rating
		return results[i].Rating > results[j].Rating
	})

	return results, nil
}

// GetPlugin retrieves a specific plugin by ID
func (mc *MarketplaceClient) GetPlugin(pluginID string) (*MarketplacePlugin, error) {
	mc.mu.RLock()
	if plugin, exists := mc.cache[pluginID]; exists && mc.isCacheValid() {
		mc.mu.RUnlock()
		return plugin, nil
	}
	mc.mu.RUnlock()

	url := fmt.Sprintf("%s/api/v1/plugins/%s", mc.baseURL, pluginID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		// Fallback to built-in plugins if request creation fails
		return mc.getBuiltinPlugin(pluginID)
	}

	if mc.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+mc.apiKey)
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		// Fallback to built-in plugins if marketplace is unavailable
		return mc.getBuiltinPlugin(pluginID)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// Try built-in plugins before returning error
		return mc.getBuiltinPlugin(pluginID)
	}
	if resp.StatusCode != http.StatusOK {
		// Fallback to built-in plugins if marketplace returns error
		return mc.getBuiltinPlugin(pluginID)
	}

	var plugin MarketplacePlugin
	if err := json.NewDecoder(resp.Body).Decode(&plugin); err != nil {
		// Fallback to built-in plugins if response parsing fails
		return mc.getBuiltinPlugin(pluginID)
	}

	// Update cache
	mc.mu.Lock()
	mc.cache[pluginID] = &plugin
	mc.mu.Unlock()

	return &plugin, nil
}

// DownloadPlugin downloads a plugin binary (always free for open source)
func (mc *MarketplaceClient) DownloadPlugin(pluginID, version, platform string) ([]byte, error) {
	// All plugins are free in open source model
	_, err := mc.GetPlugin(pluginID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/plugins/%s/download", mc.baseURL, pluginID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	if version != "" {
		q.Add("version", version)
	}
	if platform != "" {
		q.Add("platform", platform)
	}
	req.URL.RawQuery = q.Encode()

	if mc.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+mc.apiKey)
	}

	resp, err := mc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Note: Purchase functionality removed - all plugins are free in open source model

// GetFeaturedPlugins returns featured plugins
func (mc *MarketplaceClient) GetFeaturedPlugins() ([]*MarketplacePlugin, error) {
	plugins, err := mc.ListPlugins("", false)
	if err != nil {
		return nil, err
	}

	var featured []*MarketplacePlugin
	for _, plugin := range plugins {
		if plugin.Featured {
			featured = append(featured, plugin)
		}
	}

	// Sort by rating
	sort.Slice(featured, func(i, j int) bool {
		return featured[i].Rating > featured[j].Rating
	})

	return featured, nil
}

// Note: License functionality removed - all plugins are free and available to everyone

// Helper methods

func (mc *MarketplaceClient) isCacheValid() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return time.Now().Before(mc.cacheExpiry)
}

func (mc *MarketplaceClient) updateCache(plugins []*MarketplacePlugin) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Clear and repopulate cache
	mc.cache = make(map[string]*MarketplacePlugin)
	for _, plugin := range plugins {
		mc.cache[plugin.ID] = plugin
	}
	mc.cacheExpiry = time.Now().Add(10 * time.Minute)
}

func (mc *MarketplaceClient) filterFromCache(category entities.PluginType, premium bool) []*MarketplacePlugin {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var results []*MarketplacePlugin
	for _, plugin := range mc.cache {
		if category != "" && plugin.Category != category {
			continue
		}
		if premium && !plugin.Premium {
			continue
		}
		results = append(results, plugin)
	}
	return results
}

func (mc *MarketplaceClient) containsTag(tags []string, query string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

// getBuiltinPlugins returns built-in plugins when marketplace is unavailable
func (mc *MarketplaceClient) getBuiltinPlugins(category entities.PluginType, premium bool) []*MarketplacePlugin {
	plugins := []*MarketplacePlugin{
		{
			ID:          "syntax-highlight",
			Name:        "Syntax Highlighter",
			Version:     "1.0.0",
			Author:      "SliCLI Team",
			Description: "Provides syntax highlighting for code blocks with 200+ languages and multiple themes",
			Category:    entities.PluginTypeProcessor,
			Tags:        []string{"syntax", "highlighting", "code", "programming", "built-in"},
			Price: MarketplacePrice{
				Type:     PriceTypeFree,
				Amount:   0.00,
				Currency: "USD",
				Interval: "",
			},
			Rating:       4.8,
			Downloads:    15000,
			Repository:   "https://github.com/fredcamaral/slicli",
			Homepage:     "https://slicli.dev/plugins/syntax-highlight",
			License:      "MIT",
			CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC),
			Dependencies: []PluginDependency{},
			Platforms:    []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
			Status:       "approved",
			Featured:     true,
			Premium:      false,
		},
		{
			ID:          "mermaid",
			Name:        "Mermaid Diagrams",
			Version:     "1.0.0",
			Author:      "SliCLI Team",
			Description: "Generate beautiful diagrams from text using Mermaid syntax - flowcharts, sequence diagrams, and more",
			Category:    entities.PluginTypeProcessor,
			Tags:        []string{"diagrams", "mermaid", "visualization", "flowchart", "sequence", "built-in"},
			Price: MarketplacePrice{
				Type:     PriceTypeFree,
				Amount:   0.00,
				Currency: "USD",
				Interval: "",
			},
			Rating:       4.9,
			Downloads:    22000,
			Repository:   "https://github.com/fredcamaral/slicli",
			Homepage:     "https://slicli.dev/plugins/mermaid",
			License:      "MIT",
			CreatedAt:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC),
			Dependencies: []PluginDependency{},
			Platforms:    []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
			Status:       "approved",
			Featured:     true,
			Premium:      false,
		},
		{
			ID:          "code-exec",
			Name:        "Code Execution",
			Version:     "1.0.0",
			Author:      "SliCLI Team",
			Description: "Execute code blocks in presentations with safety sandboxing - supports bash, go, javascript, python",
			Category:    entities.PluginTypeProcessor,
			Tags:        []string{"code", "execution", "sandbox", "bash", "go", "javascript", "python", "built-in"},
			Price: MarketplacePrice{
				Type:     PriceTypeFree,
				Amount:   0.00,
				Currency: "USD",
				Interval: "",
			},
			Rating:       4.7,
			Downloads:    8500,
			Repository:   "https://github.com/fredcamaral/slicli",
			Homepage:     "https://slicli.dev/plugins/code-exec",
			License:      "MIT",
			CreatedAt:    time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC),
			Dependencies: []PluginDependency{},
			Platforms:    []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"},
			Status:       "approved",
			Featured:     false,
			Premium:      false,
		},
	}

	// Filter by category if specified
	if category != "" {
		filtered := make([]*MarketplacePlugin, 0)
		for _, plugin := range plugins {
			if plugin.Category == category {
				filtered = append(filtered, plugin)
			}
		}
		plugins = filtered
	}

	// Filter by premium status if specified
	if premium {
		filtered := make([]*MarketplacePlugin, 0)
		for _, plugin := range plugins {
			if plugin.Premium {
				filtered = append(filtered, plugin)
			}
		}
		plugins = filtered
	}

	// Update cache with built-in plugins
	mc.updateCache(plugins)

	return plugins
}

// getBuiltinPlugin returns a specific built-in plugin by ID
func (mc *MarketplaceClient) getBuiltinPlugin(pluginID string) (*MarketplacePlugin, error) {
	builtinPlugins := mc.getBuiltinPlugins("", false)

	for _, plugin := range builtinPlugins {
		if plugin.ID == pluginID {
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("plugin '%s' not found in built-in plugins", pluginID)
}

// License checking removed - all plugins are free and accessible
