package hrtf

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const barycentricEpsilon = 1e-9

// BarycentricWeights returns directional barycentric weights for p relative to
// tri. The direction ray through p is intersected with the plane of the
// normalized triangle before ordinary planar barycentric coordinates are
// evaluated. Degenerate triangles, triangles whose plane passes through the
// origin, and directions that intersect behind the origin return zero weights.
func BarycentricWeights(p geometry.Vec3, tri [3]geometry.Vec3) [3]float64 {
	p = p.Normalize()
	a := tri[0].Normalize()
	b := tri[1].Normalize()

	c := tri[2].Normalize()
	if p == geometry.Vec3Zero || a == geometry.Vec3Zero || b == geometry.Vec3Zero || c == geometry.Vec3Zero {
		return [3]float64{}
	}

	ab := b.Sub(a)
	ac := c.Sub(a)
	normal := ab.Cross(ac)

	normalNorm := normal.Norm()
	if normalNorm == 0 {
		return [3]float64{}
	}

	planeOffset := normal.Dot(a)

	rayDotNormal := normal.Dot(p)
	if math.Abs(planeOffset) <= barycentricEpsilon*normalNorm ||
		math.Abs(rayDotNormal) <= barycentricEpsilon*normalNorm {
		return [3]float64{}
	}

	distance := planeOffset / rayDotNormal
	if distance <= 0 || math.IsNaN(distance) || math.IsInf(distance, 0) {
		return [3]float64{}
	}

	projected := p.Scale(distance)
	ap := projected.Sub(a)
	dotABAB := ab.Dot(ab)
	dotABAC := ab.Dot(ac)
	dotACAC := ac.Dot(ac)
	dotAPAB := ap.Dot(ab)
	dotAPAC := ap.Dot(ac)

	denominator := dotABAB*dotACAC - dotABAC*dotABAC
	if denominator <= barycentricEpsilon*dotABAB*dotACAC {
		return [3]float64{}
	}

	weightB := (dotACAC*dotAPAB - dotABAC*dotAPAC) / denominator
	weightC := (dotABAB*dotAPAC - dotABAC*dotAPAB) / denominator

	return [3]float64{1 - weightB - weightC, weightB, weightC}
}

// InterpolateHRIR returns a blended HRIR for dir using an enclosing triangle
// explicitly listed in the measurement grid, or a nearest-neighbor fallback.
func InterpolateHRIR(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64) {
	if grid == nil || len(grid.Directions) == 0 {
		return nil, nil, 0
	}

	if index := exactDirectionIndex(grid, dir); index >= 0 {
		return measurementAt(grid, index)
	}

	// Only explicit topology is safe here. Arbitrarily combining measurement
	// directions can create triangles that cross unrelated parts of the sphere.
	for _, triangle := range grid.Triangles {
		if !triangleValid(triangle, len(grid.Directions)) {
			continue
		}

		weights := BarycentricWeights(dir.Normalize(), [3]geometry.Vec3{
			grid.Directions[triangle[0]].Normalize(),
			grid.Directions[triangle[1]].Normalize(),
			grid.Directions[triangle[2]].Normalize(),
		})
		if !weightsAreValid(weights) {
			continue
		}

		left, right, delay = blendMeasurements(grid, triangle, weights)

		return left, right, delay
	}

	return LookupNearest(grid, dir)
}

func exactDirectionIndex(grid *MeasurementGrid, dir geometry.Vec3) int {
	target := dir.Normalize()
	if target == geometry.Vec3Zero {
		return -1
	}

	for index, measurement := range grid.Directions {
		candidate := measurement.Normalize()
		if candidate == geometry.Vec3Zero {
			continue
		}

		if candidate.Distance(target) <= barycentricEpsilon || math.Abs(candidate.Dot(target)-1) <= barycentricEpsilon {
			return index
		}
	}

	return -1
}

func triangleValid(triangle [3]int, count int) bool {
	return triangle[0] >= 0 && triangle[1] >= 0 && triangle[2] >= 0 && triangle[0] < count && triangle[1] < count && triangle[2] < count && triangle[0] != triangle[1] && triangle[0] != triangle[2] && triangle[1] != triangle[2]
}

func weightsAreValid(weights [3]float64) bool {
	var sum float64

	for _, weight := range weights {
		if weight < -barycentricEpsilon {
			return false
		}

		sum += weight
	}

	return math.Abs(sum-1) <= 1e-6
}

func blendMeasurements(grid *MeasurementGrid, triangle [3]int, weights [3]float64) (left, right []float64, delay float64) {
	var leftSources [3][]float64
	var rightSources [3][]float64
	var delays [3]float64
	maxLen := 0

	for i, index := range triangle {
		if index < len(grid.LeftHRIRs) {
			leftSources[i] = grid.LeftHRIRs[index]
			if len(leftSources[i]) > maxLen {
				maxLen = len(leftSources[i])
			}
		}

		if index < len(grid.RightHRIRs) {
			rightSources[i] = grid.RightHRIRs[index]
			if len(rightSources[i]) > maxLen {
				maxLen = len(rightSources[i])
			}
		}

		if index < len(grid.Delays) {
			delays[i] = grid.Delays[index]
		}
	}

	if maxLen == 0 {
		return LookupNearest(grid, grid.Directions[triangle[0]])
	}

	left = make([]float64, maxLen)
	right = make([]float64, maxLen)

	for i := range 3 {
		for sampleIndex, sample := range leftSources[i] {
			left[sampleIndex] += weights[i] * sample
		}

		for sampleIndex, sample := range rightSources[i] {
			right[sampleIndex] += weights[i] * sample
		}

		delay += weights[i] * delays[i]
	}

	return left, right, delay
}
