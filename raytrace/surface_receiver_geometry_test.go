package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestNewSurfaceReceiverGeometryAndIntersection(t *testing.T) {
	t.Parallel()

	detector, err := NewSurfaceReceiver([]geometry.Vec3{
		{X: 2, Y: 0, Z: 0},
		{X: 2, Y: 2, Z: 0},
		{X: 2, Y: 2, Z: 2},
		{X: 2, Y: 0, Z: 2},
	})
	if err != nil {
		t.Fatalf("NewSurfaceReceiver() error = %v", err)
	}

	if math.Abs(detector.Area-4) > 1e-12 || detector.Center != (geometry.Vec3{X: 2, Y: 1, Z: 1}) {
		t.Fatalf("detector geometry = center %v area %v", detector.Center, detector.Area)
	}

	distance, hit := detector.Intersects(
		geometry.NewRay(geometry.Vec3{Y: 1, Z: 1}, geometry.Vec3{X: 1}),
		0,
		3,
	)
	if !hit || math.Abs(distance-2) > 1e-12 {
		t.Fatalf("Intersects() = (%v, %v), want (2, true)", distance, hit)
	}

	_, hit = detector.Intersects(
		geometry.NewRay(geometry.Vec3{Y: 3, Z: 1}, geometry.Vec3{X: 1}),
		0,
		3,
	)
	if hit {
		t.Fatal("Intersects() hit outside polygon")
	}
}

func TestNewSurfaceReceiverRejectsInvalidPolygons(t *testing.T) {
	t.Parallel()

	tests := [][]geometry.Vec3{
		{{}, {X: 1}},
		{{}, {X: 1}, {X: 2}},
		{{}, {X: 1}, {Y: 1}, {Z: 0.1}},
	}

	for _, polygon := range tests {
		_, err := NewSurfaceReceiver(polygon)
		if err == nil {
			t.Fatalf("NewSurfaceReceiver(%v) error = nil, want error", polygon)
		}
	}
}
