package raytrace

import (
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/geometry"
)

const (
	diffractionSpawnEpsilon = 1e-6
	diffractionCullingDB    = -60.0
)

func spawnDiffractionBranches(state RayState, ray geometry.Ray, hitPoint geometry.Vec3, segmentLength float64, _ []geometry.DiffractionEdge, index *DiffractionEdgeIndex, cfg LaunchConfig, rng *rand.Rand, launchEnergy float64, bandFreqs []float64) []RayState {
	if index == nil || cfg.effectiveDiffractionMode() != DiffractionKellerCone {
		return nil
	}

	if cfg.DiffractionAngularThreshold <= 0 {
		cfg.DiffractionAngularThreshold = 0.3
	}

	if cfg.DiffractionConeSamples <= 0 {
		cfg.DiffractionConeSamples = 8
	}

	padding := math.Tan(cfg.DiffractionAngularThreshold) * segmentLength
	if padding < diffractionSpawnEpsilon {
		padding = diffractionSpawnEpsilon
	}

	candidates := index.Candidates(ray.Origin, hitPoint, padding)
	if len(candidates) == 0 {
		return nil
	}

	branches := make([]RayState, 0, len(candidates)*cfg.DiffractionConeSamples)
	for _, edge := range candidates {
		closestPoint, rayT, edgeT, dist, ok := closestApproach(ray.Origin, hitPoint, edge.Start, edge.End)
		if !ok || rayT <= 0 || rayT >= segmentLength {
			continue
		}

		angle := math.Atan2(dist, rayT)
		if angle > cfg.DiffractionAngularThreshold {
			continue
		}

		edgeEnergy := attenuateEnergyByAir(state.Energy, bandFreqs, rayT)
		if maxEnergy(edgeEnergy) <= 0 {
			continue
		}

		proximity := 1 - angle/cfg.DiffractionAngularThreshold
		if proximity < 0 {
			proximity = 0
		}

		branchScale := 0.35 * proximity / float64(cfg.DiffractionConeSamples)
		if branchScale <= 0 {
			continue
		}

		for sampleIndex := range cfg.DiffractionConeSamples {
			dir := sampleKellerConeDirection(ray.Direction, edge.Direction, sampleIndex, cfg.DiffractionConeSamples, rng)
			if dir == geometry.Vec3Zero {
				continue
			}

			branchEnergy := cloneEnergy(edgeEnergy)
			for bandIndex := range branchEnergy {
				branchEnergy[bandIndex] *= branchScale
			}

			if maxEnergy(branchEnergy) <= launchEnergy*math.Pow(10, diffractionCullingDB/10) {
				continue
			}

			offset := dir.Scale(diffractionSpawnEpsilon)
			origin := closestPoint.Add(offset)
			branches = append(branches, RayState{
				Ray:          geometry.NewRay(origin, dir),
				Energy:       branchEnergy,
				PathLength:   state.PathLength + rayT + offset.Norm(),
				DelaySeconds: state.DelaySeconds,
				Bounces:      state.Bounces + 1,
			})
		}

		_ = edgeT
	}

	return branches
}

func sampleKellerConeDirection(incidentDir, edgeDir geometry.Vec3, sampleIndex, sampleCount int, rng *rand.Rand) geometry.Vec3 {
	axis := edgeDir.Normalize()
	if axis == geometry.Vec3Zero {
		return geometry.Vec3Zero
	}

	incoming := incidentDir.Normalize()
	if incoming == geometry.Vec3Zero {
		return geometry.Vec3Zero
	}

	cosTheta := clampDot(incoming.Dot(axis))
	coneAngle := math.Acos(cosTheta)

	radial := incoming.Sub(axis.Scale(cosTheta))
	if radial.Norm() <= diffractionSpawnEpsilon {
		radial = orthogonalBasis(axis)
	}

	u := radial.Normalize()

	v := axis.Cross(u).Normalize()
	if v == geometry.Vec3Zero {
		v = orthogonalBasis(axis).Cross(axis).Normalize()
	}

	azimuth := 2 * math.Pi * (float64(sampleIndex) + 0.5)
	if sampleCount > 0 {
		azimuth /= float64(sampleCount)
	}

	if rng != nil {
		azimuth += (rng.Float64() - 0.5) * (2 * math.Pi / float64(sampleCount))
	}

	sinTheta := math.Sin(coneAngle)
	dir := axis.Scale(cosTheta).
		Add(u.Scale(sinTheta * math.Cos(azimuth))).
		Add(v.Scale(sinTheta * math.Sin(azimuth)))

	if dir == geometry.Vec3Zero {
		return geometry.Vec3Zero
	}

	return dir.Normalize()
}

func closestApproach(rayStart, rayEnd, edgeStart, edgeEnd geometry.Vec3) (closest geometry.Vec3, rayT, edgeT, distance float64, ok bool) {
	u, v, w := rayEnd.Sub(rayStart), edgeEnd.Sub(edgeStart), rayStart.Sub(edgeStart)
	a := u.Dot(u)
	b := u.Dot(v)
	c := v.Dot(v)
	d := u.Dot(w)
	e := v.Dot(w)
	denom := a*c - b*b

	var sN, sD float64
	var tN, tD float64

	if denom < diffractionSpawnEpsilon {
		sN = 0
		sD = 1
		tN = e
		tD = c
	} else {
		sD = denom
		tD = denom
		sN = b*e - c*d
		tN = a*e - b*d

		if sN < 0 {
			sN = 0
			tN = e
			tD = c
		} else if sN > sD {
			sN = sD
			tN = e + b
			tD = c
		}
	}

	if tN < 0 {
		tN = 0
		sN, sD = clampClosestApproachS(-d, a, sD)
	} else if tN > tD {
		tN = tD
		sN, sD = clampClosestApproachS(-d+b, a, sD)
	}

	sc := 0.0
	if math.Abs(sD) > diffractionSpawnEpsilon {
		sc = sN / sD
	}

	tc := 0.0
	if math.Abs(tD) > diffractionSpawnEpsilon {
		tc = tN / tD
	}

	if sc < 0 || sc > 1 || tc < 0 || tc > 1 {
		return geometry.Vec3{}, 0, 0, 0, false
	}

	pointOnRay := rayStart.Add(u.Scale(sc))
	pointOnEdge := edgeStart.Add(v.Scale(tc))
	closest = pointOnEdge
	rayT = pointOnRay.Sub(rayStart).Norm()
	edgeT = tc * v.Norm()
	distance = pointOnRay.Distance(pointOnEdge)

	return closest, rayT, edgeT, distance, true
}

func clampClosestApproachS(value, a, sD float64) (float64, float64) {
	switch {
	case value < 0:
		return 0, sD
	case value > a:
		return sD, sD
	default:
		return value, a
	}
}

func orthogonalBasis(axis geometry.Vec3) geometry.Vec3 {
	basis := geometry.Vec3{X: 1}
	if math.Abs(axis.X) > 0.9 {
		basis = geometry.Vec3{Y: 1}
	}

	ortho := axis.Cross(basis)
	if ortho.Norm() <= diffractionSpawnEpsilon {
		ortho = axis.Cross(geometry.Vec3{Z: 1})
	}

	return ortho.Normalize()
}

func clampDot(v float64) float64 {
	switch {
	case v < -1:
		return -1
	case v > 1:
		return 1
	default:
		return v
	}
}
