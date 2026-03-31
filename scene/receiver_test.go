package scene

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestReceiverWorldToHeadDirIdentityOrientation(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatIdentity()}
	dir := receiver.WorldToHeadDir(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if dir != (geometry.Vec3{X: 1, Y: 0, Z: 0}) {
		t.Fatalf("WorldToHeadDir() = %#v, want +X", dir)
	}
}

func TestReceiverWorldToHeadDirRotatedOrientation(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2)}
	dir := receiver.WorldToHeadDir(geometry.Vec3{X: 1, Y: 0, Z: 0})

	if math.Abs(dir.X) > 1e-9 || math.Abs(dir.Z) > 1e-9 {
		t.Fatalf("WorldToHeadDir() = %#v, want direction in XY plane", dir)
	}
	if math.Abs(dir.Y+1) > 1e-9 {
		t.Fatalf("WorldToHeadDir() = %#v, want -Y", dir)
	}
}

func TestReceiverWorldToHeadDirQuarterTurnSwapsLaterals(t *testing.T) {
	t.Parallel()

	receiver := Receiver{Orientation: geometry.QuatFromAxisAngle(geometry.Vec3{X: 0, Y: 0, Z: 1}, math.Pi/2)}
	left := receiver.WorldToHeadDir(geometry.Vec3{X: 0, Y: 1, Z: 0})
	right := receiver.WorldToHeadDir(geometry.Vec3{X: 0, Y: -1, Z: 0})

	if math.Abs(left.X-1) > 1e-9 || math.Abs(left.Y) > 1e-9 || math.Abs(left.Z) > 1e-9 {
		t.Fatalf("left direction = %#v, want +X", left)
	}
	if math.Abs(right.X+1) > 1e-9 || math.Abs(right.Y) > 1e-9 || math.Abs(right.Z) > 1e-9 {
		t.Fatalf("right direction = %#v, want -X", right)
	}
}
