package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

func TestNewIBMSource_InsideRoom(t *testing.T) {
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	src, err := NewIBMSource(room, g, geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, SoftSource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src.NodeIdx < 0 || src.NodeIdx >= len(g.Class) {
		t.Fatalf("node index %d out of range [0, %d)", src.NodeIdx, len(g.Class))
	}

	if g.Class[src.NodeIdx] == Exterior {
		t.Error("source placed on exterior node")
	}
}

func TestNewIBMSource_OutsideRoom(t *testing.T) {
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	_, err := NewIBMSource(room, g, geometry.Vec3{X: 5, Y: 5, Z: 5}, SoftSource)
	if err == nil {
		t.Fatal("expected error for source outside room")
	}
}

func TestNewIBMSource_OnWall(t *testing.T) {
	// Position exactly on wall is outside (strict PointInside).
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	_, err := NewIBMSource(room, g, geometry.Vec3{X: 0, Y: 1.5, Z: 1.5}, SoftSource)
	if err == nil {
		t.Fatal("expected error for source on wall (strict PointInside)")
	}
}

func TestIBMSource_SoftMode(t *testing.T) {
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	src, err := NewIBMSource(room, g, geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, SoftSource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := make([]float64, g.Nx*g.Ny*g.Nz)
	field[src.NodeIdx] = 3.0

	src.Inject(field, 2.0)

	if got := field[src.NodeIdx]; got != 5.0 {
		t.Errorf("soft source: expected 5.0, got %v", got)
	}
}

func TestIBMSource_HardMode(t *testing.T) {
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	src, err := NewIBMSource(room, g, geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, HardSource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := make([]float64, g.Nx*g.Ny*g.Nz)
	field[src.NodeIdx] = 3.0

	src.Inject(field, 2.0)

	if got := field[src.NodeIdx]; got != 2.0 {
		t.Errorf("hard source: expected 2.0, got %v", got)
	}
}

func TestGaussianPulse_Shape(t *testing.T) {
	t0 := 0.01
	sigma := 0.002

	// Peak at t = t0.
	peak := GaussianPulse(t0, t0, sigma)
	if math.Abs(peak-1.0) > 1e-12 {
		t.Errorf("peak = %v, want 1.0", peak)
	}

	// Symmetric.
	left := GaussianPulse(t0-sigma, t0, sigma)
	right := GaussianPulse(t0+sigma, t0, sigma)

	if math.Abs(left-right) > 1e-12 {
		t.Errorf("asymmetric: left=%v, right=%v", left, right)
	}

	// At ±σ the value is exp(-1) ≈ 0.3679.
	expected := math.Exp(-1)
	if math.Abs(left-expected) > 1e-12 {
		t.Errorf("at ±σ: got %v, want %v", left, expected)
	}

	// Far from center: nearly zero.
	far := GaussianPulse(t0+10*sigma, t0, sigma)
	if far > 1e-40 {
		t.Errorf("far tail = %v, expected ~0", far)
	}
}

func TestSineBurst_Shape(t *testing.T) {
	freq := 1000.0
	nCycles := 4
	duration := float64(nCycles) / freq

	// Zero outside the window.
	if v := SineBurst(-0.001, freq, nCycles); v != 0 {
		t.Errorf("before window: %v, want 0", v)
	}

	if v := SineBurst(duration+0.001, freq, nCycles); v != 0 {
		t.Errorf("after window: %v, want 0", v)
	}

	// At endpoints: Hann window is zero.
	if v := SineBurst(0, freq, nCycles); math.Abs(v) > 1e-12 {
		t.Errorf("at t=0: %v, want ~0", v)
	}

	// Peak somewhere in the middle — find max by sampling.
	maxVal := 0.0
	nSamples := 10000
	dt := duration / float64(nSamples)

	for i := range nSamples {
		v := math.Abs(SineBurst(float64(i)*dt, freq, nCycles))
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal < 0.5 {
		t.Errorf("max amplitude %v too small", maxVal)
	}

	if maxVal > 1.0+1e-12 {
		t.Errorf("max amplitude %v exceeds 1.0", maxVal)
	}
}

func TestIBMSource_FDTDWithGaussianPulse(t *testing.T) {
	// Run a short FDTD simulation with a Gaussian pulse source and verify
	// that energy propagates outward from the source.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.25)
	stencil := NewIBMStencil(g, RigidWallBC())

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	src, err := NewIBMSource(room, g, geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, SoftSource)
	if err != nil {
		t.Fatalf("source creation: %v", err)
	}

	n := g.Nx * g.Ny * g.Nz
	pCur := make([]float64, n)
	pPrev := make([]float64, n)
	pNext := make([]float64, n)

	// Gaussian pulse parameters.
	t0 := 5.0 * dt // center pulse a few steps in
	sigma := 2.0 * dt

	nSteps := 100
	maxEnergy := 0.0

	for step := range nSteps {
		tNow := float64(step) * dt

		// Inject source signal.
		src.Inject(pCur, GaussianPulse(tNow, t0, sigma))

		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev

		e := totalEnergy(g, pCur)
		if e > maxEnergy {
			maxEnergy = e
		}
	}

	// Energy should have been injected.
	finalEnergy := totalEnergy(g, pCur)
	if maxEnergy == 0 {
		t.Error("no energy was injected")
	}

	// With rigid walls, energy is conserved after the pulse ends.
	// The final energy should be a significant fraction of the max.
	ratio := finalEnergy / maxEnergy
	t.Logf("max energy: %.6g, final energy: %.6g, ratio: %.4f", maxEnergy, finalEnergy, ratio)

	if ratio < 0.5 {
		t.Errorf("energy ratio %.4f too low — possible instability or decay", ratio)
	}
}

func TestIBMSource_HardSourceReflects(t *testing.T) {
	// Hard source should maintain its value regardless of incoming waves.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.25)
	stencil := NewIBMStencil(g, RigidWallBC())

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	src, err := NewIBMSource(room, g, geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, HardSource)
	if err != nil {
		t.Fatalf("source creation: %v", err)
	}

	n := g.Nx * g.Ny * g.Nz
	pCur := make([]float64, n)
	pPrev := make([]float64, n)
	pNext := make([]float64, n)

	// Drive a constant value for several steps then check the source node.
	driveValue := 1.0
	nSteps := 50

	for range nSteps {
		src.Inject(pCur, driveValue)
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}

	// After stepping, inject once more and check.
	src.Inject(pCur, driveValue)

	if got := pCur[src.NodeIdx]; got != driveValue {
		t.Errorf("hard source node = %v, want %v", got, driveValue)
	}

	// Verify some energy propagated to other nodes.
	otherEnergy := 0.0
	for i, v := range pCur {
		if i != src.NodeIdx && g.Class[i] != Exterior {
			otherEnergy += v * v
		}
	}

	if otherEnergy == 0 {
		t.Error("no energy propagated from hard source to other nodes")
	}
}

func TestIBMSource_RotatedRoom(t *testing.T) {
	// Source inside a non-rectangular (diamond) room.
	centre := geometry.Vec3{X: 2, Y: 2, Z: 2}
	diamondVerts2D := [4]geometry.Vec3{
		{X: 2, Y: 0}, {X: 4, Y: 2}, {X: 2, Y: 4}, {X: 0, Y: 2},
	}

	diamondWalls := make([]geometry.Plane, 0, 4)

	for i := range 4 {
		a := diamondVerts2D[i]
		b := diamondVerts2D[(i+1)%4]
		edge := b.Sub(a)
		perp := geometry.Vec3{X: edge.Y, Y: -edge.X}
		mid := a.Add(b).Scale(0.5)

		if perp.Dot(centre.Sub(mid)) < 0 {
			perp = perp.Neg()
		}

		diamondWalls = append(diamondWalls, geometry.NewPlaneFromPointNormal(a, perp))
	}

	allWalls := make([]geometry.Plane, 0, 6)
	allWalls = append(allWalls,
		geometry.Plane{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		geometry.Plane{Normal: geometry.Vec3{Z: -1}, Distance: -4},
	)
	allWalls = append(allWalls, diamondWalls...)

	allVerts := []geometry.Vec3{
		{X: 2, Y: 0, Z: 0},
		{X: 4, Y: 2, Z: 0},
		{X: 2, Y: 4, Z: 0},
		{X: 0, Y: 2, Z: 0},
		{X: 2, Y: 0, Z: 4},
		{X: 4, Y: 2, Z: 4},
		{X: 2, Y: 4, Z: 4},
		{X: 0, Y: 2, Z: 4},
	}

	room, err := NewConvexRoom(allWalls, allVerts)
	if err != nil {
		t.Fatalf("room construction: %v", err)
	}

	g := ClassifyGrid(room, 0.3)

	// Place source at room center.
	src, err := NewIBMSource(room, g, geometry.Vec3{X: 2, Y: 2, Z: 2}, SoftSource)
	if err != nil {
		t.Fatalf("source creation: %v", err)
	}

	if g.Class[src.NodeIdx] == Exterior {
		t.Error("source placed on exterior node in rotated room")
	}

	t.Logf("source at node %d (class=%d)", src.NodeIdx, g.Class[src.NodeIdx])
}
