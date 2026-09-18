package crossroom

import (
	"container/list"
	"sync"
)

// DefaultCacheBytes is the default byte budget for cached group
// responses. WASM callers should pass a far smaller budget; see
// docs/wasm-memory-budget.md.
const DefaultCacheBytes int64 = 256 << 20

// cacheKey identifies one cached room-group transfer function.
//
// It is keyed on the group's signature rather than its GroupID because opening
// a portal renumbers the groups: an ID-keyed entry would miss on every group in
// the building, including the ones that did not change.
type cacheKey struct {
	GroupSignature uint64
	// EndpointHash covers the source and receiver placement and the band spec.
	// It deliberately is NOT the whole-scene geometry hash: that changes on
	// every portal toggle, which would make the cache miss on exactly the
	// groups it exists to keep warm.
	EndpointHash uint64
	ConfigHash   uint64
	FromKind     uint8
	ToKind       uint8
	FromIndex    int
	ToIndex      int
}

// CacheStats reports cache behaviour, which the dynamic benchmark records.
type CacheStats struct {
	Hits      int
	Misses    int
	Evictions int
	Bytes     int64
}

// ResponseCache is a byte-budgeted LRU over simulated room-group
// responses. It is safe for concurrent use.
type ResponseCache struct {
	mu       sync.Mutex
	maxBytes int64
	bytes    int64
	order    *list.List
	entries  map[cacheKey]*list.Element
	stats    CacheStats
}

type cacheEntry struct {
	key    cacheKey
	factor *groupFactor
	bytes  int64
}

// NewResponseCache creates a cache with a byte budget. A budget of zero or
// less selects DefaultCacheBytes.
func NewResponseCache(maxBytes int64) *ResponseCache {
	if maxBytes <= 0 {
		maxBytes = DefaultCacheBytes
	}

	return &ResponseCache{
		maxBytes: maxBytes,
		order:    list.New(),
		entries:  map[cacheKey]*list.Element{},
	}
}

// Stats returns a snapshot of the cache counters.
func (c *ResponseCache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	stats := c.stats
	stats.Bytes = c.bytes

	return stats
}

// get returns a cached factor and marks it most recently used.
func (c *ResponseCache) get(key cacheKey) (*groupFactor, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.entries[key]
	if !ok {
		c.stats.Misses++

		return nil, false
	}

	c.order.MoveToFront(element)
	c.stats.Hits++

	entry, _ := element.Value.(*cacheEntry)

	return entry.factor, true
}

// put stores a factor, evicting least recently used entries to stay in budget.
func (c *ResponseCache) put(key cacheKey, factor *groupFactor) {
	if c == nil || factor == nil {
		return
	}

	size := groupFactorBytes(factor)

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[key]; ok {
		entry, _ := element.Value.(*cacheEntry)
		c.bytes += size - entry.bytes
		entry.factor = factor
		entry.bytes = size

		c.order.MoveToFront(element)
		c.evictLocked()

		return
	}

	entry := &cacheEntry{key: key, factor: factor, bytes: size}
	c.entries[key] = c.order.PushFront(entry)
	c.bytes += size

	c.evictLocked()
}

// invalidateSignature drops every entry belonging to a room group and returns
// how many were removed.
func (c *ResponseCache) invalidateSignature(signature uint64) int {
	if c == nil {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0

	for element := c.order.Front(); element != nil; {
		next := element.Next()

		entry, _ := element.Value.(*cacheEntry)
		if entry.key.GroupSignature == signature {
			c.removeLocked(element)

			removed++
		}

		element = next
	}

	return removed
}

func (c *ResponseCache) evictLocked() {
	for c.bytes > c.maxBytes {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}

		c.removeLocked(oldest)
		c.stats.Evictions++
	}
}

func (c *ResponseCache) removeLocked(element *list.Element) {
	entry, _ := element.Value.(*cacheEntry)
	c.order.Remove(element)
	delete(c.entries, entry.key)
	c.bytes -= entry.bytes
}

// groupFactorBytes estimates a factor's footprint from its sample storage,
// which dominates everything else it holds.
func groupFactorBytes(factor *groupFactor) int64 {
	const bytesPerSample = 8

	total := int64(0)

	if factor.Early != nil {
		total += int64(factor.Early.BandCount()) * int64(factor.Early.Len()) * bytesPerSample
	}

	if factor.LateEnergy != nil {
		total += int64(len(factor.LateEnergy.Bins)) * int64(factor.LateEnergy.BandCount) * bytesPerSample
	}

	// One Event is a handful of floats plus its band gain slice.
	for _, event := range factor.Events {
		total += int64(len(event.BandGain))*bytesPerSample + 64
	}

	return total
}
