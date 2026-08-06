package pde

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// benchRoom creates a rectangular room with the given dimensions and grid spacing,
// returning the grid and a rigid-wall stencil.
func benchRoom(w, d, h, spacing float64) (*IBMGrid, *IBMStencil) {
	room := rectRoom(w, d, h)
	g := ClassifyGrid(room, spacing)
	s := NewIBMStencil(g, RigidWallBC())

	return g, s
}

// benchDiamondRoom creates a 45°-rotated diamond room (low fill ratio).
func benchDiamondRoom(size, spacing float64) (*IBMGrid, *IBMStencil) {
	centre := geometry.Vec3{X: size / 2, Y: size / 2, Z: size / 2}
	r := size / 2

	diamondVerts2D := [4]geometry.Vec3{
		{X: centre.X, Y: centre.Y - r},
		{X: centre.X + r, Y: centre.Y},
		{X: centre.X, Y: centre.Y + r},
		{X: centre.X - r, Y: centre.Y},
	}

	walls := make([]geometry.Plane, 0, 6)
	walls = append(
		walls,
		geometry.Plane{Normal: geometry.Vec3{Z: 1}, Distance: 0},
		geometry.Plane{Normal: geometry.Vec3{Z: -1}, Distance: -size},
	)

	for i := range 4 {
		a := diamondVerts2D[i]
		b := diamondVerts2D[(i+1)%4]
		edge := b.Sub(a)
		perp := geometry.Vec3{X: edge.Y, Y: -edge.X}
		mid := a.Add(b).Scale(0.5)

		if perp.Dot(centre.Sub(mid)) < 0 {
			perp = perp.Neg()
		}

		walls = append(walls, geometry.NewPlaneFromPointNormal(a, perp))
	}

	verts := []geometry.Vec3{
		{X: centre.X, Y: centre.Y - r, Z: 0},
		{X: centre.X + r, Y: centre.Y, Z: 0},
		{X: centre.X, Y: centre.Y + r, Z: 0},
		{X: centre.X - r, Y: centre.Y, Z: 0},
		{X: centre.X, Y: centre.Y - r, Z: size},
		{X: centre.X + r, Y: centre.Y, Z: size},
		{X: centre.X, Y: centre.Y + r, Z: size},
		{X: centre.X - r, Y: centre.Y, Z: size},
	}

	room, err := NewConvexRoom(walls, verts)
	if err != nil {
		panic(err)
	}

	g := ClassifyGrid(room, spacing)
	s := NewIBMStencil(g, RigidWallBC())

	return g, s
}

// plainCartesianFDTDStep is a minimal 7-point FDTD step over all nodes
// (no classification, no boundary handling). Used as baseline reference.
func plainCartesianFDTDStep(pNext, pCur, pPrev []float64, nx, ny, nz int, cdt2, invH2 float64) {
	nyNz := ny * nz

	for ix := 1; ix < nx-1; ix++ {
		for iy := 1; iy < ny-1; iy++ {
			for iz := 1; iz < nz-1; iz++ {
				idx := ix*nyNz + iy*nz + iz
				u := pCur[idx]
				lap := (6*u -
					pCur[idx-nyNz] - pCur[idx+nyNz] -
					pCur[idx-nz] - pCur[idx+nz] -
					pCur[idx-1] - pCur[idx+1]) * invH2

				pNext[idx] = 2*u - pPrev[idx] - cdt2*lap
			}
		}
	}
}

// initGaussianField fills a Gaussian pulse into active nodes.
func initGaussianField(g *IBMGrid, field []float64) {
	cx := g.Origin.X + float64(g.Nx/2)*g.H
	cy := g.Origin.Y + float64(g.Ny/2)*g.H
	cz := g.Origin.Z + float64(g.Nz/2)*g.H
	sigma := float64(g.Nx) * g.H * 0.15

	for _, idx := range g.InteriorIdx {
		iz := idx % g.Nz
		iy := (idx / g.Nz) % g.Ny
		ix := idx / (g.Ny * g.Nz)
		p := g.nodePos(ix, iy, iz)
		r2 := (p.X-cx)*(p.X-cx) + (p.Y-cy)*(p.Y-cy) + (p.Z-cz)*(p.Z-cz)
		field[idx] = math.Exp(-r2 / (2 * sigma * sigma))
	}

	for _, ref := range g.BoundaryIdx {
		p := g.nodePos(ref.Ix, ref.Iy, ref.Iz)
		r2 := (p.X-cx)*(p.X-cx) + (p.Y-cy)*(p.Y-cy) + (p.Z-cz)*(p.Z-cz)
		field[ref.Idx] = math.Exp(-r2 / (2 * sigma * sigma))
	}
}

func BenchmarkIBM_FDTDStep_RectRoom(b *testing.B) {
	g, stencil := benchRoom(4, 4, 4, 0.1)

	pCur := makeField(g)
	pPrev := makeField(g)
	pNext := makeField(g)

	initGaussianField(g, pCur)
	copy(pPrev, pCur)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	total := g.Nx * g.Ny * g.Nz
	active := g.NumActive()
	b.Logf("grid: %dx%dx%d = %d nodes, active: %d (%.1f%%)",
		g.Nx, g.Ny, g.Nz, total, active, 100*float64(active)/float64(total))

	b.ResetTimer()

	for b.Loop() {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}
}

func BenchmarkPlainCartesian_FDTDStep(b *testing.B) {
	// Same grid dimensions as the rectangular IBM bench for direct comparison.
	g, stencil := benchRoom(4, 4, 4, 0.1)

	n := g.Nx * g.Ny * g.Nz

	pCur := make([]float64, n)
	pPrev := make([]float64, n)
	pNext := make([]float64, n)

	// Fill all interior with a Gaussian (no classification needed).
	cx := float64(g.Nx / 2)
	cy := float64(g.Ny / 2)
	cz := float64(g.Nz / 2)

	for ix := range g.Nx {
		for iy := range g.Ny {
			for iz := range g.Nz {
				dx := float64(ix) - cx
				dy := float64(iy) - cy
				dz := float64(iz) - cz
				pCur[ix*g.Ny*g.Nz+iy*g.Nz+iz] = math.Exp(-(dx*dx + dy*dy + dz*dz) / 100)
			}
		}
	}

	copy(pPrev, pCur)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)
	invH2 := 1.0 / (g.H * g.H)
	cdt2 := c * c * dt * dt

	b.Logf("grid: %dx%dx%d = %d nodes (all active)", g.Nx, g.Ny, g.Nz, n)
	b.ResetTimer()

	for b.Loop() {
		plainCartesianFDTDStep(pNext, pCur, pPrev, g.Nx, g.Ny, g.Nz, cdt2, invH2)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}
}

func BenchmarkIBM_FDTDStep_DiamondRoom(b *testing.B) {
	g, stencil := benchDiamondRoom(4, 0.1)

	pCur := makeField(g)
	pPrev := makeField(g)
	pNext := makeField(g)

	initGaussianField(g, pCur)
	copy(pPrev, pCur)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	total := g.Nx * g.Ny * g.Nz
	active := g.NumActive()
	b.Logf("grid: %dx%dx%d = %d nodes, active: %d (%.1f%%)",
		g.Nx, g.Ny, g.Nz, total, active, 100*float64(active)/float64(total))

	b.ResetTimer()

	for b.Loop() {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}
}

func BenchmarkIBM_ApplyLaplacian_RectRoom(b *testing.B) {
	g, stencil := benchRoom(4, 4, 4, 0.1)

	src := makeField(g)
	dst := makeField(g)

	initGaussianField(g, src)

	b.ResetTimer()

	for b.Loop() {
		stencil.ApplyLaplacian(dst, src)
	}
}

func BenchmarkIBM_FDTDStep_DiamondRoom_Compressed(b *testing.B) {
	g, _ := benchDiamondRoom(4, 0.1)
	g.EnableCompression()

	stencil := NewIBMStencil(g, RigidWallBC())

	pCur := g.NewField()
	pPrev := g.NewField()
	pNext := g.NewField()

	// Init Gaussian into compact field.
	cm := g.CompactMap
	cx := g.Origin.X + float64(g.Nx/2)*g.H
	cy := g.Origin.Y + float64(g.Ny/2)*g.H
	cz := g.Origin.Z + float64(g.Nz/2)*g.H
	sigma := float64(g.Nx) * g.H * 0.15

	for _, idx := range g.InteriorIdx {
		iz := idx % g.Nz
		iy := (idx / g.Nz) % g.Ny
		ix := idx / (g.Ny * g.Nz)
		p := g.nodePos(ix, iy, iz)
		r2 := (p.X-cx)*(p.X-cx) + (p.Y-cy)*(p.Y-cy) + (p.Z-cz)*(p.Z-cz)
		pCur[cm[idx]] = math.Exp(-r2 / (2 * sigma * sigma))
	}

	for _, ref := range g.BoundaryIdx {
		p := g.nodePos(ref.Ix, ref.Iy, ref.Iz)
		r2 := (p.X-cx)*(p.X-cx) + (p.Y-cy)*(p.Y-cy) + (p.Z-cz)*(p.Z-cz)
		pCur[cm[ref.Idx]] = math.Exp(-r2 / (2 * sigma * sigma))
	}

	copy(pPrev, pCur)

	c := 343.0
	dt := 0.95 * stencil.CFLLimit(c)

	total := g.Nx * g.Ny * g.Nz
	active := g.NumActive()
	fieldMB := float64(g.FieldSize()*8*3) / (1024 * 1024)
	fullMB := float64(total*8*3) / (1024 * 1024)
	b.Logf("grid: %dx%dx%d = %d nodes, active: %d (%.1f%%), fields: %.1f MB (full: %.1f MB)",
		g.Nx, g.Ny, g.Nz, total, active, 100*float64(active)/float64(total), fieldMB, fullMB)

	b.ResetTimer()

	for b.Loop() {
		stencil.FDTDStep(pNext, pCur, pPrev, c, dt)
		pPrev, pCur, pNext = pCur, pNext, pPrev
	}
}
