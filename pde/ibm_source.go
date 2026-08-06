package pde

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// SourceMode selects how the source signal is combined with the pressure field.
type SourceMode int

const (
	// SoftSource adds the source signal to the existing pressure (additive).
	// This is transparent to wave propagation — waves pass through unaffected.
	SoftSource SourceMode = iota

	// HardSource overwrites the pressure at the source node each time step.
	// This creates an ideal point source but reflects incoming waves.
	HardSource
)

// IBMSource represents a point source on an IBM grid at an arbitrary position.
// The source injects signal into the nearest grid node.
type IBMSource struct {
	// Room geometry for position validation.
	Room *ConvexRoom

	// Grid on which the source operates.
	Grid *IBMGrid

	// Position of the source in world coordinates.
	Position geometry.Vec3

	// NodeIdx is the flat grid index of the nearest active node.
	NodeIdx int

	// Mode selects soft (additive) or hard (overwrite) injection.
	Mode SourceMode
}

// NewIBMSource creates a point source at the given position on the IBM grid.
// Returns an error if the position is outside the room geometry.
func NewIBMSource(room *ConvexRoom, grid *IBMGrid, pos geometry.Vec3, mode SourceMode) (*IBMSource, error) {
	if room == nil {
		return nil, errors.New("source room is nil")
	}

	if grid == nil {
		return nil, errors.New("source grid is nil")
	}

	if !room.PointInside(pos) {
		return nil, errors.New("source position is outside the room")
	}

	// Find nearest active grid node.
	ix, iy, iz := nearestNode(grid, pos)
	idx := grid.nodeIndex(ix, iy, iz)

	if grid.Class[idx] == Exterior {
		// Snap failed — search neighbors for nearest active node.
		idx = findNearestActive(grid, ix, iy, iz)
		if idx < 0 {
			return nil, errors.New("no active grid node near source position")
		}
	}

	return &IBMSource{
		Room:     room,
		Grid:     grid,
		Position: pos,
		NodeIdx:  idx,
		Mode:     mode,
	}, nil
}

// Inject adds or sets the source signal value into the pressure field
// at the source node, depending on the source mode.
func (s *IBMSource) Inject(field []float64, signal float64) {
	if s == nil || s.Grid == nil {
		return
	}

	idx := s.NodeIdx
	if s.Grid.Compressed {
		if idx < 0 || idx >= len(s.Grid.CompactMap) {
			return
		}

		idx = s.Grid.CompactMap[idx]
	}

	if idx < 0 || idx >= len(field) {
		return
	}

	switch s.Mode {
	case SoftSource:
		field[idx] += signal
	case HardSource:
		field[idx] = signal
	}
}

// nearestNode finds the grid indices closest to a world position.
func nearestNode(g *IBMGrid, pos geometry.Vec3) (int, int, int) {
	ix := int(math.Round((pos.X - g.Origin.X) / g.H))
	iy := int(math.Round((pos.Y - g.Origin.Y) / g.H))
	iz := int(math.Round((pos.Z - g.Origin.Z) / g.H))

	ix = clampInt(ix, 0, g.Nx-1)
	iy = clampInt(iy, 0, g.Ny-1)
	iz = clampInt(iz, 0, g.Nz-1)

	return ix, iy, iz
}

// findNearestActive searches for the nearest active (Interior or Boundary)
// node to the given grid coordinates using expanding shells.
func findNearestActive(g *IBMGrid, cx, cy, cz int) int {
	for r := 1; r <= max(g.Nx, g.Ny, g.Nz); r++ {
		bestIdx := -1
		bestDist := math.Inf(1)

		for dx := -r; dx <= r; dx++ {
			for dy := -r; dy <= r; dy++ {
				for dz := -r; dz <= r; dz++ {
					if abs(dx) != r && abs(dy) != r && abs(dz) != r {
						continue // only check shell, not interior
					}

					nx, ny, nz := cx+dx, cy+dy, cz+dz
					if nx < 0 || nx >= g.Nx || ny < 0 || ny >= g.Ny || nz < 0 || nz >= g.Nz {
						continue
					}

					idx := g.nodeIndex(nx, ny, nz)
					if g.Class[idx] == Exterior {
						continue
					}

					d := float64(dx*dx + dy*dy + dz*dz)
					if d < bestDist {
						bestDist = d
						bestIdx = idx
					}
				}
			}
		}

		if bestIdx >= 0 {
			return bestIdx
		}
	}

	return -1
}

// GaussianPulse returns the value of a Gaussian pulse at time t.
//
//	g(t) = exp(-((t - t0) / σ)²)
//
// where t0 is the pulse center and σ controls the width.
// The bandwidth is approximately 1/(π·σ) Hz.
func GaussianPulse(t, t0, sigma float64) float64 {
	if !finite(t) || !finite(t0) || !finite(sigma) || sigma <= 0 {
		return 0
	}

	x := (t - t0) / sigma

	return math.Exp(-x * x)
}

// SineBurst returns the value of a windowed sine burst at time t.
//
//	s(t) = sin(2π·f·t) · w(t)
//
// where w(t) is a Hann window over [0, nCycles/f].
// The burst contains exactly nCycles complete cycles of frequency f.
func SineBurst(t, freqHz float64, nCycles int) float64 {
	if !finite(t) || !finite(freqHz) || freqHz <= 0 || nCycles <= 0 {
		return 0
	}

	duration := float64(nCycles) / freqHz
	if !finite(duration) || duration <= 0 {
		return 0
	}

	if t < 0 || t > duration {
		return 0
	}

	// Hann window.
	w := 0.5 * (1 - math.Cos(2*math.Pi*t/duration))

	return math.Sin(2*math.Pi*freqHz*t) * w
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}
