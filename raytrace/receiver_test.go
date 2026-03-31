package raytrace

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestSphereReceiverIntersects(t *testing.T) {
	t.Parallel()

	receiver := SphereReceiver{Center: geometry.Vec3{X: 5}, Radius: 1}

	tHit, ok := receiver.Intersects(geometry.NewRay(geometry.Vec3Zero, geometry.Vec3{X: 1}), 0, 10)
	if !ok {
		t.Fatal("Intersects() returned ok=false")
	}

	if tHit != 4 {
		t.Fatalf("tHit = %g, want 4", tHit)
	}

	if got := receiver.AngularWeight(geometry.Vec3{X: 1}); got != 1 {
		t.Fatalf("AngularWeight() = %g, want 1", got)
	}
}
