package plugin

import (
	"time"
)

// heapEntry represents an entry in the priority queue for O(log n) LRU operations
type heapEntry struct {
	key        string
	lastAccess time.Time
	index      int // heap index for efficient updates
}

// cacheHeap implements a min-heap based on last access time for LRU eviction
type cacheHeap []*heapEntry

func (h cacheHeap) Len() int { return len(h) }

func (h cacheHeap) Less(i, j int) bool {
	// Earlier access time = higher priority for eviction
	return h[i].lastAccess.Before(h[j].lastAccess)
}

func (h cacheHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *cacheHeap) Push(x interface{}) {
	n := len(*h)
	entry := x.(*heapEntry)
	entry.index = n
	*h = append(*h, entry)
}

func (h *cacheHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil // avoid memory leak
	entry.index = -1
	*h = old[0 : n-1]
	return entry
}
