package geometry

import (
	"math"
	"testing"
)

func TestRayOpenFiniteCylinder(t *testing.T) {
	edgeStart := Vec3{Z: -1}
	edgeEnd := Vec3{Z: 1}

	tests := []struct {
		name   string
		ray    Ray
		radius float64
		want   bool
	}{
		{name: "perpendicular", ray: NewRay(Vec3{X: -2, Y: 0.5}, Vec3{X: 1}), radius: 0.5, want: true},
		{name: "tangent", ray: NewRay(Vec3{X: -2, Y: 1}, Vec3{X: 1}), radius: 1, want: true},
		{name: "miss", ray: NewRay(Vec3{X: -2, Y: 1.01}, Vec3{X: 1}), radius: 1, want: false},
		{name: "parallel", ray: NewRay(Vec3{X: 0.1, Z: -2}, Vec3{Z: 1}), radius: 1, want: false},
		{name: "open endpoint", ray: NewRay(Vec3{X: -2, Z: 1}, Vec3{X: 1}), radius: 1, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hit, got := RayOpenFiniteCylinder(test.ray, edgeStart, edgeEnd, test.radius, 0, 10)
			if got != test.want {
				t.Fatalf("RayOpenFiniteCylinder() hit = %v, want %v (%#v)", got, test.want, hit)
			}

			if got && (math.Abs(hit.EdgeFraction-0.5) > 1e-12 || math.Abs(hit.RayDistance-2) > 1e-12) {
				t.Fatalf("RayOpenFiniteCylinder() = %#v, want centered hit at ray distance 2", hit)
			}
		})
	}
}
