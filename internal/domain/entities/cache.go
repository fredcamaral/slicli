package entities

// CacheStats represents cache statistics
type CacheStats struct {
	// Hits is the number of cache hits
	Hits int64 `json:"hits"`

	// Misses is the number of cache misses
	Misses int64 `json:"misses"`

	// Evictions is the number of cache evictions
	Evictions int64 `json:"evictions"`

	// Size is the current number of items in cache
	Size int `json:"size"`

	// MaxSize is the maximum cache size
	MaxSize int `json:"max_size"`

	// HitRate is the percentage of cache hits
	HitRate float64 `json:"hit_rate"`
}
