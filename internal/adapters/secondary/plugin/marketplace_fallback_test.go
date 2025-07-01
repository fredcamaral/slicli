package plugin

import (
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceClient_FallbackToBuiltinPlugins(t *testing.T) {
	t.Run("ListPlugins fallback when server unavailable", func(t *testing.T) {
		// Create client with invalid URL (server doesn't exist)
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second, // Short timeout for quick test
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Try to list plugins - should fallback to built-in plugins
		plugins, err := client.ListPlugins("", false)

		require.NoError(t, err)
		assert.Len(t, plugins, 3, "Should return 3 built-in plugins")

		// Verify we got the built-in plugins
		pluginIDs := make([]string, len(plugins))
		for i, plugin := range plugins {
			pluginIDs[i] = plugin.ID
		}

		assert.Contains(t, pluginIDs, "syntax-highlight")
		assert.Contains(t, pluginIDs, "mermaid")
		assert.Contains(t, pluginIDs, "code-exec")
	})

	t.Run("GetPlugin fallback when server unavailable", func(t *testing.T) {
		// Create client with invalid URL
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second,
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Try to get specific plugin - should fallback to built-in
		plugin, err := client.GetPlugin("syntax-highlight")

		require.NoError(t, err)
		assert.Equal(t, "syntax-highlight", plugin.ID)
		assert.Equal(t, "Syntax Highlighter", plugin.Name)
		assert.Equal(t, "SliCLI Team", plugin.Author)
		assert.Equal(t, "free", string(plugin.Price.Type))
		assert.True(t, plugin.Featured)
	})

	t.Run("GetPlugin returns error for non-existent plugin", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second,
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Try to get non-existent plugin
		_, err := client.GetPlugin("non-existent-plugin")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in built-in plugins")
	})

	t.Run("Filter by category works with built-in plugins", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second,
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Filter by processor category
		plugins, err := client.ListPlugins(entities.PluginTypeProcessor, false)

		require.NoError(t, err)
		assert.Len(t, plugins, 3, "All built-in plugins are processor type")

		// Verify all returned plugins are processor type
		for _, plugin := range plugins {
			assert.Equal(t, entities.PluginTypeProcessor, plugin.Category)
		}
	})

	t.Run("Premium filter works with built-in plugins", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second,
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Filter by premium (should return empty as all built-ins are free)
		plugins, err := client.ListPlugins("", true)

		require.NoError(t, err)
		assert.Empty(t, plugins, "No built-in plugins are premium")
	})

	t.Run("GetFeaturedPlugins works with built-in plugins", func(t *testing.T) {
		config := MarketplaceConfig{
			BaseURL:  "https://invalid-marketplace-server.example.com",
			Timeout:  1 * time.Second,
			CacheTTL: 1 * time.Minute,
		}
		client := NewMarketplaceClient(config)

		// Get featured plugins
		plugins, err := client.GetFeaturedPlugins()

		require.NoError(t, err)
		assert.Len(t, plugins, 2, "Should return 2 featured built-in plugins")

		// Verify all returned plugins are featured
		for _, plugin := range plugins {
			assert.True(t, plugin.Featured)
		}

		// Should be sorted by rating (descending)
		assert.True(t, plugins[0].Rating >= plugins[1].Rating)
	})
}

func TestBuiltinPluginsData(t *testing.T) {
	config := MarketplaceConfig{
		BaseURL:  "https://invalid-marketplace-server.example.com",
		Timeout:  1 * time.Second,
		CacheTTL: 1 * time.Minute,
	}
	client := NewMarketplaceClient(config)

	t.Run("built-in plugins have complete data", func(t *testing.T) {
		plugins, err := client.ListPlugins("", false)
		require.NoError(t, err)

		for _, plugin := range plugins {
			// Verify required fields are populated
			assert.NotEmpty(t, plugin.ID, "Plugin ID should not be empty")
			assert.NotEmpty(t, plugin.Name, "Plugin name should not be empty")
			assert.NotEmpty(t, plugin.Version, "Plugin version should not be empty")
			assert.NotEmpty(t, plugin.Author, "Plugin author should not be empty")
			assert.NotEmpty(t, plugin.Description, "Plugin description should not be empty")
			assert.NotEmpty(t, plugin.License, "Plugin license should not be empty")
			assert.NotEmpty(t, plugin.Tags, "Plugin should have tags")
			assert.NotEmpty(t, plugin.Platforms, "Plugin should support platforms")

			// Verify pricing is free
			assert.Equal(t, PriceTypeFree, plugin.Price.Type)
			assert.Equal(t, 0.00, plugin.Price.Amount)
			assert.Equal(t, "USD", plugin.Price.Currency)

			// Verify reasonable rating and download counts
			assert.GreaterOrEqual(t, plugin.Rating, 4.0, "Plugin rating should be >= 4.0")
			assert.Greater(t, plugin.Downloads, int64(0), "Plugin should have downloads")

			// Verify timestamps are set
			assert.False(t, plugin.CreatedAt.IsZero(), "CreatedAt should be set")
			assert.False(t, plugin.UpdatedAt.IsZero(), "UpdatedAt should be set")
		}
	})

	t.Run("built-in plugins have valid metadata", func(t *testing.T) {
		plugin, err := client.GetPlugin("syntax-highlight")
		require.NoError(t, err)

		assert.Equal(t, entities.PluginTypeProcessor, plugin.Category)
		assert.Contains(t, plugin.Tags, "syntax")
		assert.Contains(t, plugin.Tags, "built-in")
		assert.Contains(t, plugin.Platforms, "linux-amd64")
		assert.Contains(t, plugin.Platforms, "darwin-amd64")
		assert.Contains(t, plugin.Platforms, "windows-amd64")
		assert.Equal(t, "approved", string(plugin.Status))
	})
}
