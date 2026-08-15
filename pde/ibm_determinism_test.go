package pde

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"sort"
	"testing"
)

// gridFractionHash returns an FNV-64a hash over every boundary node's flat
// index and its six sub-cell wall fractions.
//
// The nodes are visited in ascending flat-index order rather than by ranging
// IBMGrid.Boundary: Go randomises map iteration order, so a hash taken over the
// map directly would differ run to run.  Fractions are mixed in through
// math.Float64bits so a one-ulp drift changes the hash, which is the whole
// point of the guard.
//
// scene.GeometryHash uses the same FNV-64a construction; see scene/hash.go.
func gridFractionHash(g *IBMGrid) uint64 {
	indices := make([]int, 0, len(g.Boundary))
	for idx := range g.Boundary {
		indices = append(indices, idx)
	}

	sort.Ints(indices)

	h := fnv.New64a()

	var buf [8]byte

	write := func(bits uint64) {
		binary.LittleEndian.PutUint64(buf[:], bits)
		_, _ = h.Write(buf[:])
	}

	for _, idx := range indices {
		write(uint64(idx))

		info := g.Boundary[idx]

		for a := range 3 {
			for d := range 2 {
				write(math.Float64bits(info.Frac[a][d]))
			}
		}
	}

	return h.Sum64()
}

// gridClassCounts counts nodes per class over the whole padded grid.
func gridClassCounts(g *IBMGrid) (interior, boundary, exterior int) {
	for _, c := range g.Class {
		switch c {
		case Interior:
			interior++
		case Boundary:
			boundary++
		case Exterior:
			exterior++
		}
	}

	return interior, boundary, exterior
}

// TestClassifyGridGolden pins the exact output of ClassifyGrid for rooms whose
// dimensions are exact multiples of the grid spacing — the case where a wall
// lands on a node plane and the classification used to be decided by a
// floating-point tie.
//
// The goldens below were recorded on amd64 and re-verified on arm64 under
// qemu-aarch64, where they are bit-identical.  That is only true because
// ClassifyGrid no longer lets FMA contraction move the grid origin; see
// offsetFromOrigin and sideOf.
//
// If this test fails:
//
//   - You changed the classifier on purpose (grid sizing, padding, the
//     node-plane tolerance, the wall-fraction clamp).  Re-record the goldens
//     from the reported actual values and check the arm64 run still agrees.
//   - The test passes on one architecture and fails on another, or the values
//     drift without any deliberate change here.  That is a portability
//     regression: do NOT re-record.  Fix the arithmetic instead — see
//     docs/maintenance.md on FMA contraction and geometric predicates.
func TestClassifyGridGolden(t *testing.T) {
	tests := []struct {
		name     string
		room     *ConvexRoom
		h        float64
		nx       int
		ny       int
		nz       int
		interior int
		boundary int
		exterior int
		fracHash uint64
	}{
		{
			// Fully exact in binary: dimensions, spacing and every node
			// position are representable, so this is the control case.
			name:     "cube_4m_h0.5",
			room:     rectRoom(4, 4, 4),
			h:        0.5,
			nx:       11,
			ny:       11,
			nz:       11,
			interior: 125,
			boundary: 218,
			exterior: 988,
			fracHash: 0xa894f8074782f9b6,
		},
		{
			// The shoebox from the IBM validation suite.  Every dimension is
			// an exact multiple of h, so every wall sits on a node plane.
			name:     "shoebox_3x2.5x2_h0.05",
			room:     rectRoom(3.0, 2.5, 2.0),
			h:        0.05,
			nx:       63,
			ny:       53,
			nz:       43,
			interior: 99123,
			boundary: 13626,
			exterior: 30828,
			fracHash: 0xdcaf8c779e3dd62c,
		},
		{
			// The Sabine energy-decay fixture.
			name:     "sabine_8x6x5_h0.2",
			room:     rectRoom(8, 6, 5),
			h:        0.2,
			nx:       43,
			ny:       33,
			nz:       29,
			interior: 22977,
			boundary: 5298,
			exterior: 12876,
			fracHash: 0x354507614d27240e,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := ClassifyGrid(tc.room, tc.h)
			interior, boundary, exterior := gridClassCounts(g)
			hash := gridFractionHash(g)

			t.Logf("ACTUAL nx=%d ny=%d nz=%d interior=%d boundary=%d exterior=%d fracHash=0x%016x",
				g.Nx, g.Ny, g.Nz, interior, boundary, exterior, hash)

			if g.Nx != tc.nx || g.Ny != tc.ny || g.Nz != tc.nz {
				t.Errorf("grid dims = %dx%dx%d, want %dx%dx%d",
					g.Nx, g.Ny, g.Nz, tc.nx, tc.ny, tc.nz)
			}

			if interior != tc.interior {
				t.Errorf("interior nodes = %d, want %d", interior, tc.interior)
			}

			if boundary != tc.boundary {
				t.Errorf("boundary nodes = %d, want %d", boundary, tc.boundary)
			}

			if exterior != tc.exterior {
				t.Errorf("exterior nodes = %d, want %d", exterior, tc.exterior)
			}

			if hash != tc.fracHash {
				t.Errorf("fraction hash = 0x%016x, want 0x%016x", hash, tc.fracHash)
			}
		})
	}
}

// gridClassHash hashes only the interior/boundary/exterior classification of
// every node, never the cut fractions.  That split is the point: for a room
// whose planes are not axis-aligned the fractions carry the last bits of the
// plane coefficients and are not portable, while the classification is — which
// is exactly the property Phase 26 established and therefore the one worth
// pinning.
func gridClassHash(g *IBMGrid) uint64 {
	h := fnv.New64a()

	var buf [8]byte

	// g.Class is index-ordered, so hashing it directly is already a
	// position-dependent digest — no need to walk the grid by coordinate.
	for _, c := range g.Class {
		binary.LittleEndian.PutUint64(buf[:], uint64(c))
		_, _ = h.Write(buf[:])
	}

	return h.Sum64()
}

// TestClassifyGridClassGolden pins the classification of rooms whose walls are
// oblique, which TestClassifyGridGolden cannot cover.
//
// That test hashes the cut fractions, so it is restricted to axis-aligned
// fixtures whose planes are exact in binary — and those never exercise the
// multi-product FMA path in sideOf that the Phase 26 defect lived in.  The
// invariant test does run the triangle, but it only asserts a local property
// (exterior neighbour => 0 < Frac <= 1) and stays satisfied while nodes migrate
// between Interior, Boundary and Exterior, as long as each architecture is
// internally consistent — which is precisely the failure mode being guarded
// against, since amd64 and arm64 were each internally consistent and disagreed
// with each other.
//
// Hashing the classification alone closes that gap: it is portable, and it
// changes the moment a single node moves between classes.
func TestClassifyGridClassGolden(t *testing.T) {
	tests := []struct {
		name      string
		room      *ConvexRoom
		h         float64
		nx        int
		ny        int
		nz        int
		interior  int
		boundary  int
		exterior  int
		classHash uint64
	}{
		{
			// Oblique walls from math.Sqrt(3): the fraction hash is not
			// portable here, but the classification is.
			name:      "triangle_L3_h0.1",
			room:      equilateralTriangleRoom(3.0, 1.5, 3.0*math.Sqrt(3)/3, 10.0),
			h:         0.1,
			nx:        33,
			ny:        29,
			nz:        103,
			interior:  29003,
			boundary:  8122,
			exterior:  61446,
			classHash: 0xb45880fa773af884,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := ClassifyGrid(tc.room, tc.h)
			interior, boundary, exterior := gridClassCounts(g)
			hash := gridClassHash(g)

			t.Logf("ACTUAL nx=%d ny=%d nz=%d interior=%d boundary=%d exterior=%d classHash=0x%016x",
				g.Nx, g.Ny, g.Nz, interior, boundary, exterior, hash)

			if g.Nx != tc.nx || g.Ny != tc.ny || g.Nz != tc.nz {
				t.Errorf("grid dims = %dx%dx%d, want %dx%dx%d",
					g.Nx, g.Ny, g.Nz, tc.nx, tc.ny, tc.nz)
			}

			if interior != tc.interior {
				t.Errorf("interior nodes = %d, want %d", interior, tc.interior)
			}

			if boundary != tc.boundary {
				t.Errorf("boundary nodes = %d, want %d", boundary, tc.boundary)
			}

			if exterior != tc.exterior {
				t.Errorf("exterior nodes = %d, want %d", exterior, tc.exterior)
			}

			if hash != tc.classHash {
				t.Errorf("class hash = 0x%016x, want 0x%016x", hash, tc.classHash)
			}
		})
	}
}

// TestClassifyGridFracInvariant checks the contract that makes the classifier
// independent of how a node-plane tie rounds:
//
//	neighbour exterior or out of bounds  =>  0 < Frac <= 1
//	neighbour active (interior/boundary) =>  Frac == 0 exactly
//
// The two cases are disjoint, so Frac == 0 identifies "no wall here" and
// nothing else.  Before this held, a wall that rounded to a fraction just above
// 1 was folded into the same 0 sentinel and the stencil silently applied a
// pressure-release condition to an exterior-facing direction — the opposite of
// the configured WallBC, and different per architecture.
//
// The triangle fixture is exercised here but deliberately not in
// TestClassifyGridGolden: its wall planes are derived from math.Sqrt(3) inside
// the test helper and differ by a couple of ulps between architectures, so its
// fraction hash is not portable.  This invariant is, because it does not depend
// on where the tie falls.
func TestClassifyGridFracInvariant(t *testing.T) {
	tests := []struct {
		name string
		room *ConvexRoom
		h    float64
	}{
		{name: "cube_4m_h0.5", room: rectRoom(4, 4, 4), h: 0.5},
		{name: "shoebox_3x2.5x2_h0.05", room: rectRoom(3.0, 2.5, 2.0), h: 0.05},
		{name: "sabine_8x6x5_h0.2", room: rectRoom(8, 6, 5), h: 0.2},
		{
			name: "triangle_L3_h0.1",
			room: equilateralTriangleRoom(3.0, 1.5, 3.0*math.Sqrt(3)/3, 10.0),
			h:    0.1,
		},
	}

	// Neighbour offsets in the same [axis][dir] order the classifier uses.
	neighbour := [3][2][3]int{
		{{-1, 0, 0}, {1, 0, 0}},
		{{0, -1, 0}, {0, 1, 0}},
		{{0, 0, -1}, {0, 0, 1}},
	}

	axisName := [3]string{"X", "Y", "Z"}
	dirName := [2]string{"-", "+"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := ClassifyGrid(tc.room, tc.h)

			if len(g.Boundary) == 0 {
				t.Fatal("expected at least one boundary node")
			}

			active := func(ix, iy, iz int) bool {
				if ix < 0 || ix >= g.Nx || iy < 0 || iy >= g.Ny || iz < 0 || iz >= g.Nz {
					return false
				}

				return g.Class[g.nodeIndex(ix, iy, iz)] != Exterior
			}

			for _, ref := range g.BoundaryIdx {
				for a := range 3 {
					for d := range 2 {
						o := neighbour[a][d]
						hasWall := !active(ref.Ix+o[0], ref.Iy+o[1], ref.Iz+o[2])
						frac := ref.Info.Frac[a][d]

						switch {
						case hasWall && !(frac > 0 && frac <= 1):
							t.Errorf(
								"node (%d,%d,%d) dir %s%s: exterior neighbour but Frac = %g, want in (0, 1]",
								ref.Ix, ref.Iy, ref.Iz, dirName[d], axisName[a], frac,
							)
						case !hasWall && frac != 0:
							t.Errorf(
								"node (%d,%d,%d) dir %s%s: active neighbour but Frac = %g, want exactly 0",
								ref.Ix, ref.Iy, ref.Iz, dirName[d], axisName[a], frac,
							)
						}
					}
				}
			}
		})
	}
}

// TestNodePositionsFMAFree guards the arithmetic that keeps grid geometry
// architecture-independent.
//
// offsetFromOrigin must round i*h to float64 before adding it to the base.  Go
// is allowed to fuse `base + i*h` into a single FMA, which arm64 does and amd64
// does not; the unfused and fused results differ by up to an ulp, which is
// enough to move a wall from one side of a node plane to the other.  Rewriting
// the body back into a contractable form would still pass every physics test
// while quietly reintroducing the divergence, so it is pinned here.
func TestNodePositionsFMAFree(t *testing.T) {
	t.Run("explicit_rounding", func(t *testing.T) {
		tests := []struct {
			name string
			base float64
			i    int
			h    float64
		}{
			{name: "zero_index", base: -0.1, i: 0, h: 0.1},
			{name: "exact_binary", base: -0.5, i: 9, h: 0.5},
			{name: "tenth_spacing", base: -0.1, i: 101, h: 0.1},
			{name: "negative_index", base: 1.55, i: -31, h: 0.05},
			{name: "sabine_spacing", base: -0.2, i: 42, h: 0.2},
			{name: "triangle_ceiling", base: -0.10000000000000053, i: 101, h: 0.1},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				want := tc.base + float64(float64(tc.i)*tc.h)
				got := offsetFromOrigin(tc.base, tc.i, tc.h)

				if math.Float64bits(got) != math.Float64bits(want) {
					t.Errorf("offsetFromOrigin(%v, %d, %v) = %v (%016x), want %v (%016x)",
						tc.base, tc.i, tc.h, got, math.Float64bits(got),
						want, math.Float64bits(want))
				}
			})
		}
	})

	// The triangle fixture is the geometry that exposed the divergence: its
	// ceiling at z = 10 with h = 0.1 lands exactly on a node plane, and the
	// origin is where the FMA drift entered.  Pinning the origin's exact bits
	// catches any change to how it is derived.
	t.Run("triangle_origin_bits", func(t *testing.T) {
		room := equilateralTriangleRoom(3.0, 1.5, 3.0*math.Sqrt(3)/3, 10.0)
		g := ClassifyGrid(room, 0.1)

		t.Logf("ACTUAL origin bits X=%016x Y=%016x Z=%016x",
			math.Float64bits(g.Origin.X),
			math.Float64bits(g.Origin.Y),
			math.Float64bits(g.Origin.Z))

		const (
			wantX = uint64(0xbfb99999999999a0)
			wantY = uint64(0x3fe87b66780ff2de)
			wantZ = uint64(0xbfb99999999999c0)
		)

		if got := math.Float64bits(g.Origin.X); got != wantX {
			t.Errorf("origin.X bits = %016x (%v), want %016x", got, g.Origin.X, wantX)
		}

		if got := math.Float64bits(g.Origin.Y); got != wantY {
			t.Errorf("origin.Y bits = %016x (%v), want %016x", got, g.Origin.Y, wantY)
		}

		if got := math.Float64bits(g.Origin.Z); got != wantZ {
			t.Errorf("origin.Z bits = %016x (%v), want %016x", got, g.Origin.Z, wantZ)
		}
	})
}
