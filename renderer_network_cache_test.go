package algoacoustics

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/ir"
)

func cacheTestFactor(bandCount, length int) *GroupFactor {
	return &GroupFactor{Early: ir.NewBandedResponse(48000, bandCount, length)}
}

func TestGroupResponseCacheRoundTrips(t *testing.T) {
	t.Parallel()

	cache := NewGroupResponseCache(0)
	key := GroupResponseKey{GroupSignature: 1}

	if _, ok := cache.Get(key); ok {
		t.Fatal("an empty cache returned a hit")
	}

	cache.Put(key, cacheTestFactor(6, 128))

	got, ok := cache.Get(key)
	if !ok || got == nil {
		t.Fatal("the stored factor was not returned")
	}

	stats := cache.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("stats = %+v, want one hit and one miss", stats)
	}

	if stats.Bytes <= 0 {
		t.Fatal("the cache reports no bytes held")
	}
}

func TestGroupResponseCacheEvictsByByteBudget(t *testing.T) {
	t.Parallel()

	// Each factor holds 6 bands x 128 samples x 8 bytes = 6144 bytes, so a
	// 16 KiB budget admits two and evicts on the third.
	cache := NewGroupResponseCache(16 << 10)

	for index := range 3 {
		cache.Put(GroupResponseKey{GroupSignature: uint64(index + 1)}, cacheTestFactor(6, 128))
	}

	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Fatal("the byte budget was exceeded without any eviction")
	}

	if stats.Bytes > 16<<10 {
		t.Fatalf("cache holds %d bytes, above the 16384 budget", stats.Bytes)
	}

	// The oldest entry must be the one that went.
	if _, ok := cache.Get(GroupResponseKey{GroupSignature: 1}); ok {
		t.Fatal("the least recently used entry survived eviction")
	}

	if _, ok := cache.Get(GroupResponseKey{GroupSignature: 3}); !ok {
		t.Fatal("the most recently stored entry was evicted")
	}
}

func TestGroupResponseCacheKeepsRecentlyUsedEntries(t *testing.T) {
	t.Parallel()

	cache := NewGroupResponseCache(16 << 10)

	first := GroupResponseKey{GroupSignature: 1}
	second := GroupResponseKey{GroupSignature: 2}

	cache.Put(first, cacheTestFactor(6, 128))
	cache.Put(second, cacheTestFactor(6, 128))

	// Touching the first entry must move it ahead of the second.
	if _, ok := cache.Get(first); !ok {
		t.Fatal("the first entry is missing")
	}

	cache.Put(GroupResponseKey{GroupSignature: 3}, cacheTestFactor(6, 128))

	if _, ok := cache.Get(first); !ok {
		t.Fatal("a recently used entry was evicted before an older one")
	}
}

func TestGroupResponseCacheInvalidateSignature(t *testing.T) {
	t.Parallel()

	cache := NewGroupResponseCache(0)

	cache.Put(GroupResponseKey{GroupSignature: 7, FromIndex: 0}, cacheTestFactor(2, 16))
	cache.Put(GroupResponseKey{GroupSignature: 7, FromIndex: 1}, cacheTestFactor(2, 16))
	cache.Put(GroupResponseKey{GroupSignature: 9}, cacheTestFactor(2, 16))

	if removed := cache.InvalidateSignature(7); removed != 2 {
		t.Fatalf("InvalidateSignature removed %d entries, want 2", removed)
	}

	if _, ok := cache.Get(GroupResponseKey{GroupSignature: 9}); !ok {
		t.Fatal("an unrelated group was invalidated")
	}
}

func TestGroupResponseCacheHandlesNilReceiver(t *testing.T) {
	t.Parallel()

	var cache *GroupResponseCache

	if _, ok := cache.Get(GroupResponseKey{}); ok {
		t.Fatal("a nil cache reported a hit")
	}

	cache.Put(GroupResponseKey{}, cacheTestFactor(1, 1))

	if got := cache.InvalidateSignature(1); got != 0 {
		t.Fatalf("a nil cache invalidated %d entries", got)
	}

	if got := cache.Stats(); got != (CacheStats{}) {
		t.Fatalf("a nil cache reported %+v", got)
	}
}

func TestConfigHashReactsToSettingsThatChangeAResponse(t *testing.T) {
	t.Parallel()

	cfg := ir.RenderConfig{SampleRate: 48000, DurationSeconds: 1}
	base := NewNetworkRenderer(dynamicTestConfig())

	rays := NewNetworkRenderer(dynamicTestConfig())
	rays.Config.Raytrace.Launch.NumRays = 9999

	order := NewNetworkRenderer(dynamicTestConfig())
	order.Config.ISM.MaxOrder = 5

	if base.configHash(cfg) == rays.configHash(cfg) {
		t.Fatal("changing the ray count did not change the config hash")
	}

	if base.configHash(cfg) == order.configHash(cfg) {
		t.Fatal("changing the image-source order did not change the config hash")
	}

	longer := ir.RenderConfig{SampleRate: 48000, DurationSeconds: 2}
	if base.configHash(cfg) == base.configHash(longer) {
		t.Fatal("changing the duration did not change the config hash")
	}
}
