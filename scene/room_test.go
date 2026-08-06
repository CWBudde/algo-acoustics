package scene_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestRoomVolumeUsesMeshEnclosureNotBounds(t *testing.T) {
	t.Parallel()

	mesh := &geometry.Mesh{Triangles: []geometry.Triangle{
		{V0: geometry.Vec3{}, V1: geometry.Vec3{Y: 1}, V2: geometry.Vec3{X: 1}},
		{V0: geometry.Vec3{}, V1: geometry.Vec3{X: 1}, V2: geometry.Vec3{Z: 1}},
		{V0: geometry.Vec3{}, V1: geometry.Vec3{Z: 1}, V2: geometry.Vec3{Y: 1}},
		{V0: geometry.Vec3{X: 1}, V1: geometry.Vec3{Y: 1}, V2: geometry.Vec3{Z: 1}},
	}}
	room := scene.Room{Kind: scene.RoomKindMesh, Mesh: mesh}

	got, ok := room.Volume()
	if !ok {
		t.Fatal("Volume() reported a watertight tetrahedron as unsupported")
	}

	if math.Abs(got-1.0/6.0) > 1e-12 {
		t.Fatalf("Volume() = %v, want %v", got, 1.0/6.0)
	}

	bounds, boundsOK := room.Bounds()
	if !boundsOK || bounds.Volume() != 1 {
		t.Fatalf("Bounds().Volume() = %v, want 1", bounds.Volume())
	}
}

func TestRoomVolumeRejectsInvalidShoeboxDimensions(t *testing.T) {
	t.Parallel()

	for _, dimension := range []float64{-1, 0, math.Inf(1), math.NaN()} {
		room := scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:  dimension,
				Depth:  2,
				Height: 3,
			},
		}

		if volume, ok := room.Volume(); ok {
			t.Fatalf("Volume() = (%v, true) for invalid width %v", volume, dimension)
		}
	}

	overflowing := scene.Room{
		Kind: scene.RoomKindShoebox,
		Shoebox: &scene.Shoebox{
			Width:  math.MaxFloat64,
			Depth:  math.MaxFloat64,
			Height: math.MaxFloat64,
		},
	}
	if volume, ok := overflowing.Volume(); ok {
		t.Fatalf("Volume() = (%v, true) for overflowing finite dimensions", volume)
	}
}
