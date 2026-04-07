package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func newTestShoeboxScene(width, depth, height float64) *scene.Scene {
	return &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  width,
				Depth:  depth,
				Height: height,
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
}

func TestPathCache_ValidFor_SameScene(t *testing.T) {
	t.Parallel()

	sc := newTestShoeboxScene(6, 4.5, 2.8)
	radius := 0.25

	cache := &PathCache{
		GeometryHash:   sc.GeometryHash(),
		ReceiverRadius: radius,
	}

	if !cache.ValidFor(sc, radius) {
		t.Fatal("ValidFor() = false for same scene and radius, want true")
	}
}

func TestPathCache_ValidFor_DifferentGeometry(t *testing.T) {
	t.Parallel()

	sc1 := newTestShoeboxScene(6, 4.5, 2.8)
	sc2 := newTestShoeboxScene(8, 5.0, 3.0) // different dimensions

	radius := 0.25

	cache := &PathCache{
		GeometryHash:   sc1.GeometryHash(),
		ReceiverRadius: radius,
	}

	if cache.ValidFor(sc2, radius) {
		t.Fatal("ValidFor() = true for different geometry, want false")
	}
}

func TestPathCache_ValidFor_DifferentReceiverRadius(t *testing.T) {
	t.Parallel()

	sc := newTestShoeboxScene(6, 4.5, 2.8)

	cache := &PathCache{
		GeometryHash:   sc.GeometryHash(),
		ReceiverRadius: 0.25,
	}

	if cache.ValidFor(sc, 0.5) {
		t.Fatal("ValidFor() = true for different receiver radius, want false")
	}
}
