package theme

import (
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// MemoryCache is an in-memory theme cache
type MemoryCache struct {
	mu      sync.RWMutex
	themes  map[string]*cachedTheme
	maxSize int
	ttl     time.Duration
}

// cachedTheme wraps a theme with cache metadata
type cachedTheme struct {
	theme     *entities.ThemeEngine
	expiresAt time.Time
	hits      int
	lastHit   time.Time
}

// NewMemoryCache creates a new in-memory theme cache
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		themes:  make(map[string]*cachedTheme),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached theme
func (c *MemoryCache) Get(name string) (*entities.ThemeEngine, bool) {
	c.mu.RLock()
	cached, exists := c.themes[name]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check expiration (ttl of 0 means no expiration)
	if c.ttl > 0 && time.Now().After(cached.expiresAt) {
		c.Remove(name)
		return nil, false
	}

	// Update hit statistics
	c.mu.Lock()
	cached.hits++
	cached.lastHit = time.Now()
	c.mu.Unlock()

	return cached.theme, true
}

// Set stores a theme in the cache
func (c *MemoryCache) Set(name string, theme *entities.ThemeEngine) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict
	if len(c.themes) >= c.maxSize && c.maxSize > 0 {
		c.evictLRU()
	}

	expiresAt := time.Time{} // Zero time means no expiration
	if c.ttl > 0 {
		expiresAt = time.Now().Add(c.ttl)
	}

	c.themes[name] = &cachedTheme{
		theme:     theme,
		expiresAt: expiresAt,
		hits:      0,
		lastHit:   time.Now(),
	}
}

// Remove removes a theme from the cache
func (c *MemoryCache) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.themes, name)
}

// Clear clears all cached themes
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.themes = make(map[string]*cachedTheme)
}

// evictLRU evicts the least recently used theme
func (c *MemoryCache) evictLRU() {
	var (
		evictName string
		oldestHit = time.Now()
	)

	for name, cached := range c.themes {
		if cached.lastHit.Before(oldestHit) {
			oldestHit = cached.lastHit
			evictName = name
		}
	}

	if evictName != "" {
		delete(c.themes, evictName)
	}
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() entities.CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := entities.CacheStats{
		Size:    len(c.themes),
		MaxSize: c.maxSize,
	}

	var hits int64
	var active int
	for _, cached := range c.themes {
		hits += int64(cached.hits)
		// Count as active if no expiration (zero time) or hasn't expired yet
		if cached.expiresAt.IsZero() || cached.expiresAt.After(time.Now()) {
			active++
		}
	}

	// Additional fields that can be tracked
	stats.Hits = hits

	return stats
}

// Additional type for internal use
type CacheStatsExtended struct {
	Size      int           // Current number of cached themes
	MaxSize   int           // Maximum cache size
	Active    int           // Number of non-expired themes
	TotalHits int           // Total cache hits
	TTL       time.Duration // Time to live for cached themes
}

// Ensure MemoryCache implements ThemeCache
var _ ports.ThemeCache = (*MemoryCache)(nil)
