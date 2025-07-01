package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// PollingWatcher implements file watching using polling
type PollingWatcher struct {
	interval  time.Duration
	debounce  time.Duration
	fileInfos map[string]FileInfo
	events    chan ports.FileChangeEvent
	mu        sync.RWMutex
	wg        sync.WaitGroup
	stopped   bool
	stopCh    chan struct{}
}

// FileInfo stores information about a file
type FileInfo struct {
	Size     int64
	ModTime  time.Time
	Checksum string
}

// NewPollingWatcher creates a new polling-based file watcher
func NewPollingWatcher(interval, debounce time.Duration) *PollingWatcher {
	return &PollingWatcher{
		interval:  interval,
		debounce:  debounce,
		fileInfos: make(map[string]FileInfo),
		events:    make(chan ports.FileChangeEvent, 10),
		stopCh:    make(chan struct{}),
	}
}

// Watch starts watching a file for changes
func (w *PollingWatcher) Watch(ctx context.Context, path string) (<-chan ports.FileChangeEvent, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Initial scan
	if err := w.scanFile(absPath); err != nil {
		return nil, fmt.Errorf("initial scan: %w", err)
	}

	// Start polling in background
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.pollLoop(ctx, absPath)
	}()

	return w.events, nil
}

// Stop stops the file watcher
func (w *PollingWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return nil
	}

	w.stopped = true
	close(w.stopCh)

	// Wait for goroutines to finish
	w.wg.Wait()

	// Close events channel
	close(w.events)

	return nil
}

// scanFile scans a file and stores its info
func (w *PollingWatcher) scanFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	checksum, err := w.calculateChecksum(path)
	if err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	w.mu.Lock()
	w.fileInfos[path] = FileInfo{
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Checksum: checksum,
	}
	w.mu.Unlock()

	return nil
}

// pollLoop continuously polls for file changes
func (w *PollingWatcher) pollLoop(ctx context.Context, path string) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	lastEventTime := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			changed, err := w.checkForChanges(path)
			if err != nil {
				log.Printf("watch error: %v", err)
				continue
			}

			if changed {
				// Only send event if enough time has passed since last event
				if time.Since(lastEventTime) >= w.debounce {
					event := ports.FileChangeEvent{
						Path:      path,
						Type:      ports.Modified,
						Timestamp: time.Now(),
					}

					select {
					case w.events <- event:
						lastEventTime = time.Now()
					case <-ctx.Done():
						return
					case <-w.stopCh:
						return
					}
				}
			}
		}
	}
}

// checkForChanges checks if a file has changed
func (w *PollingWatcher) checkForChanges(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File was deleted
			w.mu.Lock()
			delete(w.fileInfos, path)
			w.mu.Unlock()
			return true, nil
		}
		return false, fmt.Errorf("stat file: %w", err)
	}

	w.mu.RLock()
	oldInfo, exists := w.fileInfos[path]
	w.mu.RUnlock()

	// Smart pre-check: skip expensive checksum if size/time unchanged
	if exists && oldInfo.Size == info.Size() && oldInfo.ModTime.Equal(info.ModTime()) {
		return false, nil // File definitely hasn't changed
	}

	// Only calculate checksum when size or modification time changed
	checksum, err := w.calculateChecksum(path)
	if err != nil {
		return false, fmt.Errorf("calculate checksum: %w", err)
	}

	if !exists {
		// New file
		w.mu.Lock()
		w.fileInfos[path] = FileInfo{
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Checksum: checksum,
		}
		w.mu.Unlock()
		return true, nil
	}

	// Check if checksum actually changed
	changed := oldInfo.Checksum != checksum

	if changed {
		w.mu.Lock()
		w.fileInfos[path] = FileInfo{
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Checksum: checksum,
		}
		w.mu.Unlock()
	}

	return changed, nil
}

// calculateChecksum calculates SHA256 checksum of a file
func (w *PollingWatcher) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 - path is validated by caller
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
