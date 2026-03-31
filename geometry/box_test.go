package geometry_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestBoxContains(t *testing.T) {
	b := geometry.NewBox(geometry.Vec3{0, 0, 0}, geometry.Vec3{2, 2, 2})

	if !b.Contains(geometry.Vec3{1, 1, 1}) {
		t.Error("centre point should be inside")
	}

	if !b.Contains(geometry.Vec3{0, 0, 0}) {
		t.Error("corner point should be inside (on boundary)")
	}

	if b.Contains(geometry.Vec3{3, 1, 1}) {
		t.Error("point outside box should not be contained")
	}
}

func TestBoxCenter(t *testing.T) {
	b := geometry.NewBox(geometry.Vec3{0, 0, 0}, geometry.Vec3{4, 6, 8})
	got := b.Center()
	want := geometry.Vec3{2, 3, 4}

	if got != want {
		t.Errorf("Center = %v, want %v", got, want)
	}
}

func TestBoxDimensions(t *testing.T) {
	b := geometry.NewBox(geometry.Vec3{1, 2, 3}, geometry.Vec3{4, 6, 9})
	got := b.Dimensions()
	want := geometry.Vec3{3, 4, 6}

	if got != want {
		t.Errorf("Dimensions = %v, want %v", got, want)
	}
}

func TestBoxVolume(t *testing.T) {
	b := geometry.NewBox(geometry.Vec3{0, 0, 0}, geometry.Vec3{3, 4, 5})
	got := b.Volume()

	if math.Abs(got-60) > 1e-12 {
		t.Errorf("Volume = %v, want 60", got)
	}
}

func TestNewBoxNormalisesOrder(t *testing.T) {
	// Construct with reversed corners — should still produce a valid box.
	b := geometry.NewBox(geometry.Vec3{4, 6, 8}, geometry.Vec3{1, 2, 3})

	if b.Min.X >= b.Max.X || b.Min.Y >= b.Max.Y || b.Min.Z >= b.Max.Z {
		t.Errorf("NewBox did not normalise corners: Min=%v Max=%v", b.Min, b.Max)
	}
}
