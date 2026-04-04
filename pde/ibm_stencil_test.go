package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// --- helpers ---

// makeField allocates a zero-initialized pressure field for the grid.
func makeField(g *IBMGrid) []float64 {
	return make([]float64, g.Nx*g.Ny*g.Nz)
}

// setInterior sets all interior+boundary nodes to a constant value.
func setInterior(g *IBMGrid, field []float64, val float64) {
	for i, c := range g.Class {
		if c != Exterior {
			field[i] = val
		}
	}
}

// totalEnergy returns the sum of p² over active nodes.
func totalEnergy(g *IBMGrid, field []float64) float64 {
	e := 0.0

	for i, c := range g.Class {
		if c != Exterior {
			e += field[i] * field[i]
		}
	}

	return e
}

// --- Tests ---

func TestIBMStencil_ExteriorNodesStayZero(t *testing.T) {
	// Verify that exterior nodes are always zero after applying the stencil.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)
	stencil := NewIBMStencil(g, RigidWallBC())

	src := makeField(g)
	setInterior(g, src, 1.0)

	dst := makeField(g)
	stencil.ApplyLaplacian(dst, src)

	for i, c := range g.Class {
		if c == Exterior && dst[i] != 0 {
			t.Errorf("exterior node %d has nonzero Laplacian output: %v", i, dst[i])
		}
	}
}

func TestIBMStencil_NoReadsFromExterior(t *testing.T) {
	// Place sentinel NaN values in all exterior nodes.  If the stencil
	// reads from any exterior node, the result will contain NaN.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)
	stencil := NewIBMStencil(g, RigidWallBC())

	src := makeField(g)
	setInterior(g, src, 1.0)

	for i, c := range g.Class {
		if c == Exterior {
			src[i] = math.NaN()
		}
	}

	dst := makeField(g)
	stencil.ApplyLaplacian(dst, src)

	for i, c := range g.Class {
		if c != Exterior && math.IsNaN(dst[i]) {
			t.Errorf("active node %d has NaN output — stencil reads from exterior data", i)
		}
	}
}

func TestIBMStencil_UniformFieldLaplacianZero(t *testing.T) {
	// The Laplacian of a uniform field should be ~0 for interior nodes
	// with rigid walls (Neumann ghost = node value).
	room := rectRoom(4, 4, 4)
	g := ClassifyGrid(room, 0.5)
	stencil := NewIBMStencil(g, RigidWallBC())

	src := makeField(g)
	setInterior(g, src, 5.0)

	dst := makeField(g)
	stencil.ApplyLaplacian(dst, src)

	for i, c := range g.Class {
		if c == Interior && math.Abs(dst[i]) > 1e-10 {
			t.Errorf("interior node %d: Laplacian of constant = %v, want ~0", i, dst[i])
		}
	}

	// Boundary nodes with rigid walls: ghost = node value, so Laplacian
	// should also be ~0 for a uniform field.
	for i, c := range g.Class {
		if c == Boundary && math.Abs(dst[i]) > 1e-10 {
			t.Errorf("boundary node %d: Laplacian of constant = %v, want ~0 (rigid BC)", i, dst[i])
		}
	}
}

func TestIBMStencil_ImpedanceAbsorption(t *testing.T) {
	// With R=0 (perfectly absorbing), boundary ghost values are zero.
	// The Laplacian of a uniform field should be nonzero at boundary nodes
	// since the ghost pressure is 0 instead of equal to the node.
	room := rectRoom(4, 4, 4)
	g := ClassifyGrid(room, 0.5)
	stencil := NewIBMStencil(g, ImpedanceWallBC(0))

	src := makeField(g)
	setInterior(g, src, 5.0)

	dst := makeField(g)
	stencil.ApplyLaplacian(dst, src)

	// Interior nodes far from boundary should still have ~0 Laplacian.
	for i, c := range g.Class {
		if c == Interior && math.Abs(dst[i]) > 1e-10 {
			t.Errorf("interior node %d: Laplacian = %v, want ~0", i, dst[i])
		}
	}

	// At least some boundary nodes should have nonzero Laplacian (absorption).
	nonzero := 0

	for i, c := range g.Class {
		if c == Boundary && math.Abs(dst[i]) > 1e-10 {
			nonzero++
		}
	}

	if nonzero == 0 {
		t.Error("expected nonzero Laplacian at boundary nodes with R=0")
	}
}

func TestIBMStencil_RigidVsImpedanceR1(t *testing.T) {
	// WallRigid and WallImpedance with R=1 should produce identical results.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.5)

	rigid := NewIBMStencil(g, RigidWallBC())
	imp := NewIBMStencil(g, ImpedanceWallBC(1.0))

	src := makeField(g)
	// Non-uniform field so the Laplacian is non-trivial.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] != Exterior {
					p := g.nodePos(ix, iy, iz)
					src[idx] = p.X*p.X + p.Y*0.5 + p.Z
				}
			}
		}
	}

	dstRigid := makeField(g)
	dstImp := makeField(g)

	rigid.ApplyLaplacian(dstRigid, src)
	imp.ApplyLaplacian(dstImp, src)

	for i := range dstRigid {
		if math.Abs(dstRigid[i]-dstImp[i]) > 1e-12 {
			t.Errorf("node %d: rigid=%v vs impedance(R=1)=%v", i, dstRigid[i], dstImp[i])
		}
	}
}

func TestIBMStencil_CornerNode(t *testing.T) {
	// A small room where boundary nodes near corners have walls on
	// multiple axes.  Verify no panics and correct zero-Laplacian for
	// a constant field.
	room := rectRoom(1.5, 1.5, 1.5)
	g := ClassifyGrid(room, 0.5)
	stencil := NewIBMStencil(g, RigidWallBC())

	// Count boundary nodes with walls on ≥ 2 axes (corners).
	corners := 0

	for _, bi := range g.Boundary {
		axesWithWall := 0

		for a := range 3 {
			for d := range 2 {
				if bi.Frac[a][d] > 0 {
					axesWithWall++

					break
				}
			}
		}

		if axesWithWall >= 2 {
			corners++
		}
	}

	t.Logf("boundary=%d, corners(≥2 axes)=%d", g.NumBoundary(), corners)

	if corners == 0 {
		t.Error("expected some corner boundary nodes in a 1.5m room with h=0.5")
	}

	src := makeField(g)
	setInterior(g, src, 3.0)

	dst := makeField(g)
	stencil.ApplyLaplacian(dst, src)

	for i, c := range g.Class {
		if c != Exterior && math.Abs(dst[i]) > 1e-10 {
			t.Errorf("node %d (class %d): constant-field Laplacian = %v, want ~0", i, c, dst[i])
		}
	}
}

func TestIBMStencil_CFLLimit(t *testing.T) {
	room := rectRoom(4, 4, 4)
	g := ClassifyGrid(room, 0.1)
	stencil := NewIBMStencil(g, RigidWallBC())

	c := 343.0
	dt := stencil.CFLLimit(c)

	// Standard CFL limit: h / (c·√3).
	dtStandard := g.H / (c * math.Sqrt(3))

	if dt <= 0 {
		t.Fatalf("CFL limit = %v, must be positive", dt)
	}

	if dt > dtStandard {
		t.Errorf("CFL limit %v exceeds standard limit %v", dt, dtStandard)
	}

	t.Logf("CFL limit: %.6g s  (standard: %.6g s, ratio: %.3f)",
		dt, dtStandard, dt/dtStandard)
}

func TestIBMStencil_FDTDStability(t *testing.T) {
	// Run many FDTD steps with a Gaussian initial condition and verify
	// that energy does not blow up (CFL stability).
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.25)
	stencil := NewIBMStencil(g, RigidWallBC())

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c) // 95% of CFL for safety margin

	pCur := makeField(g)
	pPrev := makeField(g)
	pNext := makeField(g)

	// Gaussian initial condition centred in the room.
	centre := geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}
	sigma := 0.3

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] == Exterior {
					continue
				}

				p := g.nodePos(ix, iy, iz)
				r2 := (p.X-centre.X)*(p.X-centre.X) +
					(p.Y-centre.Y)*(p.Y-centre.Y) +
					(p.Z-centre.Z)*(p.Z-centre.Z)
				pCur[idx] = math.Exp(-r2 / (2 * sigma * sigma))
			}
		}
	}

	copy(pPrev, pCur)

	e0 := totalEnergy(g, pCur)
	nSteps := 500
	maxEnergy := e0

	for step := range nSteps {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev

		if step%100 == 0 {
			e := totalEnergy(g, pCur)
			if e > maxEnergy {
				maxEnergy = e
			}
		}
	}

	eFinal := totalEnergy(g, pCur)
	t.Logf("energy: initial=%.6g, final=%.6g, max=%.6g, ratio=%.3f",
		e0, eFinal, maxEnergy, maxEnergy/e0)

	// With rigid walls (no absorption), energy should oscillate but not
	// grow unbounded.  Allow 50% growth due to numerical dispersion effects
	// in the discrete system.
	if maxEnergy > 2.0*e0 {
		t.Errorf("energy grew by factor %.1f — CFL instability suspected", maxEnergy/e0)
	}
}

func TestIBMStencil_FDTDImpedanceDamping(t *testing.T) {
	// With absorbing walls (R=0.5), energy should decrease over time.
	room := rectRoom(3, 3, 3)
	g := ClassifyGrid(room, 0.25)
	stencil := NewIBMStencil(g, ImpedanceWallBC(0.5))

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	pCur := makeField(g)
	pPrev := makeField(g)
	pNext := makeField(g)

	centre := geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}
	sigma := 0.3

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] == Exterior {
					continue
				}

				p := g.nodePos(ix, iy, iz)
				r2 := (p.X-centre.X)*(p.X-centre.X) +
					(p.Y-centre.Y)*(p.Y-centre.Y) +
					(p.Z-centre.Z)*(p.Z-centre.Z)
				pCur[idx] = math.Exp(-r2 / (2 * sigma * sigma))
			}
		}
	}

	copy(pPrev, pCur)

	e0 := totalEnergy(g, pCur)

	for range 500 {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}

	eFinal := totalEnergy(g, pCur)
	t.Logf("impedance damping: initial=%.6g, final=%.6g, ratio=%.4f", e0, eFinal, eFinal/e0)

	if eFinal >= e0 {
		t.Errorf("energy did not decrease with absorbing walls: initial=%.6g, final=%.6g", e0, eFinal)
	}
}

func TestIBMStencil_RotatedRoom_CFL(t *testing.T) {
	// Verify CFL stability for a non-axis-aligned room (45° diamond)
	// where boundary fractions are non-trivial.
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
		t.Fatal(err)
	}

	g := ClassifyGrid(room, 0.3)
	stencil := NewIBMStencil(g, RigidWallBC())

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	pCur := makeField(g)
	pPrev := makeField(g)
	pNext := makeField(g)

	// Gaussian at centre.
	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				idx := g.nodeIndex(ix, iy, iz)
				if g.Class[idx] == Exterior {
					continue
				}

				p := g.nodePos(ix, iy, iz)
				r2 := (p.X-2)*(p.X-2) + (p.Y-2)*(p.Y-2) + (p.Z-2)*(p.Z-2)
				pCur[idx] = math.Exp(-r2 / 0.18)
			}
		}
	}

	copy(pPrev, pCur)

	e0 := totalEnergy(g, pCur)

	for range 300 {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}

	eFinal := totalEnergy(g, pCur)
	t.Logf("rotated room: initial=%.6g, final=%.6g, ratio=%.3f", e0, eFinal, eFinal/e0)

	if eFinal > 2.0*e0 {
		t.Errorf("energy blew up in rotated room: ratio=%.1f", eFinal/e0)
	}
}

func TestIBMStencil_CompressedMatchesFull(t *testing.T) {
	// Verify that compressed and full modes produce identical Laplacian results.
	room := rectRoom(3, 3, 3)

	gFull := ClassifyGrid(room, 0.25)
	gComp := ClassifyGrid(room, 0.25)
	gComp.EnableCompression()

	sFull := NewIBMStencil(gFull, RigidWallBC())
	sComp := NewIBMStencil(gComp, RigidWallBC())

	// Non-uniform field.
	srcFull := makeField(gFull)
	srcComp := gComp.NewField()

	for ix := range gFull.Nx {
		for iy := range gFull.Ny {
			for iz := range gFull.Nz {
				idx := gFull.nodeIndex(ix, iy, iz)
				if gFull.Class[idx] == Exterior {
					continue
				}

				p := gFull.nodePos(ix, iy, iz)
				val := math.Sin(p.X) * math.Cos(p.Y) * p.Z
				srcFull[idx] = val
				srcComp[gComp.CompactMap[idx]] = val
			}
		}
	}

	dstFull := makeField(gFull)
	dstComp := gComp.NewField()

	sFull.ApplyLaplacian(dstFull, srcFull)
	sComp.ApplyLaplacian(dstComp, srcComp)

	// Compare results.
	for ix := range gFull.Nx {
		for iy := range gFull.Ny {
			for iz := range gFull.Nz {
				flatIdx := gFull.nodeIndex(ix, iy, iz)
				if gFull.Class[flatIdx] == Exterior {
					continue
				}

				fullVal := dstFull[flatIdx]
				compVal := dstComp[gComp.CompactMap[flatIdx]]

				if math.Abs(fullVal-compVal) > 1e-12 {
					t.Errorf("node (%d,%d,%d): full=%.15g, compact=%.15g",
						ix, iy, iz, fullVal, compVal)
				}
			}
		}
	}
}

func TestIBMStencil_CompressedFDTDMatchesFull(t *testing.T) {
	// Verify that FDTD steps produce identical results in both modes.
	room := rectRoom(3, 3, 3)

	gFull := ClassifyGrid(room, 0.25)
	gComp := ClassifyGrid(room, 0.25)
	gComp.EnableCompression()

	sFull := NewIBMStencil(gFull, RigidWallBC())
	sComp := NewIBMStencil(gComp, RigidWallBC())

	c := 343.0
	dt := 0.95 * sFull.CFLLimit(c)

	// Gaussian initial condition.
	curFull := makeField(gFull)
	curComp := gComp.NewField()
	centre := geometry.Vec3{X: 1.5, Y: 1.5, Z: 1.5}
	sigma := 0.3

	for ix := range gFull.Nx {
		for iy := range gFull.Ny {
			for iz := range gFull.Nz {
				idx := gFull.nodeIndex(ix, iy, iz)
				if gFull.Class[idx] == Exterior {
					continue
				}

				p := gFull.nodePos(ix, iy, iz)
				r2 := (p.X-centre.X)*(p.X-centre.X) +
					(p.Y-centre.Y)*(p.Y-centre.Y) +
					(p.Z-centre.Z)*(p.Z-centre.Z)
				val := math.Exp(-r2 / (2 * sigma * sigma))
				curFull[idx] = val
				curComp[gComp.CompactMap[idx]] = val
			}
		}
	}

	prevFull := makeField(gFull)
	prevComp := gComp.NewField()

	copy(prevFull, curFull)
	copy(prevComp, curComp)

	nextFull := makeField(gFull)
	nextComp := gComp.NewField()

	// Run 50 steps.
	for range 50 {
		sFull.FDTDStep(nextFull, curFull, prevFull, c, dt)
		sComp.FDTDStep(nextComp, curComp, prevComp, c, dt)

		prevFull, curFull, nextFull = curFull, nextFull, prevFull
		prevComp, curComp, nextComp = curComp, nextComp, prevComp
	}

	// Compare final state.
	maxDiff := 0.0

	for ix := range gFull.Nx {
		for iy := range gFull.Ny {
			for iz := range gFull.Nz {
				flatIdx := gFull.nodeIndex(ix, iy, iz)
				if gFull.Class[flatIdx] == Exterior {
					continue
				}

				fullVal := curFull[flatIdx]
				compVal := curComp[gComp.CompactMap[flatIdx]]
				diff := math.Abs(fullVal - compVal)

				if diff > maxDiff {
					maxDiff = diff
				}
			}
		}
	}

	t.Logf("max difference after 50 FDTD steps: %.3e", maxDiff)

	if maxDiff > 1e-10 {
		t.Errorf("compressed FDTD diverged from full: max diff = %.3e", maxDiff)
	}
}
