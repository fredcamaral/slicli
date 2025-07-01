package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PanicPlugin is a plugin that panics during execution.
type PanicPlugin struct {
	MockPlugin
}

func (p *PanicPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	panic("test panic")
}

// SlowPlugin is a plugin that takes time to execute.
type SlowPlugin struct {
	MockPlugin
	duration time.Duration
}

func (p *SlowPlugin) Execute(ctx context.Context, input pluginapi.PluginInput) (pluginapi.PluginOutput, error) {
	select {
	case <-time.After(p.duration):
		return pluginapi.PluginOutput{HTML: "slow result"}, nil
	case <-ctx.Done():
		return pluginapi.PluginOutput{}, ctx.Err()
	}
}

func TestSandboxExecutor_Execute(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 10)
	ctx := context.Background()

	plugin := &MockPlugin{
		name:        "test",
		version:     "1.0.0",
		description: "Test plugin",
	}

	input := pluginapi.PluginInput{
		Content: "test content",
	}

	output, err := executor.Execute(ctx, plugin, input)
	require.NoError(t, err)
	assert.Equal(t, "<div>test content</div>", output.HTML)
}

func TestSandboxExecutor_ExecuteWithTimeout(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 10)
	ctx := context.Background()

	t.Run("successful execution within timeout", func(t *testing.T) {
		plugin := &SlowPlugin{
			MockPlugin: MockPlugin{name: "slow", version: "1.0.0"},
			duration:   100 * time.Millisecond,
		}

		input := pluginapi.PluginInput{Content: "test"}
		output, err := executor.ExecuteWithTimeout(ctx, plugin, input, 1*time.Second)
		require.NoError(t, err)
		assert.Equal(t, "slow result", output.HTML)
	})

	t.Run("timeout exceeded", func(t *testing.T) {
		plugin := &SlowPlugin{
			MockPlugin: MockPlugin{name: "slow", version: "1.0.0"},
			duration:   2 * time.Second,
		}

		input := pluginapi.PluginInput{Content: "test"}
		_, err := executor.ExecuteWithTimeout(ctx, plugin, input, 100*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
		assert.IsType(t, &pluginapi.PluginError{}, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		plugin := &SlowPlugin{
			MockPlugin: MockPlugin{name: "slow", version: "1.0.0"},
			duration:   1 * time.Second,
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		input := pluginapi.PluginInput{Content: "test"}
		_, err := executor.ExecuteWithTimeout(cancelCtx, plugin, input, 5*time.Second)
		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestSandboxExecutor_PanicRecovery(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 10)
	ctx := context.Background()

	plugin := &PanicPlugin{
		MockPlugin: MockPlugin{name: "panic", version: "1.0.0"},
	}

	input := pluginapi.PluginInput{Content: "test"}
	_, err := executor.Execute(ctx, plugin, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic")
	assert.Contains(t, err.Error(), "test panic")

	pluginErr, ok := err.(*pluginapi.PluginError)
	require.True(t, ok)
	assert.Equal(t, "panic", pluginErr.Plugin)
	assert.Equal(t, "execute", pluginErr.Operation)
}

func TestSandboxExecutor_Concurrency(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 2) // Max 2 concurrent
	ctx := context.Background()

	plugin := &SlowPlugin{
		MockPlugin: MockPlugin{name: "slow", version: "1.0.0"},
		duration:   100 * time.Millisecond,
	}

	// Start 3 executions
	results := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			input := pluginapi.PluginInput{Content: "test"}
			_, err := executor.Execute(ctx, plugin, input)
			results <- err
		}()
	}

	// All should complete eventually
	for i := 0; i < 3; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-time.After(1 * time.Second):
			t.Fatal("execution did not complete in time")
		}
	}
}

func TestSandboxExecutor_ExecuteAsync(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 10)
	ctx := context.Background()

	t.Run("successful async execution", func(t *testing.T) {
		plugin := &MockPlugin{
			name:        "test",
			version:     "1.0.0",
			description: "Test plugin",
		}

		input := pluginapi.PluginInput{Content: "async test"}
		resultChan := executor.ExecuteAsync(ctx, plugin, input)

		select {
		case result := <-resultChan:
			require.NoError(t, result.Error)
			assert.Equal(t, "<div>async test</div>", result.Output.HTML)
		case <-time.After(1 * time.Second):
			t.Fatal("async execution did not complete")
		}
	})

	t.Run("async execution with error", func(t *testing.T) {
		plugin := &MockPlugin{
			name:      "test",
			version:   "1.0.0",
			execError: errors.New("execution failed"),
		}

		input := pluginapi.PluginInput{Content: "async test"}
		resultChan := executor.ExecuteAsync(ctx, plugin, input)

		select {
		case result := <-resultChan:
			require.Error(t, result.Error)
			assert.Contains(t, result.Error.Error(), "execution failed")
		case <-time.After(1 * time.Second):
			t.Fatal("async execution did not complete")
		}
	})
}

func TestSandboxExecutor_GetExecutingPlugins(t *testing.T) {
	executor := NewSandboxExecutor(5*time.Second, 10)
	ctx := context.Background()

	// No plugins executing initially
	executing := executor.GetExecutingPlugins()
	assert.Empty(t, executing)

	// Start a slow plugin
	plugin := &SlowPlugin{
		MockPlugin: MockPlugin{name: "slow", version: "1.0.0"},
		duration:   200 * time.Millisecond,
	}

	done := make(chan bool)
	go func() {
		input := pluginapi.PluginInput{Content: "test"}
		_, _ = executor.Execute(ctx, plugin, input)
		done <- true
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Should show as executing
	executing = executor.GetExecutingPlugins()
	assert.Len(t, executing, 1)
	assert.Contains(t, executing, "slow")

	// Wait for completion
	<-done

	// Should no longer be executing
	executing = executor.GetExecutingPlugins()
	assert.Empty(t, executing)
}
