package builders

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/pkg/plugin"
)

func TestPresentationBuilder(t *testing.T) {
	t.Run("builds presentation with defaults", func(t *testing.T) {
		presentation := NewPresentationBuilder().Build()

		assert.Equal(t, "Test Presentation", presentation.Title)
		assert.Equal(t, "Test Author", presentation.Author)
		assert.Equal(t, "default", presentation.Theme)
		assert.Empty(t, presentation.Slides)
		assert.NotNil(t, presentation.Metadata)
	})

	t.Run("builds presentation with custom values", func(t *testing.T) {
		customDate := time.Date(2024, 6, 30, 10, 0, 0, 0, time.UTC)

		presentation := NewPresentationBuilder().
			WithTitle("Custom Title").
			WithAuthor("Custom Author").
			WithDate(customDate).
			WithTheme("custom-theme").
			WithSlideCount(3).
			WithMetadata("category", "technical").
			Build()

		assert.Equal(t, "Custom Title", presentation.Title)
		assert.Equal(t, "Custom Author", presentation.Author)
		assert.Equal(t, customDate, presentation.Date)
		assert.Equal(t, "custom-theme", presentation.Theme)
		assert.Len(t, presentation.Slides, 3)
		assert.Equal(t, "technical", presentation.Metadata["category"])
	})

	t.Run("minimal presentation helper", func(t *testing.T) {
		presentation := MinimalPresentation()

		assert.Equal(t, "Minimal", presentation.Title)
		assert.Len(t, presentation.Slides, 1)
	})

	t.Run("large presentation helper", func(t *testing.T) {
		presentation := LargePresentation()

		assert.Equal(t, "Large Presentation", presentation.Title)
		assert.Len(t, presentation.Slides, 50)
	})
}

func TestSlideBuilder(t *testing.T) {
	t.Run("builds slide with defaults", func(t *testing.T) {
		slide := NewSlideBuilder().Build()

		assert.Equal(t, "slide-1", slide.ID)
		assert.Equal(t, 0, slide.Index)
		assert.Equal(t, "Test Slide", slide.Title)
		assert.Contains(t, slide.Content, "# Test Slide")
		assert.Contains(t, slide.HTML, "<h1>Test Slide</h1>")
		assert.Equal(t, "Test notes", slide.Notes)
		assert.NotNil(t, slide.Metadata)
	})

	t.Run("builds slide with custom values", func(t *testing.T) {
		slide := NewSlideBuilder().
			WithID(5).
			WithTitle("Custom Slide").
			WithHTML("<h1>Custom HTML</h1>").
			WithNotes("Custom notes").
			WithMetadata("type", "title").
			Build()

		assert.Equal(t, "slide-5", slide.ID)
		assert.Equal(t, 4, slide.Index) // 0-based index
		assert.Equal(t, "Custom Slide", slide.Title)
		assert.Equal(t, "<h1>Custom HTML</h1>", slide.HTML)
		assert.Equal(t, "Custom notes", slide.Notes)
		assert.Equal(t, "title", slide.Metadata["type"])
	})
}

func TestPluginMetadataBuilder(t *testing.T) {
	t.Run("builds metadata with defaults", func(t *testing.T) {
		metadata := NewPluginMetadataBuilder().Build()

		assert.Equal(t, "test-plugin", metadata.Name)
		assert.Equal(t, "1.0.0", metadata.Version)
		assert.Equal(t, "Test plugin for unit tests", metadata.Description)
		assert.Equal(t, "Test Author", metadata.Author)
		assert.Equal(t, "MIT", metadata.License)
		assert.Equal(t, entities.PluginTypeProcessor, metadata.Type)
		assert.Contains(t, metadata.Tags, "test")
		assert.NotNil(t, metadata.Config)
	})

	t.Run("builds metadata with custom values", func(t *testing.T) {
		metadata := NewPluginMetadataBuilder().
			WithName("custom-plugin").
			WithVersion("2.1.0").
			WithDescription("Custom plugin").
			WithAuthor("Custom Author").
			WithType(entities.PluginTypeExporter).
			WithTags([]string{"custom", "export"}).
			WithConfig("timeout", "30s").
			Build()

		assert.Equal(t, "custom-plugin", metadata.Name)
		assert.Equal(t, "2.1.0", metadata.Version)
		assert.Equal(t, "Custom plugin", metadata.Description)
		assert.Equal(t, "Custom Author", metadata.Author)
		assert.Equal(t, entities.PluginTypeExporter, metadata.Type)
		assert.Equal(t, []string{"custom", "export"}, metadata.Tags)
		assert.Equal(t, "30s", metadata.Config["timeout"])
	})
}

func TestLoadedPluginBuilder(t *testing.T) {
	t.Run("builds loaded plugin with defaults", func(t *testing.T) {
		plugin := NewLoadedPluginBuilder().Build()

		assert.Equal(t, "test-plugin", plugin.Metadata.Name)
		assert.Equal(t, "/test/plugins/test-plugin.so", plugin.Path)
		assert.Equal(t, entities.PluginStatusLoaded, plugin.Status)
		assert.Empty(t, plugin.ErrorMsg)
		assert.NotZero(t, plugin.LoadedAt)
		assert.NotZero(t, plugin.LastUsed)
	})

	t.Run("builds loaded plugin with error", func(t *testing.T) {
		plugin := NewLoadedPluginBuilder().
			WithError("plugin initialization failed").
			Build()

		assert.Equal(t, entities.PluginStatusError, plugin.Status)
		assert.Equal(t, "plugin initialization failed", plugin.ErrorMsg)
	})
}

func TestTestPlugin(t *testing.T) {
	t.Run("creates test plugin with defaults", func(t *testing.T) {
		testPlugin := NewTestPlugin()

		assert.Equal(t, "test-plugin", testPlugin.Name())
		assert.Equal(t, "1.0.0", testPlugin.Version())
		assert.Equal(t, "Test plugin for unit tests", testPlugin.Description())

		// Test init
		err := testPlugin.Init(nil)
		assert.NoError(t, err)

		// Test execute
		ctx := context.Background()
		input := plugin.PluginInput{}
		output, err := testPlugin.Execute(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, "<div>test output</div>", output.HTML)

		// Test cleanup
		err = testPlugin.Cleanup()
		assert.NoError(t, err)
	})

	t.Run("creates test plugin with custom behavior", func(t *testing.T) {
		testPlugin := NewTestPlugin().
			WithName("custom-plugin").
			WithVersion("2.0.0").
			WithExecuteError(assert.AnError)

		assert.Equal(t, "custom-plugin", testPlugin.Name())
		assert.Equal(t, "2.0.0", testPlugin.Version())

		// Test execute error
		ctx := context.Background()
		input := plugin.PluginInput{}
		_, err := testPlugin.Execute(ctx, input)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("helper functions work", func(t *testing.T) {
		processor := ProcessorPlugin()
		assert.Equal(t, "processor-plugin", processor.Name())

		syntaxHighlight := SyntaxHighlightPlugin()
		assert.Equal(t, "syntax-highlight", syntaxHighlight.Name())

		mermaid := MermaidPlugin()
		assert.Equal(t, "mermaid", mermaid.Name())

		failing := FailingPlugin()
		assert.Equal(t, "failing-plugin", failing.Name())
		ctx := context.Background()
		input := plugin.PluginInput{}
		_, err := failing.Execute(ctx, input)
		assert.Error(t, err)
	})
}
