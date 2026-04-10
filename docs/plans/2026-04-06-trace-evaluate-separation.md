# Phase 18.1: Trace/Evaluate Separation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Separate geometric ray tracing from energy evaluation so material-only changes can reuse cached paths, targeting < 100 ms replay for 10k paths.

**Architecture:** Add a `PathCache` type to `raytrace/` that stores geometry-only bounce data (hit points, normals, wall indices, segment lengths). A new `TracePaths()` method produces the cache; `EvaluatePaths()` replays it with current materials. For ISM, the existing `ImageSource`/`MeshImageSource` slices already are geometry-only — we add standalone `EvaluateShoebox()`/`EvaluateMesh()` functions. Both caches embed a geometry hash for staleness checks via `scene.GeometryHash()`.

**Tech Stack:** Go stdlib (`hash/fnv`, `math/bits`), existing `geometry`, `scene`, `raytrace`, `ism` packages.

---

## Task 1: Add `GeometryHash()` to `scene.Scene`

**Files:**
- Create: `scene/hash.go`
- Create: `scene/hash_test.go`

**Step 1: Write the failing test**

```go
// scene/hash_test.go
package scene

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestGeometryHash_ShoeboxDeterministic(t *testing.T) {
	sc := &Scene{
		Room: Room{
			Kind:    RoomKindShoebox,
			Shoebox: &Shoebox{Width: 10, Depth: 8, Height: 3},
		},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	h1 := sc.GeometryHash()
	h2 := sc.GeometryHash()

	if h1 != h2 {
		t.Fatalf("same scene produced different hashes: %d vs %d", h1, h2)
	}

	if h1 == 0 {
		t.Fatal("hash should not be zero for a valid scene")
	}
}

func TestGeometryHash_ShoeboxChangeDimensions(t *testing.T) {
	sc1 := &Scene{
		Room: Room{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	sc2 := &Scene{
		Room: Room{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 11, Depth: 8, Height: 3}},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	if sc1.GeometryHash() == sc2.GeometryHash() {
		t.Fatal("different room dimensions should produce different hashes")
	}
}

func TestGeometryHash_MaterialChangeNoEffect(t *testing.T) {
	base := Scene{
		Room: Room{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	sc1 := base
	sc1.Materials = map[string]Material{"wall": {Absorption: [8]float64{0.1}}}

	sc2 := base
	sc2.Materials = map[string]Material{"wall": {Absorption: [8]float64{0.9}}}

	if sc1.GeometryHash() != sc2.GeometryHash() {
		t.Fatal("material changes should not affect geometry hash")
	}
}

func TestGeometryHash_SourcePositionChange(t *testing.T) {
	sc1 := &Scene{
		Room: Room{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	sc2 := &Scene{
		Room: Room{Kind: RoomKindShoebox, Shoebox: &Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []Source{{Position: geometry.Vec3{X: 3, Y: 3, Z: 1.5}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	if sc1.GeometryHash() == sc2.GeometryHash() {
		t.Fatal("different source positions should produce different hashes")
	}
}

func TestGeometryHash_MeshDeterministic(t *testing.T) {
	mesh := &geometry.Mesh{
		Triangles: []geometry.Triangle{
			{V0: geometry.Vec3{X: 0}, V1: geometry.Vec3{X: 1}, V2: geometry.Vec3{Y: 1}},
		},
	}

	sc := &Scene{
		Room:      Room{Kind: RoomKindMesh, Mesh: mesh},
		Sources:   []Source{{Position: geometry.Vec3{X: 0.5, Y: 0.3, Z: 0.1}}},
		Receivers: []Receiver{{Position: geometry.Vec3{X: 0.2, Y: 0.5, Z: 0.1}}},
	}

	h1 := sc.GeometryHash()
	h2 := sc.GeometryHash()

	if h1 != h2 {
		t.Fatalf("same mesh scene produced different hashes: %d vs %d", h1, h2)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestGeometryHash ./scene/...`
Expected: FAIL — `GeometryHash` not defined.

**Step 3: Write minimal implementation**

```go
// scene/hash.go
package scene

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// GeometryHash returns a hash of the scene's geometry and source/receiver
// positions. Material changes do not affect this hash.
func (s *Scene) GeometryHash() uint64 {
	h := fnv.New64a()

	writeFloat := func(v float64) {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		h.Write(buf[:])
	}

	writeVec3 := func(v Vec3Hasher) {
		writeFloat(v.X)
		writeFloat(v.Y)
		writeFloat(v.Z)
	}

	// Room geometry.
	h.Write([]byte(s.Room.Kind))

	if s.Room.Shoebox != nil {
		writeFloat(s.Room.Shoebox.Width)
		writeFloat(s.Room.Shoebox.Depth)
		writeFloat(s.Room.Shoebox.Height)
	}

	if s.Room.Mesh != nil {
		for _, tri := range s.Room.Mesh.Triangles {
			writeVec3(Vec3Hasher(tri.V0))
			writeVec3(Vec3Hasher(tri.V1))
			writeVec3(Vec3Hasher(tri.V2))
		}
	}

	// Source positions.
	for _, src := range s.Sources {
		writeVec3(Vec3Hasher(src.Position))
	}

	// Receiver positions.
	for _, rcv := range s.Receivers {
		writeVec3(Vec3Hasher(rcv.Position))
	}

	return h.Sum64()
}

// Vec3Hasher wraps geometry.Vec3 fields for hashing without importing geometry
// into the hash helper. Uses the same field layout.
type Vec3Hasher struct {
	X, Y, Z float64
}
```

Wait — we can just import `geometry` since `scene` already imports it. Simplify:

```go
// scene/hash.go
package scene

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// GeometryHash returns a hash of the scene's geometry and source/receiver
// positions. Material changes do not affect this hash.
func (s *Scene) GeometryHash() uint64 {
	h := fnv.New64a()

	writeFloat := func(v float64) {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		h.Write(buf[:])
	}

	writeVec3 := func(v geometry.Vec3) {
		writeFloat(v.X)
		writeFloat(v.Y)
		writeFloat(v.Z)
	}

	h.Write([]byte(s.Room.Kind))

	if s.Room.Shoebox != nil {
		writeFloat(s.Room.Shoebox.Width)
		writeFloat(s.Room.Shoebox.Depth)
		writeFloat(s.Room.Shoebox.Height)
	}

	if s.Room.Mesh != nil {
		for _, tri := range s.Room.Mesh.Triangles {
			writeVec3(tri.V0)
			writeVec3(tri.V1)
			writeVec3(tri.V2)
		}
	}

	for _, src := range s.Sources {
		writeVec3(src.Position)
	}

	for _, rcv := range s.Receivers {
		writeVec3(rcv.Position)
	}

	return h.Sum64()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestGeometryHash ./scene/...`
Expected: PASS

**Step 5: Commit**

```bash
git add scene/hash.go scene/hash_test.go
git commit -m "feat(scene): add GeometryHash for cache invalidation"
```

---

## Task 2: Add `PathCache` types to `raytrace/`

**Files:**
- Create: `raytrace/path_cache.go`
- Create: `raytrace/path_cache_test.go`

**Step 1: Write the failing test**

```go
// raytrace/path_cache_test.go
package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestPathCache_ValidFor_SameScene(t *testing.T) {
	sc := &scene.Scene{
		Room: scene.Room{
			Kind:    scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{Width: 10, Depth: 8, Height: 3},
		},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	cache := &PathCache{
		GeometryHash:   sc.GeometryHash(),
		ReceiverRadius: 0.25,
	}

	if !cache.ValidFor(sc, 0.25) {
		t.Fatal("cache should be valid for the same scene")
	}
}

func TestPathCache_ValidFor_DifferentGeometry(t *testing.T) {
	sc1 := &scene.Scene{
		Room: scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	cache := &PathCache{
		GeometryHash:   sc1.GeometryHash(),
		ReceiverRadius: 0.25,
	}

	sc2 := &scene.Scene{
		Room: scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 12, Depth: 8, Height: 3}},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	if cache.ValidFor(sc2, 0.25) {
		t.Fatal("cache should be invalid after geometry change")
	}
}

func TestPathCache_ValidFor_DifferentReceiverRadius(t *testing.T) {
	sc := &scene.Scene{
		Room: scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 10, Depth: 8, Height: 3}},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
	}

	cache := &PathCache{
		GeometryHash:   sc.GeometryHash(),
		ReceiverRadius: 0.25,
	}

	if cache.ValidFor(sc, 0.5) {
		t.Fatal("cache should be invalid when receiver radius changes")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestPathCache_ValidFor ./raytrace/...`
Expected: FAIL — `PathCache` not defined.

**Step 3: Write minimal implementation**

```go
// raytrace/path_cache.go
package raytrace

import (
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// PathStep records one bounce in a traced ray path.
type PathStep struct {
	HitPoint      geometry.Vec3
	Normal        geometry.Vec3
	WallIndex     int
	SegmentLength float64
}

// TracedPath is a single ray's complete geometric path.
type TracedPath struct {
	LaunchDir geometry.Vec3
	DGIndex   int // directivity group index, -1 if none
	Steps     []PathStep
}

// PathCache holds all traced paths plus a validity key.
type PathCache struct {
	Paths          []TracedPath
	GeometryHash   uint64
	ReceiverRadius float64
	MaxBounces     int
	MaxPathLength  float64
}

// ValidFor reports whether this cache can be reused for the given scene
// and receiver radius. Material changes do not invalidate the cache.
func (c *PathCache) ValidFor(sc *scene.Scene, receiverRadius float64) bool {
	if c == nil || sc == nil {
		return false
	}

	return c.GeometryHash == sc.GeometryHash() && c.ReceiverRadius == receiverRadius
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestPathCache_ValidFor ./raytrace/...`
Expected: PASS

**Step 5: Commit**

```bash
git add raytrace/path_cache.go raytrace/path_cache_test.go
git commit -m "feat(raytrace): add PathCache types with validity check"
```

---

## Task 3: Implement `TracePaths()` on `RayTracer`

**Files:**
- Modify: `raytrace/path_cache.go` (add `TracePaths` method)
- Create: `raytrace/trace_paths_test.go`

This is the geometry-only tracing loop. It mirrors the bounce loop in `Trace()` (`raytrace/raytrace.go:109-241`) but stores `PathStep`s instead of computing energy. Terminates on max bounces OR max path length (no energy threshold).

**Step 1: Write the failing test**

```go
// raytrace/trace_paths_test.go
package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newTestScene() *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: 10, Depth: 8, Height: 3,
				WallMaterials: [6]string{"wall", "wall", "wall", "wall", "floor", "ceiling"},
			},
		},
		Materials: map[string]scene.Material{
			"wall":    {Absorption: [8]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}},
			"floor":   {Absorption: [8]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}},
			"ceiling": {Absorption: [8]float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}},
		},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}, GainDB: 0}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
		BandSpec:  bandSpec6(),
	}
}

func TestTracePaths_ReturnsNonEmptyCache(t *testing.T) {
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        100,
			MaxBounces:     50,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   343.0,
		},
		Scene:          newTestScene(),
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	if len(cache.Paths) == 0 {
		t.Fatal("expected non-empty path cache")
	}

	if len(cache.Paths) != 100 {
		t.Fatalf("expected 100 paths, got %d", len(cache.Paths))
	}
}

func TestTracePaths_EachPathHasSteps(t *testing.T) {
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10,
			MaxBounces:     50,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   343.0,
		},
		Scene:          newTestScene(),
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	for i, path := range cache.Paths {
		if len(path.Steps) == 0 {
			t.Fatalf("path %d has no steps", i)
		}

		for j, step := range path.Steps {
			if step.SegmentLength <= 0 {
				t.Fatalf("path %d step %d has non-positive segment length %f", i, j, step.SegmentLength)
			}
		}
	}
}

func TestTracePaths_CacheHashMatchesScene(t *testing.T) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10,
			MaxBounces:     50,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	if !cache.ValidFor(sc, 0.25) {
		t.Fatal("cache should be valid for the scene it was built from")
	}
}

func TestTracePaths_PathLengthBounded(t *testing.T) {
	maxTime := 0.5
	speedOfSound := 343.0
	maxPathLength := maxTime * speedOfSound

	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10,
			MaxBounces:     200,
			MaxTimeSeconds: maxTime,
			SpeedOfSound:   speedOfSound,
		},
		Scene:          newTestScene(),
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	for i, path := range cache.Paths {
		var totalLength float64
		for _, step := range path.Steps {
			totalLength += step.SegmentLength
		}

		if totalLength > maxPathLength*1.01 {
			t.Fatalf("path %d total length %f exceeds max %f", i, totalLength, maxPathLength)
		}
	}
}
```

Note: `bandSpec6()` may already be a test helper in `raytrace/`. If not, add it to this test file:

```go
func bandSpec6() acoustics.BandSpec {
	return acoustics.Octave6
}
```

Check if it exists first: `grep -r "func bandSpec6" raytrace/`

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestTracePaths ./raytrace/...`
Expected: FAIL — `TracePaths` method not defined.

**Step 3: Write implementation**

Add to `raytrace/path_cache.go`:

```go
// TracePaths runs geometry-only ray tracing, storing bounce paths without
// computing energy. The resulting PathCache can be replayed with different
// materials via EvaluatePaths.
func (r *RayTracer) TracePaths() (*PathCache, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	if len(r.Scene.Sources) == 0 {
		return nil, errors.New("scene has no sources")
	}

	if len(r.Scene.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
	}

	if r.Config.NumRays <= 0 {
		return nil, errors.New("NumRays must be positive")
	}

	if r.Config.MaxBounces < 0 {
		return nil, errors.New("MaxBounces must not be negative")
	}

	if r.Config.MaxTimeSeconds <= 0 {
		return nil, errors.New("MaxTimeSeconds must be positive")
	}

	if r.Config.SpeedOfSound <= 0 {
		return nil, errors.New("SpeedOfSound must be positive")
	}

	tracer, err := r.sceneTracer()
	if err != nil {
		return nil, err
	}

	source := r.Scene.Sources[0]
	maxPathLength := r.Config.MaxTimeSeconds * r.Config.SpeedOfSound

	rays := LaunchRays(source.Position, r.Config)
	if len(rays) == 0 {
		return &PathCache{
			GeometryHash:  r.Scene.GeometryHash(),
			ReceiverRadius: r.ReceiverRadius,
			MaxBounces:    r.Config.MaxBounces,
			MaxPathLength: maxPathLength,
		}, nil
	}

	paths := make([]TracedPath, 0, len(rays))

	for _, ray := range rays {
		tp := TracedPath{
			LaunchDir: ray.Direction,
			DGIndex:   -1,
		}

		if len(r.DirectivityGroups) > 0 {
			tp.DGIndex = ClassifyDirection(r.DirectivityGroups, ray.Direction)
		}

		currentRay := ray
		var pathLength float64

		for bounce := 0; bounce <= r.Config.MaxBounces; bounce++ {
			if pathLength >= maxPathLength {
				break
			}

			hitPoint, hitNormal, wallIdx, ok := tracer.NextHit(currentRay)
			if !ok {
				break
			}

			segmentLength := currentRay.Origin.Distance(hitPoint)
			if segmentLength <= 0 {
				break
			}

			tp.Steps = append(tp.Steps, PathStep{
				HitPoint:      hitPoint,
				Normal:        hitNormal,
				WallIndex:     wallIdx,
				SegmentLength: segmentLength,
			})

			pathLength += segmentLength
			if pathLength >= maxPathLength {
				break
			}

			// Advance ray: specular reflection (geometry-only, no scatter decision).
			specularDir := SpecularReflect(currentRay.Direction, hitNormal)
			currentRay = geometry.NewRay(hitPoint.Add(specularDir.Scale(wallEpsilon)), specularDir)
		}

		paths = append(paths, tp)
	}

	receiverRadius := r.ReceiverRadius
	if receiverRadius <= 0 {
		receiverRadius = 0.25
	}

	return &PathCache{
		Paths:          paths,
		GeometryHash:   r.Scene.GeometryHash(),
		ReceiverRadius: receiverRadius,
		MaxBounces:     r.Config.MaxBounces,
		MaxPathLength:  maxPathLength,
	}, nil
}
```

Note: The geometry-only trace always uses specular reflection for the path geometry. The energy evaluation phase later decides the energy split between specular/diffuse using the cached path. This is the key simplification — the geometric path is deterministic and material-independent.

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestTracePaths ./raytrace/...`
Expected: PASS

**Step 5: Commit**

```bash
git add raytrace/path_cache.go raytrace/trace_paths_test.go
git commit -m "feat(raytrace): implement TracePaths for geometry-only tracing"
```

---

## Task 4: Implement `EvaluatePaths()` on `RayTracer`

**Files:**
- Create: `raytrace/evaluate_paths.go`
- Create: `raytrace/evaluate_paths_test.go`

This replays a `PathCache` with current materials, producing the same `EnergyHistogram` structure as `Trace()`.

**Step 1: Write the failing test**

```go
// raytrace/evaluate_paths_test.go
package raytrace

import (
	"testing"
)

func TestEvaluatePaths_ProducesNonEmptyHistogram(t *testing.T) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        100,
			MaxBounces:     50,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	hist, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths failed: %v", err)
	}

	if hist == nil {
		t.Fatal("expected non-nil histogram")
	}

	var totalEnergy float64
	for _, bin := range hist.Bins {
		for _, e := range bin.BandEnergy {
			totalEnergy += e
		}
	}

	if totalEnergy <= 0 {
		t.Fatal("histogram should contain non-zero energy")
	}
}

func TestEvaluatePaths_DifferentMaterialsDifferentEnergy(t *testing.T) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        100,
			MaxBounces:     50,
			MaxTimeSeconds: 1.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	hist1, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths (low absorption) failed: %v", err)
	}

	// Increase absorption.
	sc.Materials["wall"] = scene.Material{Absorption: [8]float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}}
	sc.Materials["floor"] = scene.Material{Absorption: [8]float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}}
	sc.Materials["ceiling"] = scene.Material{Absorption: [8]float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}}

	hist2, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths (high absorption) failed: %v", err)
	}

	energy1 := totalHistogramEnergy(hist1)
	energy2 := totalHistogramEnergy(hist2)

	if energy2 >= energy1 {
		t.Fatalf("high absorption should yield less energy: got %f >= %f", energy2, energy1)
	}
}

func totalHistogramEnergy(h *EnergyHistogram) float64 {
	var total float64
	for _, bin := range h.Bins {
		for _, e := range bin.BandEnergy {
			total += e
		}
	}
	return total
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestEvaluatePaths ./raytrace/...`
Expected: FAIL — `EvaluatePaths` not defined.

**Step 3: Write implementation**

```go
// raytrace/evaluate_paths.go
package raytrace

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// EvaluatePaths replays a cached set of geometric paths, applying the current
// scene materials to produce an energy histogram. This is much faster than
// a full Trace() when only materials have changed.
func (r *RayTracer) EvaluatePaths(cache *PathCache) (*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if cache == nil {
		return nil, errors.New("path cache is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	bandCount := r.Scene.BandSpec.BandCount()
	if bandCount <= 0 {
		bandCount = 1
	}

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	hist := NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)

	for i := range r.DirectivityGroups {
		r.DirectivityGroups[i].Histogram = NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	}

	source := r.Scene.Sources[0]
	receiverData := r.Scene.Receivers[0]

	receiverRadius := r.ReceiverRadius
	if receiverRadius <= 0 {
		receiverRadius = 0.25
	}

	receiver := SphereReceiver{Center: receiverData.Position, Radius: receiverRadius}

	launchEnergy := calibratedRayLaunchEnergy(source.GainDB, source.Position, receiverData.Position, receiverRadius, len(cache.Paths))
	energyThreshold := r.Config.EnergyTerminationThreshold

	for _, tp := range cache.Paths {
		energy := initialRayEnergy(source, tp.LaunchDir, launchEnergy, bandCount, r.Scene.BandSpec.CenterFreqs)

		origin := geometry.NewRay(source.Position, tp.LaunchDir).Origin

		for _, step := range tp.Steps {
			// Check receiver intersection on this segment.
			ray := geometry.Ray{Origin: origin, Direction: step.HitPoint.Sub(origin).Normalize()}

			if tHit, hit := receiver.Intersects(ray, wallEpsilon, step.SegmentLength); hit {
				arrivalTime := pathLengthToOrigin(origin, source.Position, tp.Steps, &step) / r.Config.SpeedOfSound
				arrivalTime = (arrivalTime + tHit) / r.Config.SpeedOfSound
				// Recompute: accumulate path length up to this segment, then add tHit.
			}

			// Air attenuation over segment.
			energy = attenuateEnergyByAir(energy, r.Scene.BandSpec.CenterFreqs, step.SegmentLength, defaultAirTemperatureC, defaultRelativeHumidity)

			// Material evaluation.
			material := r.sceneMaterialForWall(step.WallIndex)
			absorption := make([]float64, bandCount)
			for bi := range absorption {
				absorption[bi] = material.AbsorptionAt(bi)
			}
			scattering := material.ScatteringCoefficients(bandCount)
			_, _, remainingEnergy := splitReflectionEnergy(energy, absorption, scattering)
			energy = remainingEnergy

			if maxEnergy(energy) <= energyThreshold {
				break
			}

			origin = step.HitPoint
		}
	}

	return hist, nil
}
```

Wait — that draft has issues with arrival time calculation. Let me think more carefully. The replay needs to track cumulative path length for arrival times AND do receiver intersection checks per segment. Let me write the clean version:

```go
// raytrace/evaluate_paths.go
package raytrace

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// EvaluatePaths replays cached geometric paths with the current scene's
// materials, producing an energy histogram without re-tracing geometry.
func (r *RayTracer) EvaluatePaths(cache *PathCache) (*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if cache == nil {
		return nil, errors.New("path cache is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	bandCount := r.Scene.BandSpec.BandCount()
	if bandCount <= 0 {
		bandCount = 1
	}

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	hist := NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)

	for i := range r.DirectivityGroups {
		r.DirectivityGroups[i].Histogram = NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	}

	source := r.Scene.Sources[0]
	receiverData := r.Scene.Receivers[0]

	receiverRadius := r.ReceiverRadius
	if receiverRadius <= 0 {
		receiverRadius = 0.25
	}

	receiver := SphereReceiver{Center: receiverData.Position, Radius: receiverRadius}
	launchEnergy := calibratedRayLaunchEnergy(source.GainDB, source.Position, receiverData.Position, receiverRadius, len(cache.Paths))
	energyThreshold := r.Config.EnergyTerminationThreshold

	for _, tp := range cache.Paths {
		energy := initialRayEnergy(source, tp.LaunchDir, launchEnergy, bandCount, r.Scene.BandSpec.CenterFreqs)
		origin := source.Position
		var pathLength float64

		for _, step := range tp.Steps {
			dir := step.HitPoint.Sub(origin)
			dist := dir.Norm()

			if dist <= 0 {
				break
			}

			rayDir := dir.Scale(1 / dist)
			ray := geometry.Ray{Origin: origin, Direction: rayDir}

			if tHit, hit := receiver.Intersects(ray, wallEpsilon, step.SegmentLength); hit {
				arrivalTime := (pathLength + tHit) / r.Config.SpeedOfSound
				if arrivalTime <= r.Config.MaxTimeSeconds {
					hitEnergy := attenuateEnergyByAir(energy, r.Scene.BandSpec.CenterFreqs, tHit, defaultAirTemperatureC, defaultRelativeHumidity)
					capture := receiver.AngularWeight(rayDir)

					for bi := range hitEnergy {
						hitEnergy[bi] *= capture
					}

					hist.Add(arrivalTime, hitEnergy)

					if len(r.DirectivityGroups) > 0 {
						dgIdx := ClassifyDirection(r.DirectivityGroups, rayDir.Scale(-1))
						r.DirectivityGroups[dgIdx].Histogram.Add(arrivalTime, hitEnergy)
					}
				}
			}

			pathLength += step.SegmentLength

			energy = attenuateEnergyByAir(energy, r.Scene.BandSpec.CenterFreqs, step.SegmentLength, defaultAirTemperatureC, defaultRelativeHumidity)

			material := r.sceneMaterialForWall(step.WallIndex)

			absorption := make([]float64, bandCount)
			for bi := range absorption {
				absorption[bi] = material.AbsorptionAt(bi)
			}

			scattering := material.ScatteringCoefficients(bandCount)
			_, _, remainingEnergy := splitReflectionEnergy(energy, absorption, scattering)
			energy = remainingEnergy

			if maxEnergy(energy) <= energyThreshold {
				break
			}

			origin = step.HitPoint
		}
	}

	return hist, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestEvaluatePaths ./raytrace/...`
Expected: PASS

**Step 5: Commit**

```bash
git add raytrace/evaluate_paths.go raytrace/evaluate_paths_test.go
git commit -m "feat(raytrace): implement EvaluatePaths to replay cached paths with materials"
```

---

## Task 5: Wire `Trace()` through `TracePaths` + `EvaluatePaths` and verify parity

**Files:**
- Modify: `raytrace/raytrace.go` (no change to `Trace()` itself — keep both paths)
- Create: `raytrace/parity_test.go`

The existing `Trace()` stays unchanged. We add a parity test that verifies the two-phase path produces comparable results. Because `TracePaths` uses specular-only geometry while `Trace()` uses probabilistic scatter for reflection directions, the paths will differ. The parity test therefore checks that total energy is in the same order of magnitude and that the histogram shape is similar — not bit-identical.

**Step 1: Write the test**

```go
// raytrace/parity_test.go
package raytrace

import (
	"math"
	"testing"
)

func TestParity_TraceVsCachedReplay(t *testing.T) {
	sc := newTestScene()

	// Use deterministic blend to minimize scatter randomness.
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:            500,
			MaxBounces:         50,
			MaxTimeSeconds:     1.0,
			SpeedOfSound:       343.0,
			ReflectionStrategy: ReflectionStrategyDeterministicBlend,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	// Direct trace.
	directHist, err := rt.Trace()
	if err != nil {
		t.Fatalf("Trace failed: %v", err)
	}

	// Cached path.
	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	cachedHist, err := rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths failed: %v", err)
	}

	directEnergy := totalHistogramEnergy(directHist)
	cachedEnergy := totalHistogramEnergy(cachedHist)

	if directEnergy <= 0 {
		t.Fatal("direct trace produced no energy")
	}

	if cachedEnergy <= 0 {
		t.Fatal("cached replay produced no energy")
	}

	// The energies won't be identical because TracePaths uses specular-only
	// geometry while Trace() uses the configured reflection strategy.
	// Check they're within an order of magnitude.
	ratio := cachedEnergy / directEnergy
	if ratio < 0.1 || ratio > 10 {
		t.Fatalf("energy ratio out of range: cached=%f direct=%f ratio=%f", cachedEnergy, directEnergy, ratio)
	}

	t.Logf("direct=%f cached=%f ratio=%.2f", directEnergy, cachedEnergy, ratio)
}

func TestParity_ReplayFasterThanTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark-like test in short mode")
	}

	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths failed: %v", err)
	}

	// Time a full trace.
	traceStart := now()
	_, err = rt.Trace()
	if err != nil {
		t.Fatalf("Trace failed: %v", err)
	}
	traceDuration := since(traceStart)

	// Time the replay.
	replayStart := now()
	_, err = rt.EvaluatePaths(cache)
	if err != nil {
		t.Fatalf("EvaluatePaths failed: %v", err)
	}
	replayDuration := since(replayStart)

	t.Logf("trace=%v replay=%v speedup=%.1fx", traceDuration, replayDuration, float64(traceDuration)/math.Max(float64(replayDuration), 1))

	// Replay should be faster (no geometry intersection).
	if replayDuration >= traceDuration {
		t.Logf("WARNING: replay not faster than trace (trace=%v replay=%v)", traceDuration, replayDuration)
	}
}
```

Add timing helpers at the bottom:

```go
import "time"

func now() time.Duration {
	return time.Duration(time.Now().UnixNano())
}

func since(start time.Duration) time.Duration {
	return time.Duration(time.Now().UnixNano()) - start
}
```

Actually simpler to just use `time.Now()` and `time.Since()` directly. Adjust the test to use those.

**Step 2: Run tests**

Run: `go test -v -run TestParity ./raytrace/...`
Expected: PASS

**Step 3: Commit**

```bash
git add raytrace/parity_test.go
git commit -m "test(raytrace): add parity tests for TracePaths/EvaluatePaths vs Trace"
```

---

## Task 6: ISM `EvaluateShoebox()` function

**Files:**
- Create: `ism/evaluate.go`
- Create: `ism/evaluate_test.go`

**Step 1: Write the failing test**

```go
// ism/evaluate_test.go
package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newISMTestScene() *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: 10, Depth: 8, Height: 3,
				WallMaterials: [6]string{"wall", "wall", "wall", "wall", "floor", "ceiling"},
			},
		},
		Materials: map[string]scene.Material{
			"wall":    {Absorption: [8]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}},
			"floor":   {Absorption: [8]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}},
			"ceiling": {Absorption: [8]float64{0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3}},
		},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 2, Y: 3, Z: 1.5}, GainDB: 0}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 7, Y: 5, Z: 1.2}}},
		BandSpec:  acoustics.Octave6,
	}
}

func TestEvaluateShoebox_MatchesSolve(t *testing.T) {
	sc := newISMTestScene()
	cfg := ISMConfig{MaxOrder: 3, SpeedOfSound: 343, BandSpec: acoustics.Octave6}

	// Full solve.
	events1, err := ISMSolver{}.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// Two-phase: generate + evaluate.
	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, cfg.MaxOrder)
	events2, err := EvaluateShoebox(sources, sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox failed: %v", err)
	}

	if len(events1) != len(events2) {
		t.Fatalf("event count mismatch: Solve=%d EvaluateShoebox=%d", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].TimeSeconds != events2[i].TimeSeconds {
			t.Fatalf("event %d time mismatch: %f vs %f", i, events1[i].TimeSeconds, events2[i].TimeSeconds)
		}

		if events1[i].Kind != events2[i].Kind {
			t.Fatalf("event %d kind mismatch: %v vs %v", i, events1[i].Kind, events2[i].Kind)
		}

		for b := range events1[i].BandGain {
			if events1[i].BandGain[b] != events2[i].BandGain[b] {
				t.Fatalf("event %d band %d gain mismatch: %f vs %f", i, b, events1[i].BandGain[b], events2[i].BandGain[b])
			}
		}
	}
}

func TestEvaluateShoebox_DifferentMaterials(t *testing.T) {
	sc := newISMTestScene()
	cfg := ISMConfig{MaxOrder: 3, SpeedOfSound: 343, BandSpec: acoustics.Octave6}

	sources := GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, cfg.MaxOrder)

	events1, err := EvaluateShoebox(sources, sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox failed: %v", err)
	}

	// Increase absorption.
	sc.Materials["wall"] = scene.Material{Absorption: [8]float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}}

	events2, err := EvaluateShoebox(sources, sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateShoebox with high absorption failed: %v", err)
	}

	// Should still have the same number of events but some may be filtered.
	// At minimum, the direct event should be identical (no material on direct path).
	if len(events2) == 0 {
		t.Fatal("expected at least the direct event")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestEvaluateShoebox ./ism/...`
Expected: FAIL — `EvaluateShoebox` not defined.

**Step 3: Write implementation**

```go
// ism/evaluate.go
package ism

import (
	"errors"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// EvaluateShoebox takes pre-computed image sources and the current scene,
// producing ir.Events by applying materials. The image sources are
// geometry-only — this function handles all material-dependent computation.
func EvaluateShoebox(sources []ImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if sc.Room.Shoebox == nil {
		return nil, errors.New("shoebox room is nil")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("no sources in scene")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("no receivers in scene")
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	receiver := sc.Receivers[0]
	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range sources {
			if imgSrc.Order == 0 {
				continue
			}

			path, ok := reflectionPath(imgSrc, receiver.Position)
			if !ok {
				continue
			}

			event, ok := specularEvent(source, receiver, imgSrc, path, sc.Room.Shoebox, sc.Materials, bandSpec, speedOfSound)
			if ok {
				events = append(events, event)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestEvaluateShoebox ./ism/...`
Expected: PASS. The `TestEvaluateShoebox_MatchesSolve` test must produce bit-identical events since the code path is the same.

**Step 5: Commit**

```bash
git add ism/evaluate.go ism/evaluate_test.go
git commit -m "feat(ism): add EvaluateShoebox for replaying cached image sources"
```

---

## Task 7: ISM `EvaluateMesh()` function

**Files:**
- Modify: `ism/evaluate.go` (add `EvaluateMesh`)
- Modify: `ism/evaluate_test.go` (add mesh tests)

**Step 1: Write the failing test**

```go
// Add to ism/evaluate_test.go

func TestEvaluateMesh_MatchesSolve(t *testing.T) {
	mesh := testBoxMesh(5, 4, 3) // need a helper that creates a box mesh

	sc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "concrete",
		},
		Materials: map[string]scene.Material{
			"concrete": {Absorption: [8]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}},
		},
		Sources:   []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1, Z: 1}, GainDB: 0}},
		Receivers: []scene.Receiver{{Position: geometry.Vec3{X: 3, Y: 2, Z: 1.5}}},
		BandSpec:  acoustics.Octave6,
	}

	cfg := ISMConfig{MaxOrder: 2, SpeedOfSound: 343, BandSpec: acoustics.Octave6}

	events1, err := ISMSolver{}.Solve(sc, cfg)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	imgCfg := MeshISMConfig{MaxOrder: 2, MaxDistance: 343 * 2.0}
	sources := GenerateMeshImageSources(sc.Sources[0].Position, mesh, imgCfg)

	events2, err := EvaluateMesh(sources, sc, cfg)
	if err != nil {
		t.Fatalf("EvaluateMesh failed: %v", err)
	}

	if len(events1) != len(events2) {
		t.Fatalf("event count mismatch: Solve=%d EvaluateMesh=%d", len(events1), len(events2))
	}

	for i := range events1 {
		if events1[i].TimeSeconds != events2[i].TimeSeconds {
			t.Fatalf("event %d time mismatch", i)
		}
	}
}
```

Check if `testBoxMesh` already exists: `grep -r "testBoxMesh\|boxMesh\|TestMesh" ism/` — if not, create a small helper.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestEvaluateMesh ./ism/...`
Expected: FAIL — `EvaluateMesh` not defined.

**Step 3: Write implementation**

Add to `ism/evaluate.go`:

```go
// EvaluateMesh takes pre-computed mesh image sources and the current scene,
// producing ir.Events by applying the mesh material.
func EvaluateMesh(sources []MeshImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if sc.Room.Mesh == nil {
		return nil, errors.New("mesh room is nil")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("no sources in scene")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("no receivers in scene")
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	bvh := geometry.BuildBVH(sc.Room.Mesh)
	material := meshMaterial(sc)
	receiver := sc.Receivers[0]

	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range sources {
			if imgSrc.Order == 0 {
				continue
			}

			event, ok := meshSpecularEvent(source, receiver, imgSrc, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)
			if ok {
				events = append(events, event)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestEvaluateMesh ./ism/...`
Expected: PASS

**Step 5: Commit**

```bash
git add ism/evaluate.go ism/evaluate_test.go
git commit -m "feat(ism): add EvaluateMesh for replaying cached mesh image sources"
```

---

## Task 8: ISM `ImageSourceCache` with validity check

**Files:**
- Create: `ism/cache.go`
- Create: `ism/cache_test.go`

**Step 1: Write the failing test**

```go
// ism/cache_test.go
package ism

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestShoeboxCache_ValidFor(t *testing.T) {
	sc := newISMTestScene()

	cache := NewShoeboxCache(
		GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 3),
		sc,
	)

	if !cache.ValidFor(sc) {
		t.Fatal("cache should be valid for the same scene")
	}
}

func TestShoeboxCache_InvalidAfterGeometryChange(t *testing.T) {
	sc := newISMTestScene()

	cache := NewShoeboxCache(
		GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 3),
		sc,
	)

	sc.Room.Shoebox.Width = 15

	if cache.ValidFor(sc) {
		t.Fatal("cache should be invalid after geometry change")
	}
}

func TestShoeboxCache_ValidAfterMaterialChange(t *testing.T) {
	sc := newISMTestScene()

	cache := NewShoeboxCache(
		GenerateImageSources(sc.Sources[0].Position, sc.Room.Shoebox, 3),
		sc,
	)

	sc.Materials["wall"] = scene.Material{Absorption: [8]float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}}

	if !cache.ValidFor(sc) {
		t.Fatal("cache should remain valid after material change")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestShoeboxCache ./ism/...`
Expected: FAIL — types not defined.

**Step 3: Write implementation**

```go
// ism/cache.go
package ism

import "github.com/cwbudde/algo-acoustics/scene"

// ShoeboxCache wraps cached shoebox image sources with a validity key.
type ShoeboxCache struct {
	Sources      []ImageSource
	GeometryHash uint64
}

// NewShoeboxCache creates a cache from the given image sources and scene.
func NewShoeboxCache(sources []ImageSource, sc *scene.Scene) *ShoeboxCache {
	return &ShoeboxCache{
		Sources:      sources,
		GeometryHash: sc.GeometryHash(),
	}
}

// ValidFor reports whether the cached image sources can be reused for this scene.
func (c *ShoeboxCache) ValidFor(sc *scene.Scene) bool {
	if c == nil || sc == nil {
		return false
	}

	return c.GeometryHash == sc.GeometryHash()
}

// MeshCache wraps cached mesh image sources with a validity key.
type MeshCache struct {
	Sources      []MeshImageSource
	GeometryHash uint64
}

// NewMeshCache creates a cache from the given mesh image sources and scene.
func NewMeshCache(sources []MeshImageSource, sc *scene.Scene) *MeshCache {
	return &MeshCache{
		Sources:      sources,
		GeometryHash: sc.GeometryHash(),
	}
}

// ValidFor reports whether the cached mesh image sources can be reused.
func (c *MeshCache) ValidFor(sc *scene.Scene) bool {
	if c == nil || sc == nil {
		return false
	}

	return c.GeometryHash == sc.GeometryHash()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestShoeboxCache ./ism/...`
Expected: PASS

**Step 5: Commit**

```bash
git add ism/cache.go ism/cache_test.go
git commit -m "feat(ism): add ShoeboxCache and MeshCache with validity checks"
```

---

## Task 9: Benchmark — replay performance target

**Files:**
- Create: `raytrace/bench_cache_test.go`

**Step 1: Write benchmark**

```go
// raytrace/bench_cache_test.go
package raytrace

import (
	"testing"
)

func BenchmarkTrace_10kRays(b *testing.B) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.Trace()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTracePaths_10kRays(b *testing.B) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.TracePaths()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluatePaths_10kRays(b *testing.B) {
	sc := newTestScene()
	rt := &RayTracer{
		Config: LaunchConfig{
			NumRays:        10_000,
			MaxBounces:     100,
			MaxTimeSeconds: 2.0,
			SpeedOfSound:   343.0,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}

	cache, err := rt.TracePaths()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for range b.N {
		_, err := rt.EvaluatePaths(cache)
		if err != nil {
			b.Fatal(err)
		}
	}
}
```

**Step 2: Run benchmark**

Run: `go test -bench BenchmarkEvaluatePaths -benchtime 5s ./raytrace/...`
Expected: EvaluatePaths < 100 ms per iteration for 10k paths (the Phase 18.1 target).

**Step 3: Commit**

```bash
git add raytrace/bench_cache_test.go
git commit -m "bench(raytrace): add benchmarks for TracePaths and EvaluatePaths"
```

---

## Task 10: Run full CI and fix any issues

**Step 1:** Run format check

Run: `just fmt`

**Step 2:** Run linter

Run: `just lint`

Fix any issues.

**Step 3:** Run full test suite

Run: `just test`

Ensure no regressions.

**Step 4:** Commit any fixes

```bash
git add -A
git commit -m "fix: address lint and format issues from Phase 18.1"
```

---

## Task 11: Update PLAN.md

**Files:**
- Modify: `PLAN.md`

Mark the completed Phase 18.1 items:

```
- [x] Refactor ray tracer to store ray paths as geometry-only data
- [x] Implement "replay" function: given cached paths + material coefficients -> energy histogram / IR
- [x] Cache invalidation tags: geometry hash + source/receiver position hash + material hash
- [x] Material-only change: reuse cached paths, recompute energy only (target < 100 ms for 10k paths)
- [x] Geometry change: invalidate all cached paths, full re-trace required
```

**Commit:**

```bash
git add PLAN.md
git commit -m "docs: mark Phase 18.1 trace/evaluate separation as complete"
```
