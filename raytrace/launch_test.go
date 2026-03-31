package raytrace

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestFibonacciSphereProducesUnitVectors(t *testing.T) {
	t.Parallel()

	directions := FibonacciSphere(64)
	if got, want := len(directions), 64; got != want {
		t.Fatalf("len(FibonacciSphere) = %d, want %d", got, want)
	}

	for i, direction := range directions {
		if diff := math.Abs(direction.Norm() - 1); diff > 1e-12 {
			t.Fatalf("direction %d norm = %g, want 1", i, direction.Norm())
		}
	}
}

func TestLaunchRaysUsesRequestedCount(t *testing.T) {
	t.Parallel()

	origin := geometry.Vec3{X: 1, Y: 2, Z: 3}
	rays := LaunchRays(origin, LaunchConfig{NumRays: 11})
	if got, want := len(rays), 11; got != want {
		t.Fatalf("len(LaunchRays) = %d, want %d", got, want)
	}

	for i, ray := range rays {
		if ray.Origin != origin {
			t.Fatalf("ray %d origin = %#v, want %#v", i, ray.Origin, origin)
		}
		if diff := math.Abs(ray.Direction.Norm() - 1); diff > 1e-12 {
			t.Fatalf("ray %d direction norm = %g, want 1", i, ray.Direction.Norm())
		}
	}
}
