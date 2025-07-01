package plugin

import (
	"container/heap"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	pluginapi "github.com/fredcamaral/slicli/pkg/plugin"
)

// MemoryCache is an in-memory cache for plugin outputs with O(log n) eviction.
type MemoryCache struct {
	mu          sync.RWMutex
	entries     map[string]*cacheEntry
	heap        *cacheHeap
	heapLookup  map[string]*heapEntry // key -> heap entry mapping for O(1) access
	maxSize     int64
	currentSize int64
	stats       entities.CacheStats
}

type cacheEntry struct {
	output    *pluginapi.PluginOutput
	size      int64
	expiresAt time.Time
	hits      int64
}

// NewMemoryCache creates a new memory cache with heap-based LRU eviction.
func NewMemoryCache(maxSize int64) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // 100MB default
	}
	h := &cacheHeap{}
	heap.Init(h)

	return &MemoryCache{
		entries:    make(map[string]*cacheEntry),
		heap:       h,
		heapLookup: make(map[string]*heapEntry),
		maxSize:    maxSize,
		stats: entities.CacheStats{
			MaxSize: int(maxSize),
		},
	}
}

// Get retrieves a cached result.
func (c *MemoryCache) Get(key string) (*pluginapi.PluginOutput, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	heapEntry, heapExists := c.heapLookup[key]
	c.mu.RUnlock()

	if !exists || !heapExists {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		c.removeFromHeap(key)
		delete(c.entries, key)
		c.currentSize -= entry.size
		c.stats.Evictions++
		c.stats.Misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	entry.hits++
	c.stats.Hits++

	// Update heap entry access time - O(log n)
	heapEntry.lastAccess = time.Now()
	heap.Fix(c.heap, heapEntry.index)
	c.mu.Unlock()

	return entry.output, true
}

// Set stores a result in the cache.
func (c *MemoryCache) Set(key string, output *pluginapi.PluginOutput, ttl time.Duration) {
	if output == nil {
		return
	}

	// Calculate size
	size := c.calculateSize(output)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove existing entry if it exists
	if existingEntry, exists := c.entries[key]; exists {
		c.removeFromHeap(key)
		delete(c.entries, key)
		c.currentSize -= existingEntry.size
	}

	// Check if we need to evict entries
	if c.currentSize+size > c.maxSize {
		c.evictLRU(size)
	}

	// Store the entry
	now := time.Now()
	c.entries[key] = &cacheEntry{
		output:    output,
		size:      size,
		expiresAt: now.Add(ttl),
		hits:      0,
	}

	// Add to heap for LRU tracking
	heapEntry := &heapEntry{
		key:        key,
		lastAccess: now,
	}
	heap.Push(c.heap, heapEntry)
	c.heapLookup[key] = heapEntry

	c.currentSize += size
	c.stats.Size = len(c.entries)
}

// Remove removes a result from the cache.
func (c *MemoryCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.removeFromHeap(key)
		delete(c.entries, key)
		c.currentSize -= entry.size
		c.stats.Size = len(c.entries)
	}
}

// Clear removes all results from the cache.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.heapLookup = make(map[string]*heapEntry)
	*c.heap = (*c.heap)[:0] // clear heap slice
	c.currentSize = 0
	c.stats.Size = 0
}

// Stats returns cache statistics.
func (c *MemoryCache) Stats() entities.CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	return stats
}

// calculateSize estimates the size of a plugin output.
func (c *MemoryCache) calculateSize(output *pluginapi.PluginOutput) int64 {
	size := int64(len(output.HTML))
	for _, asset := range output.Assets {
		size += int64(len(asset.Name))
		size += int64(len(asset.Content))
		size += int64(len(asset.ContentType))
	}
	// Add some overhead for the map structure
	size += int64(len(output.Metadata)) * 100
	return size
}

// evictLRU evicts least recently used entries to make room.
func (c *MemoryCache) evictLRU(neededSize int64) {
	// Remove expired entries first
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			c.removeFromHeap(key)
			delete(c.entries, key)
			c.currentSize -= entry.size
			c.stats.Evictions++
			if c.currentSize+neededSize <= c.maxSize {
				return
			}
		}
	}

	// Heap-based LRU eviction - O(log n) per eviction
	for c.currentSize+neededSize > c.maxSize && c.heap.Len() > 0 {
		// Pop least recently used entry in O(log n)
		lruHeapEntry := heap.Pop(c.heap).(*heapEntry)

		// Remove from cache
		if entry, exists := c.entries[lruHeapEntry.key]; exists {
			delete(c.entries, lruHeapEntry.key)
			delete(c.heapLookup, lruHeapEntry.key)
			c.currentSize -= entry.size
			c.stats.Evictions++
		}
	}
}

// Cleanup removes expired entries periodically.
func (c *MemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			c.removeFromHeap(key)
			delete(c.entries, key)
			c.currentSize -= entry.size
			c.stats.Evictions++
		}
	}
	c.stats.Size = len(c.entries)
}

// removeFromHeap removes an entry from the heap by key
func (c *MemoryCache) removeFromHeap(key string) {
	if heapEntry, exists := c.heapLookup[key]; exists {
		heap.Remove(c.heap, heapEntry.index)
		delete(c.heapLookup, key)
	}
}

// StartCleanupTimer starts a timer to periodically clean up expired entries.
func (c *MemoryCache) StartCleanupTimer(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.Cleanup()
		}
	}()
}
