package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// LaunchConfig controls Monte Carlo ray emission for the late-field model.
type LaunchConfig struct {
	NumRays                    int
	MaxBounces                 int
	MaxTimeSeconds             float64
	SpeedOfSound               float64
	ReflectionStrategy         ReflectionStrategy
	EnergyTerminationThreshold float64
	DiffractionAngularThreshold float64
	DiffractionConeSamples      int
}

// FibonacciSphere returns near-uniform unit vectors on the sphere.
func FibonacciSphere(n int) []geometry.Vec3 {
	if n <= 0 {
		return nil
	}

	out := make([]geometry.Vec3, n)
	phi := (1 + math.Sqrt(5)) / 2
	denom := phi * phi

	for i := range n {
		z := 1 - (2*float64(i)+1)/float64(n)
		theta := 2 * math.Pi * float64(i) / denom
		radius := math.Sqrt(math.Max(0, 1-z*z))
		out[i] = geometry.Vec3{
			X: radius * math.Cos(theta),
			Y: radius * math.Sin(theta),
			Z: z,
		}
	}

	return out
}

// StratifiedDirections returns a deterministic jittered spherical grid.
func StratifiedDirections(n int) []geometry.Vec3 {
	if n <= 0 {
		return nil
	}

	rows := int(math.Ceil(math.Sqrt(float64(n))))
	cols := int(math.Ceil(float64(n) / float64(rows)))
	out := make([]geometry.Vec3, 0, n)

	for row := 0; row < rows && len(out) < n; row++ {
		for col := 0; col < cols && len(out) < n; col++ {
			jitterU := pseudoRandom2(row, col)
			jitterV := pseudoRandom2(col, row)
			u := (float64(col) + 0.5 + 0.35*(jitterU-0.5)) / float64(cols)
			v := (float64(row) + 0.5 + 0.35*(jitterV-0.5)) / float64(rows)
			v = math.Min(math.Max(v, 0), 1)
			z := 1 - 2*v
			radius := math.Sqrt(math.Max(0, 1-z*z))
			phi := 2 * math.Pi * u
			out = append(out, geometry.Vec3{
				X: radius * math.Cos(phi),
				Y: radius * math.Sin(phi),
				Z: z,
			})
		}
	}

	return out
}

// LaunchRays emits rays from src in uniformly distributed directions.
func LaunchRays(src geometry.Vec3, cfg LaunchConfig) []geometry.Ray {
	if cfg.NumRays <= 0 {
		return nil
	}

	directions := FibonacciSphere(cfg.NumRays)

	rays := make([]geometry.Ray, len(directions))
	for i, dir := range directions {
		rays[i] = geometry.NewRay(src, dir)
	}

	return rays
}

func pseudoRandom2(a, b int) float64 {
	x := math.Sin(float64(a)*12.9898+float64(b)*78.233) * 43758.5453
	return x - math.Floor(x)
}
