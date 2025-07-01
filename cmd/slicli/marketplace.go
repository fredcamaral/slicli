package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/config"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/plugin"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/spf13/cobra"
)

var marketplaceCmd = &cobra.Command{
	Use:   "marketplace",
	Short: "Browse and install plugins from marketplace",
	Long: `The marketplace command provides access to the slicli plugin marketplace.
You can browse, search, and install both free and premium plugins to extend slicli functionality.`,
}

var marketplaceListCmd = &cobra.Command{
	Use:   "list [category]",
	Short: "List available plugins",
	Long: `List plugins available in the marketplace.
Optionally filter by category: syntax, diagram, export, content, or utility.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMarketplaceList,
}

var marketplaceSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for plugins",
	Long:  "Search plugins by name, description, or tags.",
	Args:  cobra.ExactArgs(1),
	RunE:  runMarketplaceSearch,
}

var marketplaceInstallCmd = &cobra.Command{
	Use:   "install <plugin-id>",
	Short: "Install a plugin",
	Long: `Install a plugin from the marketplace.
For premium plugins, you must purchase a license first.`,
	Args: cobra.ExactArgs(1),
	RunE: runMarketplaceInstall,
}

var marketplaceInfoCmd = &cobra.Command{
	Use:   "info <plugin-id>",
	Short: "Show plugin information",
	Long:  "Display detailed information about a specific plugin.",
	Args:  cobra.ExactArgs(1),
	RunE:  runMarketplaceInfo,
}

var marketplaceFeaturedCmd = &cobra.Command{
	Use:   "featured",
	Short: "Show featured plugins",
	Long:  "Display featured plugins recommended by the community.",
	RunE:  runMarketplaceFeatured,
}

// Purchase and license commands removed - all plugins are free

// Flags
var (
	marketplacePremiumOnly bool
	marketplaceShowAll     bool
	marketplaceFormat      string
	marketplaceVersion     string
)

func init() {
	// Add marketplace subcommands
	marketplaceCmd.AddCommand(marketplaceListCmd)
	marketplaceCmd.AddCommand(marketplaceSearchCmd)
	marketplaceCmd.AddCommand(marketplaceInstallCmd)
	marketplaceCmd.AddCommand(marketplaceInfoCmd)
	marketplaceCmd.AddCommand(marketplaceFeaturedCmd)
	// Purchase and license commands removed - all plugins are free

	// Flags for list command
	marketplaceListCmd.Flags().BoolVar(&marketplacePremiumOnly, "premium", false, "Show only premium plugins")
	marketplaceListCmd.Flags().StringVar(&marketplaceFormat, "format", "table", "Output format: table, json")

	// Flags for install command
	marketplaceInstallCmd.Flags().StringVar(&marketplaceVersion, "version", "", "Specific version to install")
	marketplaceInstallCmd.Flags().BoolVar(&marketplaceShowAll, "force", false, "Force install even if already installed")

	// Add to root command
	rootCmd.AddCommand(marketplaceCmd)
}

func getMarketplaceClient() (*plugin.MarketplaceClient, error) {
	return getMarketplaceClientWithConfig(nil)
}

func getMarketplaceClientWithConfig(appConfig *entities.Config) (*plugin.MarketplaceClient, error) {
	// Load basic config if none provided
	if appConfig == nil {
		// Start with defaults and try to load global config
		appConfig = config.GetDefaultConfig()

		loader := config.NewTOMLLoader()
		ctx := context.Background()

		// Try to load global config and merge if available
		if globalConfig, err := loader.LoadGlobal(ctx); err == nil && globalConfig != nil {
			// Simple merge for marketplace URL (would need proper merge for production)
			if globalConfig.Plugins.MarketplaceURL != "" {
				appConfig.Plugins.MarketplaceURL = globalConfig.Plugins.MarketplaceURL
			}
		}
	}

	// Create marketplace configuration using config system
	marketplaceConfig := plugin.MarketplaceConfig{
		BaseURL: appConfig.Plugins.GetMarketplaceURL(),
		APIKey:  os.Getenv("SLICLI_API_KEY"),
		UserID:  getUserID(),
	}

	return plugin.NewMarketplaceClient(marketplaceConfig), nil
}

func getUserID() string {
	// Try to get user ID from config or environment
	if userID := os.Getenv("SLICLI_USER_ID"); userID != "" {
		return userID
	}

	// Fallback to system username for now
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}

	return "anonymous"
}

func runMarketplaceList(cmd *cobra.Command, args []string) error {
	client, err := getMarketplaceClient()
	if err != nil {
		return fmt.Errorf("failed to create marketplace client: %w", err)
	}

	var category entities.PluginType
	if len(args) > 0 {
		category = entities.PluginType(args[0])
	}

	plugins, err := client.ListPlugins(category, marketplacePremiumOnly)
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins found matching criteria.")
		return nil
	}

	if marketplaceFormat == "json" {
		return printPluginsJSON(plugins)
	}

	return printPluginsTable(plugins)
}

func runMarketplaceSearch(cmd *cobra.Command, args []string) error {
	client, err := getMarketplaceClient()
	if err != nil {
		return fmt.Errorf("failed to create marketplace client: %w", err)
	}

	plugins, err := client.SearchPlugins(args[0])
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Printf("No plugins found for query: %s\n", args[0])
		return nil
	}

	fmt.Printf("Found %d plugin(s) for '%s':\n\n", len(plugins), args[0])
	return printPluginsTable(plugins)
}

func runMarketplaceInstall(cmd *cobra.Command, args []string) error {
	pluginID := args[0]

	client, err := getMarketplaceClient()
	if err != nil {
		return fmt.Errorf("failed to create marketplace client: %w", err)
	}

	// Get plugin information
	pluginInfo, err := client.GetPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// All plugins are free in open source model

	// Determine platform
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	version := marketplaceVersion
	if version == "" {
		version = pluginInfo.Version
	}

	fmt.Printf("Installing %s v%s...\n", pluginInfo.Name, version)

	// Download plugin
	pluginData, err := client.DownloadPlugin(pluginID, version, platform)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Create plugins directory
	pluginsDir := getPluginsDirectory()
	if err := os.MkdirAll(pluginsDir, 0750); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Save plugin file
	pluginPath := filepath.Join(pluginsDir, pluginID+".so")
	if err := os.WriteFile(pluginPath, pluginData, 0600); err != nil {
		return fmt.Errorf("failed to save plugin: %w", err)
	}

	fmt.Printf("✓ Successfully installed %s to %s\n", pluginInfo.Name, pluginPath)
	fmt.Printf("Use it in your presentations with: <!-- plugin: %s -->\n", pluginID)

	return nil
}

func runMarketplaceInfo(cmd *cobra.Command, args []string) error {
	client, err := getMarketplaceClient()
	if err != nil {
		return fmt.Errorf("failed to create marketplace client: %w", err)
	}

	plugin, err := client.GetPlugin(args[0])
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// Print detailed plugin information
	fmt.Printf("Plugin: %s\n", plugin.Name)
	fmt.Printf("ID: %s\n", plugin.ID)
	fmt.Printf("Version: %s\n", plugin.Version)
	fmt.Printf("Author: %s\n", plugin.Author)
	fmt.Printf("Description: %s\n", plugin.Description)
	fmt.Printf("Category: %s\n", plugin.Category)
	fmt.Printf("License: %s\n", plugin.License)
	fmt.Printf("Rating: %.1f/5.0\n", plugin.Rating)
	fmt.Printf("Downloads: %s\n", formatNumber(plugin.Downloads))

	fmt.Println("Price: Free (Open Source)")

	if len(plugin.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(plugin.Tags, ", "))
	}

	if len(plugin.Dependencies) > 0 {
		fmt.Println("\nDependencies:")
		for _, dep := range plugin.Dependencies {
			fmt.Printf("  - %s (%s)\n", dep.Name, dep.Version)
		}
	}

	if len(plugin.Platforms) > 0 {
		fmt.Printf("\nPlatforms: %s\n", strings.Join(plugin.Platforms, ", "))
	}

	if plugin.Repository != "" {
		fmt.Printf("\nRepository: %s\n", plugin.Repository)
	}

	if plugin.Homepage != "" {
		fmt.Printf("Homepage: %s\n", plugin.Homepage)
	}

	return nil
}

func runMarketplaceFeatured(cmd *cobra.Command, args []string) error {
	client, err := getMarketplaceClient()
	if err != nil {
		return fmt.Errorf("failed to create marketplace client: %w", err)
	}

	plugins, err := client.GetFeaturedPlugins()
	if err != nil {
		return fmt.Errorf("failed to get featured plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Println("No featured plugins available.")
		return nil
	}

	fmt.Printf("Featured Plugins (%d):\n\n", len(plugins))
	return printPluginsTable(plugins)
}

// Purchase and license functions removed - all plugins are free

// Helper functions

func printPluginsTable(plugins []*plugin.MarketplacePlugin) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "ID\tNAME\tVERSION\tCATEGORY\tRATING\tDOWNLOADS\tPRICE\tDESCRIPTION\n")

	for _, p := range plugins {
		price := "Free"

		description := p.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f\t%s\t%s\t%s\n",
			p.ID, p.Name, p.Version, p.Category, p.Rating,
			formatNumber(p.Downloads), price, description)
	}

	return w.Flush()
}

func printPluginsJSON(plugins []*plugin.MarketplacePlugin) error {
	// Create JSON output structure
	output := struct {
		Plugins []*plugin.MarketplacePlugin `json:"plugins"`
		Count   int                         `json:"count"`
		Format  string                      `json:"format"`
	}{
		Plugins: plugins,
		Count:   len(plugins),
		Format:  "marketplace-v1",
	}

	// Marshal to JSON with pretty printing
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plugins to JSON: %w", err)
	}

	// Print to stdout
	fmt.Println(string(jsonData))
	return nil
}

func formatNumber(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func getPluginsDirectory() string {
	// Try XDG config directory first
	if configDir := os.Getenv("XDG_CONFIG_HOME"); configDir != "" {
		return filepath.Join(configDir, "slicli", "plugins")
	}

	// Fall back to home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./plugins" // fallback to current directory
	}

	return filepath.Join(homeDir, ".config", "slicli", "plugins")
}
