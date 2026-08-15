package algoacoustics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
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

// TestConfigHashCoversEverySolverSetting is the guard for the documented
// shared-cache use case: a renderer must never be handed a factor another
// renderer simulated with different solver behaviour.
func TestConfigHashCoversEverySolverSetting(t *testing.T) {
	t.Parallel()

	cfg := ir.RenderConfig{SampleRate: 48000, DurationSeconds: 1}
	base := NewNetworkRenderer(dynamicTestConfig()).configHash(cfg)

	cases := []struct {
		name   string
		mutate func(*NetworkRendererConfig)
	}{
		{"ISM diffraction", func(c *NetworkRendererConfig) { c.ISM.EnableDiffraction = true }},
		{"ISM diffraction order", func(c *NetworkRendererConfig) { c.ISM.MaxDiffractionOrder = 2 }},
		{"ray time limit", func(c *NetworkRendererConfig) { c.Raytrace.Launch.MaxTimeSeconds = 3 }},
		{"diffuse rain", func(c *NetworkRendererConfig) { c.Raytrace.Launch.DiffuseRain = true }},
		{"ray speed of sound", func(c *NetworkRendererConfig) { c.Raytrace.Launch.SpeedOfSound = 340 }},
		{"energy termination", func(c *NetworkRendererConfig) { c.Raytrace.Launch.EnergyTerminationThreshold = 1e-7 }},
		{"reflection strategy", func(c *NetworkRendererConfig) { c.Raytrace.Launch.ReflectionStrategy = 1 }},
		{"direction groups", func(c *NetworkRendererConfig) { c.Raytrace.DirectionGroupAzimuth = 12 }},
		{"path hops", func(c *NetworkRendererConfig) { c.MaxPathHops = 6 }},
		{"path count", func(c *NetworkRendererConfig) { c.MaxPaths = 3 }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			changed := dynamicTestConfig()
			testCase.mutate(&changed)

			if NewNetworkRenderer(changed).configHash(cfg) == base {
				t.Fatalf("changing %s did not change the config hash", testCase.name)
			}
		})
	}
}

// TestEndpointHashCoversSourceAndReceiverAttributes pins the other half of the
// key. The ISM solver applies the source pattern along its orientation, and the
// terminal late factor spatializes through the receiver's HRTF, so an
// orientation-blind key would return a response aimed the wrong way.
func TestEndpointHashCoversSourceAndReceiverAttributes(t *testing.T) {
	t.Parallel()

	turned := geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2)

	cases := []struct {
		name   string
		mutate func(*scene.Scene)
	}{
		{"source position", func(s *scene.Scene) { s.Sources[0].Position.X += 0.5 }},
		{"source orientation", func(s *scene.Scene) { s.Sources[0].Orientation = turned }},
		{"source gain", func(s *scene.Scene) { s.Sources[0].GainDB = 6 }},
		{"source directivity", func(s *scene.Scene) {
			s.Sources[0].Directivity = directivity.CardioidModel{Axis: geometry.Vec3{X: 1}, OrderN: 1}
		}},
		{"receiver position", func(s *scene.Scene) { s.Receivers[0].Position.Y += 0.5 }},
		{"receiver orientation", func(s *scene.Scene) { s.Receivers[0].Orientation = turned }},
		{"receiver HRTF", func(s *scene.Scene) { s.Receivers[0].HRTF = hrtf.NoopDataset{SampleRateHz: 48000} }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			base := chainRoomScene(t, 2, 0.25)
			changed := chainRoomScene(t, 2, 0.25)
			testCase.mutate(changed)

			if endpointHash(base) == endpointHash(changed) {
				t.Fatalf("changing the %s did not change the endpoint hash", testCase.name)
			}
		})
	}
}

// TestEndpointHashIsStableForAnUnchangedScene keeps the key from churning:
// hashing a reference type by identity is only acceptable because the same
// scene keeps the same objects.
func TestEndpointHashIsStableForAnUnchangedScene(t *testing.T) {
	t.Parallel()

	dataset := &hrtf.NoopDataset{SampleRateHz: 48000}

	first := chainRoomScene(t, 2, 0.25)
	first.Receivers[0].HRTF = dataset

	// A second scene with the same endpoints and the same dataset object must
	// key identically, or a portal toggle would miss on every group.
	second := chainRoomScene(t, 2, 0.25)
	second.Receivers[0].HRTF = dataset

	if endpointHash(first) != endpointHash(second) {
		t.Fatal("two identical scenes produced different endpoint hashes")
	}
}
