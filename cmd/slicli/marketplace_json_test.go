package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/plugin"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintPluginsJSON(t *testing.T) {
	// Create test plugins
	testPlugins := []*plugin.MarketplacePlugin{
		{
			ID:          "syntax-highlight",
			Name:        "Syntax Highlighter",
			Version:     "1.0.0",
			Author:      "SliCLI Team",
			Description: "Provides syntax highlighting for code blocks",
			Category:    entities.PluginTypeProcessor,
			Tags:        []string{"syntax", "highlighting", "code"},
			Price: plugin.MarketplacePrice{
				Type:     "free",
				Amount:   0.00,
				Currency: "USD",
				Interval: "",
			},
			Rating:    4.5,
			Downloads: 12500,
			License:   "MIT",
			CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			Platforms: []string{"linux-amd64", "darwin-amd64", "windows-amd64"},
			Status:    "active",
			Featured:  true,
			Premium:   false,
		},
		{
			ID:          "mermaid",
			Name:        "Mermaid Diagrams",
			Version:     "2.1.0",
			Author:      "Community",
			Description: "Generate beautiful diagrams from text",
			Category:    entities.PluginTypeProcessor,
			Tags:        []string{"diagrams", "mermaid", "visualization"},
			Price: plugin.MarketplacePrice{
				Type:     "free",
				Amount:   0.00,
				Currency: "USD",
				Interval: "",
			},
			Rating:    4.8,
			Downloads: 25000,
			License:   "MIT",
			CreatedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Platforms: []string{"linux-amd64", "darwin-amd64"},
			Status:    "active",
			Featured:  false,
			Premium:   false,
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the function
	err := printPluginsJSON(testPlugins)
	require.NoError(t, err)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read the output
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	output := buf.String()

	// Parse the JSON output
	var result struct {
		Plugins []*plugin.MarketplacePlugin `json:"plugins"`
		Count   int                         `json:"count"`
		Format  string                      `json:"format"`
	}

	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify the structure
	assert.Equal(t, 2, result.Count)
	assert.Equal(t, "marketplace-v1", result.Format)
	assert.Len(t, result.Plugins, 2)

	// Verify first plugin data
	plugin1 := result.Plugins[0]
	assert.Equal(t, "syntax-highlight", plugin1.ID)
	assert.Equal(t, "Syntax Highlighter", plugin1.Name)
	assert.Equal(t, "1.0.0", plugin1.Version)
	assert.Equal(t, "SliCLI Team", plugin1.Author)
	assert.Equal(t, float64(4.5), plugin1.Rating)
	assert.Equal(t, int64(12500), plugin1.Downloads)
	assert.True(t, plugin1.Featured)
	assert.False(t, plugin1.Premium)
	assert.Contains(t, plugin1.Tags, "syntax")
	assert.Contains(t, plugin1.Platforms, "linux-amd64")

	// Verify second plugin data
	plugin2 := result.Plugins[1]
	assert.Equal(t, "mermaid", plugin2.ID)
	assert.Equal(t, "Mermaid Diagrams", plugin2.Name)
	assert.Equal(t, "2.1.0", plugin2.Version)
	assert.Equal(t, float64(4.8), plugin2.Rating)
	assert.Equal(t, int64(25000), plugin2.Downloads)
	assert.False(t, plugin2.Featured)
	assert.Contains(t, plugin2.Tags, "diagrams")

	// Verify pricing (should be free for all)
	assert.Equal(t, "free", string(plugin1.Price.Type))
	assert.Equal(t, 0.00, plugin1.Price.Amount)
	assert.Equal(t, "free", string(plugin2.Price.Type))
	assert.Equal(t, 0.00, plugin2.Price.Amount)
}

func TestPrintPluginsJSON_EmptyList(t *testing.T) {
	// Test with empty plugin list
	var emptyPlugins []*plugin.MarketplacePlugin

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the function
	err := printPluginsJSON(emptyPlugins)
	require.NoError(t, err)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read the output
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	output := buf.String()

	// Parse the JSON output
	var result struct {
		Plugins []*plugin.MarketplacePlugin `json:"plugins"`
		Count   int                         `json:"count"`
		Format  string                      `json:"format"`
	}

	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// Verify empty result
	assert.Equal(t, 0, result.Count)
	assert.Equal(t, "marketplace-v1", result.Format)
	assert.Empty(t, result.Plugins)
}

func TestPrintPluginsJSON_ValidJSONStructure(t *testing.T) {
	// Create a simple test plugin
	testPlugin := &plugin.MarketplacePlugin{
		ID:          "test-plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Author:      "Test Author",
		Description: "A test plugin for JSON output verification",
		Category:    entities.PluginTypeProcessor,
		Tags:        []string{"test"},
		Rating:      5.0,
		Downloads:   1000,
		License:     "MIT",
		Platforms:   []string{"linux-amd64"},
		Status:      "active",
		Featured:    false,
		Premium:     false,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the function
	err := printPluginsJSON([]*plugin.MarketplacePlugin{testPlugin})
	require.NoError(t, err)

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Read the output
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	output := buf.String()

	// Verify it's valid JSON
	var jsonData interface{}
	err = json.Unmarshal([]byte(output), &jsonData)
	require.NoError(t, err, "Output should be valid JSON")

	// Verify the JSON is properly formatted (indented)
	assert.Contains(t, output, "  \"plugins\":")
	assert.Contains(t, output, "  \"count\":")
	assert.Contains(t, output, "  \"format\":")
}
