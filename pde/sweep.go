package pde

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
	"github.com/cwbudde/algo-pde/poisson"
)

// SweepShoebox computes a frequency response for a shoebox room.
func SweepShoebox(room *scene.Shoebox, src, rcv geometry.Vec3, cfg SweepConfig) (*TransferFunction, error) {
	if room == nil {
		return nil, fmt.Errorf("shoebox room is nil")
	}
	if cfg.NumPoints <= 0 {
		return nil, fmt.Errorf("NumPoints must be positive")
	}
	if cfg.FreqMin <= 0 || cfg.FreqMax <= 0 || cfg.FreqMax < cfg.FreqMin {
		return nil, fmt.Errorf("invalid frequency sweep range")
	}

	freqs := make([]float64, cfg.NumPoints)
	h := make([]complex128, cfg.NumPoints)
	step := 0.0
	if cfg.NumPoints > 1 {
		step = (cfg.FreqMax - cfg.FreqMin) / float64(cfg.NumPoints-1)
	}

	for i := range freqs {
		freqs[i] = cfg.FreqMin + float64(i)*step
		value, err := solveAtFrequency(room, src, rcv, freqs[i], cfg.BoundaryCondition)
		if err != nil {
			return nil, err
		}
		h[i] = value
	}

	return &TransferFunction{Freqs: freqs, H: h}, nil
}

func solveAtFrequency(room *scene.Shoebox, src, rcv geometry.Vec3, freqHz float64, boundaryCondition string) (complex128, error) {
	alpha := math.Pow(2*math.Pi*freqHz/acoustics.SpeedOfSound, 2)
	hx, hy, hz := gridSpacing(room, freqHz)
	nx := maxInt(4, int(math.Ceil(room.Width/hx)))
	ny := maxInt(4, int(math.Ceil(room.Depth/hy)))
	nz := maxInt(4, int(math.Ceil(room.Height/hz)))

	hx = room.Width / float64(nx)
	hy = room.Depth / float64(ny)
	hz = room.Height / float64(nz)

	bc := boundaryType(boundaryCondition)
	plan, err := poisson.NewHelmholtzPlan(3, []int{nx, ny, nz}, []float64{hx, hy, hz}, []poisson.BCType{bc, bc, bc}, alpha)
	if err != nil {
		return 0, err
	}

	rhs := make([]float64, nx*ny*nz)
	ix, iy, iz := nearestCell(src, nx, ny, nz, hx, hy, hz)
	rhs[cellIndex(ix, iy, iz, ny, nz)] = 1

	solution := make([]float64, nx*ny*nz)
	if err := plan.Solve(solution, rhs); err != nil {
		return 0, err
	}

	rx, ry, rz := nearestCell(rcv, nx, ny, nz, hx, hy, hz)
	value := solution[cellIndex(rx, ry, rz, ny, nz)]
	phase := -2 * math.Pi * freqHz * src.Distance(rcv) / acoustics.SpeedOfSound

	return complex(value*math.Cos(phase), value*math.Sin(phase)), nil
}

func gridSpacing(room *scene.Shoebox, freqHz float64) (float64, float64, float64) {
	pointsPerWavelength := 8.0
	spacing := acoustics.SpeedOfSound / (pointsPerWavelength * freqHz)
	if spacing <= 0 {
		spacing = minPositive(room.Width, room.Depth, room.Height) / 8
	}
	return spacing, spacing, spacing
}

func boundaryType(name string) poisson.BCType {
	switch name {
	case "dirichlet":
		return poisson.Dirichlet
	case "periodic":
		return poisson.Periodic
	default:
		return poisson.Neumann
	}
}

func minPositive(values ...float64) float64 {
	min := math.Inf(1)
	for _, value := range values {
		if value > 0 && value < min {
			min = value
		}
	}
	if math.IsInf(min, 1) {
		return 1
	}
	return min
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
