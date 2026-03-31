package directivity

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestOmniModelGainLinear(t *testing.T) {
	t.Parallel()

	model := OmniModel{}
	if got := model.GainLinear(125, geometry.Vec3{X: 1, Y: 2, Z: 3}); got != 1 {
		t.Fatalf("GainLinear() = %v, want 1", got)
	}
}

func TestCardioidModelGainLinear(t *testing.T) {
	t.Parallel()

	model := CardioidModel{Axis: geometry.Vec3{X: 1, Y: 0, Z: 0}, OrderN: 1}

	if got := model.GainLinear(125, geometry.Vec3{X: 1, Y: 0, Z: 0}); got != 1 {
		t.Fatalf("on-axis GainLinear() = %v, want 1", got)
	}

	if got := model.GainLinear(125, geometry.Vec3{X: -1, Y: 0, Z: 0}); got != 0 {
		t.Fatalf("rear GainLinear() = %v, want 0", got)
	}
}

func TestCardioidModelZeroAxisOrDirection(t *testing.T) {
	t.Parallel()

	model := CardioidModel{Axis: geometry.Vec3Zero, OrderN: 1}
	if got := model.GainLinear(125, geometry.Vec3{X: 1, Y: 0, Z: 0}); got != 0 {
		t.Fatalf("zero-axis GainLinear() = %v, want 0", got)
	}

	model.Axis = geometry.Vec3{X: 1, Y: 0, Z: 0}
	if got := model.GainLinear(125, geometry.Vec3Zero); got != 0 {
		t.Fatalf("zero-direction GainLinear() = %v, want 0", got)
	}
}

func TestCardioidModelPowerResponse(t *testing.T) {
	t.Parallel()

	model := CardioidModel{Axis: geometry.Vec3{X: 1, Y: 0, Z: 0}, OrderN: 2}
	got := model.GainLinear(125, geometry.Vec3{X: 0, Y: 1, Z: 0})

	want := 0.5 * 0.5
	if got != want {
		t.Fatalf("side GainLinear() = %v, want %v", got, want)
	}
}
