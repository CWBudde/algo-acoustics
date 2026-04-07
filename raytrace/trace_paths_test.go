package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newTracePathsRayTracer() *RayTracer {
	sc := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  6,
				Depth:  4.5,
				Height: 2.8,
				WallMaterials: [6]string{
					"reflective", "reflective", "reflective",
					"reflective", "reflective", "reflective",
				},
			},
		},
		Materials: map[string]scene.Material{
			"reflective": scene.MaterialFullyReflective(),
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1.2, Y: 1.0, Z: 1.2}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3.5, Y: 2.2, Z: 1.2}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	return &RayTracer{
		Config: LaunchConfig{
			NumRays:        100,
			MaxBounces:     8,
			MaxTimeSeconds: 1,
			SpeedOfSound:   acoustics.SpeedOfSound,
		},
		Scene:          sc,
		ReceiverRadius: 0.25,
	}
}

func TestTracePaths_ReturnsNonEmptyCache(t *testing.T) {
	t.Parallel()

	rt := newTracePathsRayTracer()

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	if len(cache.Paths) != 100 {
		t.Fatalf("TracePaths() returned %d paths, want 100", len(cache.Paths))
	}
}

func TestTracePaths_EachPathHasSteps(t *testing.T) {
	t.Parallel()

	rt := newTracePathsRayTracer()

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	for i, path := range cache.Paths {
		if len(path.Steps) == 0 {
			t.Fatalf("path %d has 0 steps, want >0", i)
		}

		for j, step := range path.Steps {
			if step.SegmentLength <= 0 {
				t.Fatalf("path %d step %d has segment length %v, want >0", i, j, step.SegmentLength)
			}
		}
	}
}

func TestTracePaths_CacheHashMatchesScene(t *testing.T) {
	t.Parallel()

	rt := newTracePathsRayTracer()

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	if !cache.ValidFor(rt.Scene, rt.ReceiverRadius) {
		t.Fatal("cache.ValidFor(scene, radius) = false, want true")
	}
}

func TestTracePaths_PathLengthBounded(t *testing.T) {
	t.Parallel()

	rt := newTracePathsRayTracer()
	maxPathLength := rt.Config.MaxTimeSeconds * rt.Config.SpeedOfSound

	cache, err := rt.TracePaths()
	if err != nil {
		t.Fatalf("TracePaths() error = %v", err)
	}

	for i, path := range cache.Paths {
		var total float64
		for _, step := range path.Steps {
			total += step.SegmentLength
		}

		if total > maxPathLength+1e-6 {
			t.Fatalf("path %d total length %v exceeds max %v", i, total, maxPathLength)
		}
	}
}
