package builders

import (
	"context"
	"errors"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/pkg/plugin"
)

// PluginMetadataBuilder helps build PluginMetadata entities for testing
type PluginMetadataBuilder struct {
	metadata *entities.PluginMetadata
}

// NewPluginMetadataBuilder creates a new plugin metadata builder with sensible defaults
func NewPluginMetadataBuilder() *PluginMetadataBuilder {
	return &PluginMetadataBuilder{
		metadata: &entities.PluginMetadata{
			Name:        "test-plugin",
			Version:     "1.0.0",
			Description: "Test plugin for unit tests",
			Author:      "Test Author",
			License:     "MIT",
			Homepage:    "https://example.com",
			Type:        entities.PluginTypeProcessor,
			Tags:        []string{"test"},
			Config:      make(map[string]string),
		},
	}
}

// WithName sets the plugin name
func (b *PluginMetadataBuilder) WithName(name string) *PluginMetadataBuilder {
	b.metadata.Name = name
	return b
}

// WithVersion sets the plugin version
func (b *PluginMetadataBuilder) WithVersion(version string) *PluginMetadataBuilder {
	b.metadata.Version = version
	return b
}

// WithDescription sets the plugin description
func (b *PluginMetadataBuilder) WithDescription(description string) *PluginMetadataBuilder {
	b.metadata.Description = description
	return b
}

// WithAuthor sets the plugin author
func (b *PluginMetadataBuilder) WithAuthor(author string) *PluginMetadataBuilder {
	b.metadata.Author = author
	return b
}

// WithType sets the plugin type
func (b *PluginMetadataBuilder) WithType(pluginType entities.PluginType) *PluginMetadataBuilder {
	b.metadata.Type = pluginType
	return b
}

// WithTags sets the plugin tags
func (b *PluginMetadataBuilder) WithTags(tags []string) *PluginMetadataBuilder {
	b.metadata.Tags = tags
	return b
}

// WithHomepage sets the plugin homepage
func (b *PluginMetadataBuilder) WithHomepage(homepage string) *PluginMetadataBuilder {
	b.metadata.Homepage = homepage
	return b
}

// WithConfig sets plugin configuration
func (b *PluginMetadataBuilder) WithConfig(key string, value string) *PluginMetadataBuilder {
	if b.metadata.Config == nil {
		b.metadata.Config = make(map[string]string)
	}
	b.metadata.Config[key] = value
	return b
}

// Build creates the final PluginMetadata entity
func (b *PluginMetadataBuilder) Build() entities.PluginMetadata {
	return entities.PluginMetadata{
		Name:        b.metadata.Name,
		Version:     b.metadata.Version,
		Description: b.metadata.Description,
		Author:      b.metadata.Author,
		License:     b.metadata.License,
		Homepage:    b.metadata.Homepage,
		Type:        b.metadata.Type,
		Tags:        append([]string{}, b.metadata.Tags...),
		Config:      copyConfig(b.metadata.Config),
	}
}

// LoadedPluginBuilder helps build LoadedPlugin entities for testing
type LoadedPluginBuilder struct {
	plugin *entities.LoadedPlugin
}

// NewLoadedPluginBuilder creates a new loaded plugin builder with sensible defaults
func NewLoadedPluginBuilder() *LoadedPluginBuilder {
	return &LoadedPluginBuilder{
		plugin: &entities.LoadedPlugin{
			Metadata:   NewPluginMetadataBuilder().Build(),
			Path:       "/test/plugins/test-plugin.so",
			LoadedAt:   time.Now(),
			Status:     entities.PluginStatusLoaded,
			ErrorMsg:   "",
			LastUsed:   time.Now(),
			Statistics: entities.PluginStatistics{},
		},
	}
}

// WithMetadata sets the plugin metadata
func (b *LoadedPluginBuilder) WithMetadata(metadata entities.PluginMetadata) *LoadedPluginBuilder {
	b.plugin.Metadata = metadata
	return b
}

// WithPath sets the plugin path
func (b *LoadedPluginBuilder) WithPath(path string) *LoadedPluginBuilder {
	b.plugin.Path = path
	return b
}

// WithStatus sets the plugin status
func (b *LoadedPluginBuilder) WithStatus(status entities.PluginStatus) *LoadedPluginBuilder {
	b.plugin.Status = status
	return b
}

// WithError sets the plugin error status and message
func (b *LoadedPluginBuilder) WithError(errorMsg string) *LoadedPluginBuilder {
	b.plugin.Status = entities.PluginStatusError
	b.plugin.ErrorMsg = errorMsg
	return b
}

// Build creates the final LoadedPlugin entity
func (b *LoadedPluginBuilder) Build() *entities.LoadedPlugin {
	return &entities.LoadedPlugin{
		Metadata:   b.plugin.Metadata,
		Path:       b.plugin.Path,
		LoadedAt:   b.plugin.LoadedAt,
		Status:     b.plugin.Status,
		ErrorMsg:   b.plugin.ErrorMsg,
		LastUsed:   b.plugin.LastUsed,
		Statistics: b.plugin.Statistics,
	}
}

// TestPlugin implements the Plugin interface for testing
type TestPlugin struct {
	name        string
	version     string
	description string
	initError   error
	execError   error
	output      plugin.PluginOutput
}

// NewTestPlugin creates a new test plugin with sensible defaults
func NewTestPlugin() *TestPlugin {
	return &TestPlugin{
		name:        "test-plugin",
		version:     "1.0.0",
		description: "Test plugin for unit tests",
		output:      plugin.PluginOutput{HTML: "<div>test output</div>"},
	}
}

// WithName sets the plugin name
func (p *TestPlugin) WithName(name string) *TestPlugin {
	p.name = name
	return p
}

// WithVersion sets the plugin version
func (p *TestPlugin) WithVersion(version string) *TestPlugin {
	p.version = version
	return p
}

// WithInitError sets an error to be returned from Init()
func (p *TestPlugin) WithInitError(err error) *TestPlugin {
	p.initError = err
	return p
}

// WithExecuteError sets an error to be returned from Execute()
func (p *TestPlugin) WithExecuteError(err error) *TestPlugin {
	p.execError = err
	return p
}

// WithOutput sets the output to be returned from Execute()
func (p *TestPlugin) WithOutput(output plugin.PluginOutput) *TestPlugin {
	p.output = output
	return p
}

// Plugin interface implementation
func (p *TestPlugin) Name() string        { return p.name }
func (p *TestPlugin) Version() string     { return p.version }
func (p *TestPlugin) Description() string { return p.description }

func (p *TestPlugin) Init(config map[string]interface{}) error {
	return p.initError
}

func (p *TestPlugin) Execute(ctx context.Context, input plugin.PluginInput) (plugin.PluginOutput, error) {
	if p.execError != nil {
		return plugin.PluginOutput{}, p.execError
	}
	return p.output, nil
}

func (p *TestPlugin) Cleanup() error {
	return nil
}

// Common plugin types for testing

// ProcessorPlugin creates a test processor plugin
func ProcessorPlugin() *TestPlugin {
	return NewTestPlugin().
		WithName("processor-plugin").
		WithOutput(plugin.PluginOutput{HTML: "<div>processed</div>"})
}

// SyntaxHighlightPlugin creates a test syntax highlighting plugin
func SyntaxHighlightPlugin() *TestPlugin {
	return NewTestPlugin().
		WithName("syntax-highlight").
		WithOutput(plugin.PluginOutput{HTML: "<pre><code class=\"go\">func main() {}</code></pre>"})
}

// MermaidPlugin creates a test Mermaid diagram plugin
func MermaidPlugin() *TestPlugin {
	return NewTestPlugin().
		WithName("mermaid").
		WithOutput(plugin.PluginOutput{HTML: "<div class=\"mermaid\">graph TD; A-->B;</div>"})
}

// FailingPlugin creates a plugin that always fails
func FailingPlugin() *TestPlugin {
	return NewTestPlugin().
		WithName("failing-plugin").
		WithExecuteError(errors.New("plugin execution failed"))
}

// copyConfig creates a deep copy of config map
func copyConfig(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}
	copy := make(map[string]string)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}
