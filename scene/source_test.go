package scene

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestSourceDirectionToIdentityOrientation(t *testing.T) {
	t.Parallel()

	source := Source{Position: geometry.Vec3Zero, Orientation: geometry.QuatIdentity()}
	dir := source.DirectionTo(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if dir != (geometry.Vec3{X: 1, Y: 0, Z: 0}) {
		t.Fatalf("DirectionTo() = %#v, want +X", dir)
	}
}

func TestSourceDirectionToRotatedOrientation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		angle   float64
		wantDir geometry.Vec3
	}{
		{name: "90deg", angle: math.Pi / 2, wantDir: geometry.Vec3{X: 0, Y: -1, Z: 0}},
		{name: "180deg", angle: math.Pi, wantDir: geometry.Vec3{X: -1, Y: 0, Z: 0}},
		{name: "270deg", angle: 3 * math.Pi / 2, wantDir: geometry.Vec3{X: 0, Y: 1, Z: 0}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := Source{
				Position:    geometry.Vec3Zero,
				Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, tc.angle),
			}
			dir := source.DirectionTo(geometry.Vec3{X: 1, Y: 0, Z: 0})

			if math.Abs(dir.X-tc.wantDir.X) > 1e-9 || math.Abs(dir.Y-tc.wantDir.Y) > 1e-9 || math.Abs(dir.Z-tc.wantDir.Z) > 1e-9 {
				t.Fatalf("DirectionTo() = %#v, want %#v", dir, tc.wantDir)
			}
		})
	}
}
