package metrics

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/scene"
)

// sabineConstant is the Sabine formula constant 0.161 s/m, equivalent to
// 4 * ln(10^6) / (6 * c) where c = 343 m/s.
const sabineConstant = 0.161

// logDecayFactor is 6 * ln(10) ≈ 13.8159.
// Energy in a reverberant field decays as E(t) ∝ exp(-logDecayFactor * t / T60).
const logDecayFactor = 6 * math.Ln10

// RoomStats holds pre-computed room geometry for fast statistical estimation.
// Compute once via ShoeboxStatsFromScene and reuse across all estimator calls.
type RoomStats struct {
	Volume      float64   // room volume in m³
	SurfaceArea float64   // total room surface area in m²
	AlphaByBand []float64 // area-weighted average absorption coefficient per octave band
}

// ShoeboxStatsFromScene extracts volume, surface area, and per-band absorption
// statistics from a shoebox scene. Returns an error for non-shoebox scenes.
func ShoeboxStatsFromScene(sc *scene.Scene) (RoomStats, error) {
	if sc == nil {
		return RoomStats{}, errors.New("scene must not be nil")
	}

	if sc.Room.Kind != scene.RoomKindShoebox || sc.Room.Shoebox == nil {
		return RoomStats{}, errors.New("scene must have a shoebox room")
	}

	sb := sc.Room.Shoebox
	w, d, h := sb.Width, sb.Depth, sb.Height

	if w <= 0 || d <= 0 || h <= 0 {
		return RoomStats{}, errors.New("shoebox dimensions must be positive")
	}

	bandCount := len(sc.BandSpec.CenterFreqs)
	if bandCount == 0 {
		return RoomStats{}, errors.New("scene must have at least one frequency band")
	}

	// Wall ordering: [xMin, xMax, yMin, yMax, zMin, zMax]
	faceAreas := [6]float64{
		d * h, // xMin
		d * h, // xMax
		w * h, // yMin
		w * h, // yMax
		w * d, // zMin
		w * d, // zMax
	}

	totalArea := 0.0
	for _, a := range faceAreas {
		totalArea += a
	}

	alphaByBand := make([]float64, bandCount)
	for band := range bandCount {
		totalAbsArea := 0.0

		for faceIdx, faceArea := range faceAreas {
			matName := sb.WallMaterials[faceIdx]
			mat, ok := sc.Materials[matName]

			if !ok {
				continue // treat missing material as fully reflective
			}

			totalAbsArea += faceArea * mat.AbsorptionAt(band)
		}

		alphaByBand[band] = totalAbsArea / totalArea
	}

	return RoomStats{
		Volume:      w * d * h,
		SurfaceArea: totalArea,
		AlphaByBand: alphaByBand,
	}, nil
}

// SabineRT60 estimates reverberation time using Sabine's formula:
//
//	T60 = 0.161 * V / (S * α)
//
// Sabine's formula assumes a perfectly diffuse field and is most accurate for
// low absorption (α < 0.2). Use EyringRT60 for higher absorption.
func SabineRT60(stats RoomStats, bandIndex int) (float64, error) {
	alpha, err := bandAlpha(stats, bandIndex)
	if err != nil {
		return 0, err
	}

	if alpha == 0 {
		return 0, errors.New("average absorption is zero: Sabine RT60 is undefined")
	}

	return sabineConstant * stats.Volume / (stats.SurfaceArea * alpha), nil
}

// EyringRT60 estimates reverberation time using Eyring's formula:
//
//	T60 = 0.161 * V / (-S * ln(1 - α))
//
// More accurate than Sabine for higher absorption (α ≥ 0.2).
func EyringRT60(stats RoomStats, bandIndex int) (float64, error) {
	alpha, err := bandAlpha(stats, bandIndex)
	if err != nil {
		return 0, err
	}

	if alpha <= 0 {
		return 0, errors.New("average absorption must be positive for Eyring RT60")
	}

	if alpha >= 1 {
		return 0, errors.New("average absorption must be less than 1 for Eyring RT60")
	}

	return sabineConstant * stats.Volume / (-stats.SurfaceArea * math.Log(1-alpha)), nil
}

// CriticalDistance returns the distance at which the direct-field sound level
// equals the reverberant-field level:
//
//	Dc = sqrt(Q * R / (16π))   where R = S*α / (1-α)
//
// q is the source directivity factor (Q = 1 for omnidirectional, Q = 2 for
// a hemisphere-mounted source, etc.).
func CriticalDistance(stats RoomStats, bandIndex int, q float64) (float64, error) {
	if q <= 0 {
		return 0, fmt.Errorf("directivity factor must be positive, got %g", q)
	}

	alpha, err := bandAlpha(stats, bandIndex)
	if err != nil {
		return 0, err
	}

	if alpha <= 0 {
		return 0, errors.New("average absorption must be positive for critical distance")
	}

	if alpha >= 1 {
		return 0, errors.New("average absorption must be less than 1 for critical distance")
	}

	roomConstant := stats.SurfaceArea * alpha / (1 - alpha)

	return math.Sqrt(q * roomConstant / (16 * math.Pi)), nil
}

// EstimateC80 estimates the clarity index C80 from room statistics.
// The estimate uses Eyring RT60 and models the reverberant field as a pure
// exponential decay:
//
//	C80 = 10 * log10(exp(0.08 * γ) - 1)   where γ = logDecayFactor / T60
func EstimateC80(stats RoomStats, bandIndex int) (float64, error) {
	t60, err := EyringRT60(stats, bandIndex)
	if err != nil {
		return 0, fmt.Errorf("EstimateC80: %w", err)
	}

	return c80FromT60(t60), nil
}

// EstimateD50 estimates the definition metric D50 from room statistics.
// The estimate uses Eyring RT60 and models the reverberant field as a pure
// exponential decay:
//
//	D50 = 1 - exp(-0.05 * γ)   where γ = logDecayFactor / T60
func EstimateD50(stats RoomStats, bandIndex int) (float64, error) {
	t60, err := EyringRT60(stats, bandIndex)
	if err != nil {
		return 0, fmt.Errorf("EstimateD50: %w", err)
	}

	return d50FromT60(t60), nil
}

// c80FromT60 computes the C80 estimate for a given T60.
func c80FromT60(t60 float64) float64 {
	exponent := 0.08 * logDecayFactor / t60
	ratio := math.Exp(exponent) - 1

	if ratio <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(ratio)
}

// d50FromT60 computes the D50 estimate for a given T60.
func d50FromT60(t60 float64) float64 {
	return 1 - math.Exp(-0.05*logDecayFactor/t60)
}

// bandAlpha validates stats and returns the absorption coefficient for the
// requested band index.
func bandAlpha(stats RoomStats, bandIndex int) (float64, error) {
	if stats.Volume <= 0 {
		return 0, errors.New("room volume must be positive")
	}

	if stats.SurfaceArea <= 0 {
		return 0, errors.New("room surface area must be positive")
	}

	if bandIndex < 0 || bandIndex >= len(stats.AlphaByBand) {
		return 0, fmt.Errorf("band index %d out of range [0, %d)", bandIndex, len(stats.AlphaByBand))
	}

	return stats.AlphaByBand[bandIndex], nil
}
