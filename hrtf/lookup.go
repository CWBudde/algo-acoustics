package hrtf

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// MeasurementGrid stores the measured HRTF directions and HRIRs for a dataset.
type MeasurementGrid struct {
	Directions []geometry.Vec3 `json:"directions,omitempty"`
	LeftHRIRs  [][]float64     `json:"leftHrir,omitempty"`
	RightHRIRs [][]float64     `json:"rightHrir,omitempty"`
	Delays     []float64       `json:"delays,omitempty"`
	Triangles  [][3]int        `json:"triangles,omitempty"`
}

// NearestNeighborDataset stores a sample rate and an optional measurement grid.
type NearestNeighborDataset struct {
	SampleRateHz int              `json:"sampleRate"`
	Grid         *MeasurementGrid `json:"grid,omitempty"`
}

// SampleRate returns the dataset sample rate.
func (d NearestNeighborDataset) SampleRate() int {
	return d.SampleRateHz
}

// Lookup returns the nearest measured HRIR pair or a centered identity impulse
// when no measurement grid is available.
func (d NearestNeighborDataset) Lookup(direction geometry.Vec3) (left, right []float64, delaySeconds float64, err error) {
	if d.Grid == nil || len(d.Grid.Directions) == 0 {
		return []float64{1}, []float64{1}, 0, nil
	}

	left, right, delaySeconds = LookupNearest(d.Grid, direction)
	if len(left) == 0 && len(right) == 0 {
		return []float64{1}, []float64{1}, 0, nil
	}

	return left, right, delaySeconds, nil
}

// NearestNeighbor returns the index of the closest measurement direction.
func NearestNeighbor(grid *MeasurementGrid, dir geometry.Vec3) int {
	if grid == nil || len(grid.Directions) == 0 {
		return -1
	}

	target := dir.Normalize()
	if target == geometry.Vec3Zero {
		target = geometry.Vec3{X: 1}
	}

	bestIndex := 0
	bestScore := math.Inf(-1)
	for index, measurement := range grid.Directions {
		score := measurement.Normalize().Dot(target)
		if index == 0 || score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}

	return bestIndex
}

// LookupNearest returns the nearest measured HRIR pair for dir.
func LookupNearest(grid *MeasurementGrid, dir geometry.Vec3) (left, right []float64, delay float64) {
	index := NearestNeighbor(grid, dir)
	if index < 0 {
		return nil, nil, 0
	}

	return measurementAt(grid, index)
}

func measurementAt(grid *MeasurementGrid, index int) (left, right []float64, delay float64) {
	if grid == nil || index < 0 || index >= len(grid.Directions) {
		return nil, nil, 0
	}

	if index < len(grid.LeftHRIRs) {
		left = cloneFloat64s(grid.LeftHRIRs[index])
	}
	if index < len(grid.RightHRIRs) {
		right = cloneFloat64s(grid.RightHRIRs[index])
	}
	if index < len(grid.Delays) {
		delay = grid.Delays[index]
	}

	return left, right, delay
}

func cloneFloat64s(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	out := make([]float64, len(values))
	copy(out, values)
	return out
}
