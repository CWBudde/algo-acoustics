package hrtf

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const barycentricEpsilon = 1e-9

// BarycentricWeights returns the barycentric weights of p relative to tri.
func BarycentricWeights(p geometry.Vec3, tri [3]geometry.Vec3) [3]float64 {
	totalArea := triangleArea(tri[0], tri[1], tri[2])
	if totalArea <= 0 {
		return [3]float64{}
	}

	return [3]float64{
		triangleArea(p, tri[1], tri[2]) / totalArea,
		triangleArea(tri[0], p, tri[2]) / totalArea,
		triangleArea(tri[0], tri[1], p) / totalArea,
	}
}

// InterpolateHRIR returns a blended HRIR for dir using the enclosing triangle
// when one can be found, or a nearest-neighbor fallback otherwise.
func InterpolateHRIR(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64) {
	if grid == nil || len(grid.Directions) == 0 {
		return nil, nil, 0
	}

	if index := exactDirectionIndex(grid, dir); index >= 0 {
		return measurementAt(grid, index)
	}

	triangles := grid.Triangles
	if len(triangles) == 0 {
		triangles = allMeasurementTriangles(len(grid.Directions))
	}

	for _, triangle := range triangles {
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

func allMeasurementTriangles(count int) [][3]int {
	if count < 3 {
		return nil
	}

	triangles := make([][3]int, 0, count*(count-1)*(count-2)/6)
	for i := 0; i < count-2; i++ {
		for j := i + 1; j < count-1; j++ {
			for k := j + 1; k < count; k++ {
				triangles = append(triangles, [3]int{i, j, k})
			}
		}
	}

	return triangles
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
	for i := 0; i < 3; i++ {
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

func triangleArea(a, b, c geometry.Vec3) float64 {
	return b.Sub(a).Cross(c.Sub(a)).Norm() * 0.5
}