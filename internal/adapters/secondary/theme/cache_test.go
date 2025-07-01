package theme

import (
	"fmt"
	"html/template"
	"testing"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func createTestTheme(name string) *entities.ThemeEngine {
	return &entities.ThemeEngine{
		Name: name,
		Path: "/themes/" + name,
		Templates: map[string]*template.Template{
			"presentation": template.New("presentation"),
			"slide":        template.New("slide"),
		},
		Assets: map[string]*entities.ThemeAsset{
			"css/main.css": {
				Path:        "/themes/" + name + "/assets/css/main.css",
				Content:     []byte("body { color: red; }"),
				ContentType: "text/css",
			},
		},
		Config: entities.ThemeEngineConfig{
			Variables: map[string]string{
				"primary-color": "#000",
			},
		},
		LoadedAt: time.Now(),
	}
}

func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Test setting and getting a theme
	theme := createTestTheme("test")
	cache.Set("test", theme)

	retrieved, found := cache.Get("test")
	assert.True(t, found)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test", retrieved.Name)

	// Test getting non-existent theme
	_, found = cache.Get("nonexistent")
	assert.False(t, found)
}

func TestMemoryCache_TTLExpiration(t *testing.T) {
	cache := NewMemoryCache(10, 100*time.Millisecond)

	// Set a theme
	theme := createTestTheme("test")
	cache.Set("test", theme)

	// Should be retrievable immediately
	_, found := cache.Get("test")
	assert.True(t, found)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should no longer be retrievable
	_, found = cache.Get("test")
	assert.False(t, found)
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	cache := NewMemoryCache(3, 1*time.Hour)

	// Fill cache to capacity
	cache.Set("theme1", createTestTheme("theme1"))
	cache.Set("theme2", createTestTheme("theme2"))
	cache.Set("theme3", createTestTheme("theme3"))

	// Access theme1 and theme2 to make them more recently used
	cache.Get("theme1")
	cache.Get("theme2")

	// Add a new theme, should evict theme3 (least recently used)
	cache.Set("theme4", createTestTheme("theme4"))

	// Check what's in cache
	_, found := cache.Get("theme1")
	assert.True(t, found)
	_, found = cache.Get("theme2")
	assert.True(t, found)
	_, found = cache.Get("theme4")
	assert.True(t, found)
	_, found = cache.Get("theme3")
	assert.False(t, found) // Should have been evicted
}

func TestMemoryCache_Remove(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Set and remove a theme
	theme := createTestTheme("test")
	cache.Set("test", theme)

	cache.Remove("test")

	// Should no longer be retrievable
	_, found := cache.Get("test")
	assert.False(t, found)

	// Removing non-existent should not panic
	cache.Remove("nonexistent")
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Add multiple themes
	cache.Set("theme1", createTestTheme("theme1"))
	cache.Set("theme2", createTestTheme("theme2"))
	cache.Set("theme3", createTestTheme("theme3"))

	// Clear cache
	cache.Clear()

	// Nothing should be retrievable
	_, found := cache.Get("theme1")
	assert.False(t, found)
	_, found = cache.Get("theme2")
	assert.False(t, found)
	_, found = cache.Get("theme3")
	assert.False(t, found)

	// Stats should show everything cleared
	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 10, stats.MaxSize)
}

func TestMemoryCache_Stats(t *testing.T) {
	cache := NewMemoryCache(3, 1*time.Hour)

	// Initial stats
	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, 3, stats.MaxSize)

	// Add themes
	cache.Set("theme1", createTestTheme("theme1"))
	cache.Set("theme2", createTestTheme("theme2"))

	// Get existing (hit)
	cache.Get("theme1")

	// Get non-existing (miss)
	cache.Get("nonexistent")

	stats = cache.Stats()
	assert.Equal(t, 2, stats.Size)
	assert.Equal(t, int64(1), stats.Hits)

	// Fill cache and trigger eviction
	cache.Set("theme3", createTestTheme("theme3"))
	cache.Set("theme4", createTestTheme("theme4")) // Should evict theme2

	stats = cache.Stats()
	assert.Equal(t, 3, stats.Size)
	assert.Equal(t, 3, stats.MaxSize)
}

func TestMemoryCache_Concurrent(t *testing.T) {
	cache := NewMemoryCache(100, 1*time.Hour)
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			theme := createTestTheme(fmt.Sprintf("theme%d", id))
			cache.Set(fmt.Sprintf("theme%d", id), theme)
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			time.Sleep(10 * time.Millisecond) // Give writers a head start
			cache.Get(fmt.Sprintf("theme%d", id))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify all themes are in cache
	for i := 0; i < 10; i++ {
		_, found := cache.Get(fmt.Sprintf("theme%d", i))
		assert.True(t, found)
	}
}

func TestMemoryCache_UpdateExisting(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Set initial theme
	theme1 := createTestTheme("test")
	theme1.Config.Variables["version"] = "1.0.0"
	cache.Set("test", theme1)

	// Update with new version
	theme2 := createTestTheme("test")
	theme2.Config.Variables["version"] = "2.0.0"
	cache.Set("test", theme2)

	// Should get the updated version
	retrieved, found := cache.Get("test")
	assert.True(t, found)
	assert.Equal(t, "2.0.0", retrieved.Config.Variables["version"])

	// Stats should show no additional size increase
	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
}

func TestMemoryCache_ZeroTTL(t *testing.T) {
	// Zero TTL means no expiration
	cache := NewMemoryCache(10, 0)

	theme := createTestTheme("test")
	cache.Set("test", theme)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Should still be retrievable
	_, found := cache.Get("test")
	assert.True(t, found)
}

func TestMemoryCache_MinimalSize(t *testing.T) {
	// Cache with size 1
	cache := NewMemoryCache(1, 1*time.Hour)

	cache.Set("theme1", createTestTheme("theme1"))
	cache.Set("theme2", createTestTheme("theme2")) // Should evict theme1

	_, found := cache.Get("theme1")
	assert.False(t, found)
	_, found = cache.Get("theme2")
	assert.True(t, found)

	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 1, stats.MaxSize)
}
