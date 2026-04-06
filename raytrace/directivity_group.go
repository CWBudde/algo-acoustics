package raytrace

import (
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// DirectivityGroup represents an angular sector of the spherical detector.
// Each sector accumulates a separate energy histogram, enabling directional
// late-field energy analysis for binaural rendering.
type DirectivityGroup struct {
	AzimuthCenter   float64       // sector center azimuth in radians [0, 2*pi)
	ElevationCenter float64       // sector center elevation in radians [-pi/2, pi/2]
	AzimuthExtent   float64       // full angular width in azimuth (radians)
	ElevationExtent float64       // full angular width in elevation (radians)
	Direction       geometry.Vec3 // unit vector at sector centroid for HRIR lookup
	Histogram       *EnergyHistogram
}

// NewDirectivityGroups subdivides the sphere into azSteps * elSteps angular
// sectors. Azimuth spans [0, 2*pi), elevation spans [-pi/2, pi/2].
// Groups are ordered elevation-major: all azimuth steps for the lowest
// elevation band first, then the next band, and so on.
func NewDirectivityGroups(azSteps, elSteps int) []DirectivityGroup {
	if azSteps <= 0 || elSteps <= 0 {
		return nil
	}

	azExtent := 2 * math.Pi / float64(azSteps)
	elExtent := math.Pi / float64(elSteps)

	dgs := make([]DirectivityGroup, 0, azSteps*elSteps)

	for el := range elSteps {
		elCenter := -math.Pi/2 + (float64(el)+0.5)*elExtent

		for az := range azSteps {
			azCenter := (float64(az) + 0.5) * azExtent

			dgs = append(dgs, DirectivityGroup{
				AzimuthCenter:   azCenter,
				ElevationCenter: elCenter,
				AzimuthExtent:   azExtent,
				ElevationExtent: elExtent,
				Direction: geometry.Vec3{
					X: math.Cos(elCenter) * math.Cos(azCenter),
					Y: math.Cos(elCenter) * math.Sin(azCenter),
					Z: math.Sin(elCenter),
				},
			})
		}
	}

	return dgs
}

// ClassifyDirection returns the index of the directivity group that contains
// the given direction vector. Returns 0 for zero-length vectors.
func ClassifyDirection(dgs []DirectivityGroup, dir geometry.Vec3) int {
	if len(dgs) == 0 {
		return 0
	}

	norm := dir.Norm()
	if norm == 0 {
		return 0
	}

	// Compute spherical coordinates from the direction vector.
	el := math.Asin(dir.Z / norm)
	az := math.Atan2(dir.Y, dir.X)

	if az < 0 {
		az += 2 * math.Pi
	}

	bestIdx := 0
	bestDist := math.MaxFloat64

	for i, dg := range dgs {
		dAz := angleDiffWrapped(az, dg.AzimuthCenter)
		dEl := el - dg.ElevationCenter

		dist := dAz*dAz + dEl*dEl
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	return bestIdx
}

// DGHitProbabilities computes the per-slot hit probability for each directivity
// group. Returns probs[d][k] = P(d, k) = E_d(k) / sum_d(E_d(k)).
// For time slots where no DG received energy, probabilities are distributed
// uniformly (fully diffuse assumption). Returns nil if DGs are empty or have
// no histograms.
func DGHitProbabilities(dgs []DirectivityGroup) [][]float64 {
	if len(dgs) == 0 {
		return nil
	}

	slotCount := 0

	for _, dg := range dgs {
		if dg.Histogram == nil {
			return nil
		}

		if len(dg.Histogram.Bins) > slotCount {
			slotCount = len(dg.Histogram.Bins)
		}
	}

	if slotCount == 0 {
		return nil
	}

	uniform := 1.0 / float64(len(dgs))
	probs := make([][]float64, len(dgs))

	for d := range probs {
		probs[d] = make([]float64, slotCount)
	}

	for k := range slotCount {
		var totalEnergy float64

		for d := range dgs {
			if k < len(dgs[d].Histogram.Bins) {
				for _, e := range dgs[d].Histogram.Bins[k].BandEnergy {
					totalEnergy += e
				}
			}
		}

		if totalEnergy <= 0 {
			for d := range dgs {
				probs[d][k] = uniform
			}

			continue
		}

		for d := range dgs {
			var dgEnergy float64

			if k < len(dgs[d].Histogram.Bins) {
				for _, e := range dgs[d].Histogram.Bins[k].BandEnergy {
					dgEnergy += e
				}
			}

			probs[d][k] = dgEnergy / totalEnergy
		}
	}

	return probs
}

// angleDiffWrapped returns the shortest signed angular difference on [0, 2*pi).
func angleDiffWrapped(a, b float64) float64 {
	d := a - b
	for d > math.Pi {
		d -= 2 * math.Pi
	}

	for d < -math.Pi {
		d += 2 * math.Pi
	}

	return d
}
