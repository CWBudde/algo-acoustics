package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestShoeboxTracerNextHitFindsNearestWall(t *testing.T) {
	t.Parallel()

	tracer, err := NewShoeboxTracer(&scene.Shoebox{Width: 4, Depth: 3, Height: 2})
	if err != nil {
		t.Fatalf("NewShoeboxTracer() error = %v", err)
	}

	hitPoint, normal, wallIdx, ok := tracer.NextHit(geometry.NewRay(geometry.Vec3{X: 1, Y: 1, Z: 1}, geometry.Vec3{X: 1, Y: 0, Z: 0}))
	if !ok {
		t.Fatal("NextHit() returned ok=false")
	}

	if wallIdx != 1 {
		t.Fatalf("wallIdx = %d, want 1", wallIdx)
	}

	if diff := math.Abs(hitPoint.X - 4); diff > 1e-12 {
		t.Fatalf("hitPoint.X = %g, want 4", hitPoint.X)
	}

	if hitPoint.Y != 1 || hitPoint.Z != 1 {
		t.Fatalf("hitPoint = %#v, want x wall hit at y=1 z=1", hitPoint)
	}

	if normal != (geometry.Vec3{X: 1}) {
		t.Fatalf("normal = %#v, want +X", normal)
	}
}
