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

	source := Source{
		Position:    geometry.Vec3Zero,
		Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2),
	}
	dir := source.DirectionTo(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if math.Abs(dir.X) > 1e-9 || math.Abs(dir.Z) > 1e-9 {
		t.Fatalf("DirectionTo() = %#v, want perpendicular direction in XY plane", dir)
	}
	if math.Abs(math.Abs(dir.Y)-1) > 1e-9 {
		t.Fatalf("DirectionTo() = %#v, want unit-length perpendicular direction", dir)
	}
}
