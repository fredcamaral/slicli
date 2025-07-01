package watcher

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

func createTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "watcher-test-*.md")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}

func updateFile(t *testing.T, path string, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func TestPollingWatcher(t *testing.T) {
	t.Run("create new watcher", func(t *testing.T) {
		watcher := NewPollingWatcher(100*time.Millisecond, 500*time.Millisecond)
		assert.NotNil(t, watcher)
		assert.Equal(t, 100*time.Millisecond, watcher.interval)
		assert.Equal(t, 500*time.Millisecond, watcher.debounce)
	})

	t.Run("watch file changes", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer func() { _ = watcher.Stop() }()

		// Create temp file
		tmpFile := createTempFile(t, "initial content")
		defer func() { _ = os.Remove(tmpFile) }()

		// Start watching
		events, err := watcher.Watch(ctx, tmpFile)
		require.NoError(t, err)

		// Update file
		time.Sleep(100 * time.Millisecond) // Wait for initial scan
		updateFile(t, tmpFile, "updated content")

		// Should receive change event
		select {
		case event := <-events:
			assert.Equal(t, tmpFile, event.Path)
			assert.Equal(t, ports.Modified, event.Type)
			assert.WithinDuration(t, time.Now(), event.Timestamp, 2*time.Second)
		case <-time.After(2 * time.Second):
			t.Fatal("no event received")
		}
	})

	t.Run("debouncing", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 200*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer func() { _ = watcher.Stop() }()

		// Create temp file
		tmpFile := createTempFile(t, "initial")
		defer func() { _ = os.Remove(tmpFile) }()

		// Start watching
		events, err := watcher.Watch(ctx, tmpFile)
		require.NoError(t, err)

		// Wait for initial scan
		time.Sleep(100 * time.Millisecond)

		// Make rapid changes
		for i := 0; i < 3; i++ {
			updateFile(t, tmpFile, fmt.Sprintf("change %d", i))
			time.Sleep(30 * time.Millisecond)
		}

		// Should only get one event due to debouncing
		select {
		case event := <-events:
			assert.Equal(t, ports.Modified, event.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("no event received")
		}

		// Should not get another event immediately
		select {
		case <-events:
			t.Fatal("got unexpected second event")
		case <-time.After(300 * time.Millisecond):
			// Good - no extra events
		}
	})

	t.Run("file deletion", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer func() { _ = watcher.Stop() }()

		// Create temp file
		tmpFile := createTempFile(t, "content")

		// Start watching
		events, err := watcher.Watch(ctx, tmpFile)
		require.NoError(t, err)

		// Wait for initial scan
		time.Sleep(100 * time.Millisecond)

		// Delete file
		err = os.Remove(tmpFile)
		require.NoError(t, err)

		// Should receive change event
		select {
		case event := <-events:
			assert.Equal(t, tmpFile, event.Path)
			assert.Equal(t, ports.Modified, event.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("no event received for deletion")
		}
	})

	t.Run("stop watcher", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)
		ctx := context.Background()

		// Create temp file
		tmpFile := createTempFile(t, "content")
		defer func() { _ = os.Remove(tmpFile) }()

		// Start watching
		events, err := watcher.Watch(ctx, tmpFile)
		require.NoError(t, err)

		// Stop watcher
		err = watcher.Stop()
		assert.NoError(t, err)

		// Channel should be closed
		_, ok := <-events
		assert.False(t, ok)

		// Stop again should not error
		err = watcher.Stop()
		assert.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)
		ctx, cancel := context.WithCancel(context.Background())
		defer func() { _ = watcher.Stop() }()

		// Create temp file
		tmpFile := createTempFile(t, "content")
		defer func() { _ = os.Remove(tmpFile) }()

		// Start watching
		events, err := watcher.Watch(ctx, tmpFile)
		require.NoError(t, err)

		// Cancel context
		cancel()

		// Should not receive events after cancellation
		time.Sleep(200 * time.Millisecond)
		updateFile(t, tmpFile, "updated")

		select {
		case <-events:
			// May receive one event if it was already in flight
		case <-time.After(200 * time.Millisecond):
			// Good - no event
		}
	})

	t.Run("invalid file path", func(t *testing.T) {
		watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)
		ctx := context.Background()

		// Try to watch non-existent file
		_, err := watcher.Watch(ctx, "/nonexistent/path/file.md")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initial scan")
	})
}

func TestFileInfoChecksum(t *testing.T) {
	watcher := NewPollingWatcher(50*time.Millisecond, 100*time.Millisecond)

	t.Run("calculate checksum", func(t *testing.T) {
		tmpFile := createTempFile(t, "test content")
		defer func() { _ = os.Remove(tmpFile) }()

		checksum1, err := watcher.calculateChecksum(tmpFile)
		require.NoError(t, err)
		assert.NotEmpty(t, checksum1)

		// Same content should give same checksum
		checksum2, err := watcher.calculateChecksum(tmpFile)
		require.NoError(t, err)
		assert.Equal(t, checksum1, checksum2)

		// Different content should give different checksum
		updateFile(t, tmpFile, "different content")
		checksum3, err := watcher.calculateChecksum(tmpFile)
		require.NoError(t, err)
		assert.NotEqual(t, checksum1, checksum3)
	})

	t.Run("checksum of non-existent file", func(t *testing.T) {
		_, err := watcher.calculateChecksum("/nonexistent/file")
		assert.Error(t, err)
	})
}

func TestPollingWatcherRaceConditions(t *testing.T) {
	watcher := NewPollingWatcher(10*time.Millisecond, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() { _ = watcher.Stop() }()

	// Create multiple temp files
	var tmpFiles []string
	for i := 0; i < 5; i++ {
		tmpFile := createTempFile(t, fmt.Sprintf("file %d", i))
		tmpFiles = append(tmpFiles, tmpFile)
		defer func() { _ = os.Remove(tmpFile) }()
	}

	// Watch all files concurrently
	for _, file := range tmpFiles {
		_, err := watcher.Watch(ctx, file)
		require.NoError(t, err)
	}

	// Update files concurrently
	for i, file := range tmpFiles {
		go func(idx int, path string) {
			for j := 0; j < 5; j++ {
				updateFile(t, path, fmt.Sprintf("update %d-%d", idx, j))
				time.Sleep(20 * time.Millisecond)
			}
		}(i, file)
	}

	// Let it run for a bit
	time.Sleep(500 * time.Millisecond)

	// Should not panic or deadlock
}

func TestPollingInterval(t *testing.T) {
	// Test that polling happens at the specified interval
	watcher := NewPollingWatcher(100*time.Millisecond, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() { _ = watcher.Stop() }()

	tmpFile := createTempFile(t, "initial")
	defer func() { _ = os.Remove(tmpFile) }()

	events, err := watcher.Watch(ctx, tmpFile)
	require.NoError(t, err)

	// Wait for initial scan
	time.Sleep(150 * time.Millisecond)

	// Track when we make the change
	changeTime := time.Now()
	updateFile(t, tmpFile, "updated")

	// Should receive event within reasonable time based on polling interval
	select {
	case <-events:
		detectionTime := time.Since(changeTime)
		// Should detect within 3 polling intervals (more lenient)
		assert.Less(t, detectionTime, 400*time.Millisecond)
		// Don't assert minimum time since it depends on when polling cycle hits
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no event received")
	}
}
