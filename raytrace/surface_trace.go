package raytrace

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// TraceSurfaces traces incident source-room energy at planar surface
// receivers. Unlike Trace, the result is total energy captured by each surface
// and therefore uses source power divided by particle count rather than the
// spherical receiver's intensity calibration.
//
//nolint:gocyclo,maintidx // Mirrors the coupled propagation loop in Trace.
func (r *RayTracer) TraceSurfaces(detectors []SurfaceReceiver) ([]*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	if len(r.Scene.Sources) != 1 {
		return nil, fmt.Errorf("raytrace TraceSurfaces requires exactly one source, got %d", len(r.Scene.Sources))
	}

	if r.Config.NumRays <= 0 {
		return nil, errors.New("NumRays must be positive")
	}

	if r.Config.MaxBounces < 0 {
		return nil, errors.New("MaxBounces must not be negative")
	}

	if r.Config.MaxTimeSeconds <= 0 {
		return nil, errors.New("MaxTimeSeconds must be positive")
	}

	if r.Config.SpeedOfSound <= 0 {
		return nil, errors.New("SpeedOfSound must be positive")
	}

	for index, detector := range detectors {
		if len(detector.Polygon) < 3 || detector.Area <= 0 || detector.Normal == geometry.Vec3Zero {
			return nil, fmt.Errorf("surface receiver %d is invalid", index)
		}
	}

	bandCount := r.Scene.BandSpec.BandCount()
	if bandCount <= 0 {
		bandCount = 1
	}

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	histograms := make([]*EnergyHistogram, len(detectors))
	for index := range histograms {
		histograms[index] = NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	}

	tracer, err := r.sceneTracer()
	if err != nil {
		return nil, err
	}

	diffractionIndex := r.diffractionEdgeIndex()
	source := r.Scene.Sources[0]
	rng := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic acoustic simulation.
	rays := LaunchRays(source.Position, r.Config)

	if len(rays) == 0 {
		return histograms, nil
	}

	launchEnergy := math.Pow(10, source.GainDB/10) / float64(len(rays))
	maxPathLength := r.Config.MaxTimeSeconds * r.Config.SpeedOfSound
	energyThreshold := r.Config.EnergyTerminationThreshold

	states := make([]RayState, 0, len(rays))
	for _, ray := range rays {
		states = append(states, RayState{
			Ray:    ray,
			Energy: initialRayEnergy(source, ray.Direction, launchEnergy, bandCount, r.Scene.BandSpec.CenterFreqs),
		})
	}

	for len(states) > 0 {
		state := states[len(states)-1]
		states = states[:len(states)-1]

		if maxEnergy(state.Energy) <= energyThreshold {
			continue
		}

		currentRay := state.Ray
		pathLength := state.PathLength
		rainCovered := state.RainCovered

		for bounce := state.Bounces; bounce <= r.Config.MaxBounces; bounce++ {
			if pathLength >= maxPathLength {
				break
			}

			hitPoint, hitNormal, wallIndex, ok := tracer.NextHit(currentRay)
			if !ok {
				break
			}

			segmentLength := currentRay.Origin.Distance(hitPoint)
			if segmentLength <= 0 {
				break
			}

			if !rainCovered {
				for detectorIndex, detector := range detectors {
					tHit, hit := detector.Intersects(currentRay, wallEpsilon, segmentLength+wallEpsilon)
					if !hit {
						continue
					}

					arrivalTime := (pathLength + tHit) / r.Config.SpeedOfSound
					if arrivalTime <= r.Config.MaxTimeSeconds {
						energy := attenuateEnergyByAir(state.Energy, r.Scene.BandSpec.CenterFreqs, tHit)
						histograms[detectorIndex].Add(arrivalTime, energy)
					}
				}
			}

			switch r.Config.effectiveDiffractionMode() {
			case DiffractionKellerCone:
				branches := spawnDiffractionBranches(state, currentRay, hitPoint, segmentLength, nil, diffractionIndex, r.Config, rng, launchEnergy, r.Scene.BandSpec.CenterFreqs)
				states = append(states, branches...)
			case DiffractionDAPDFRain:
				deposits := dapdfRainForSegment(state, currentRay, segmentLength, diffractionIndex, tracer, nil, detectors, r.Config, r.Scene.BandSpec.CenterFreqs, launchEnergy)
				for _, deposit := range deposits {
					if deposit.SurfaceIndex >= 0 && deposit.SurfaceIndex < len(histograms) {
						histograms[deposit.SurfaceIndex].Add(deposit.ArrivalTime, oneBandEnergy(bandCount, deposit.BandIndex, deposit.Energy))
					}
				}
			}

			pathLength += segmentLength
			if pathLength >= maxPathLength {
				break
			}

			state.Energy = attenuateEnergyByAir(state.Energy, r.Scene.BandSpec.CenterFreqs, segmentLength)
			material := r.sceneMaterialForWall(wallIndex)
			incidentNormal := faceIncidentSide(currentRay.Direction, hitNormal)

			absorption := make([]float64, bandCount)
			for bandIndex := range absorption {
				absorption[bandIndex] = material.AbsorptionAt(bandIndex)
			}

			scattering := material.ScatteringCoefficients(bandCount)
			specEnergy, diffuseEnergy, remainingEnergy := splitReflectionEnergy(state.Energy, absorption, scattering)

			if r.Config.DiffuseRain {
				for detectorIndex, detector := range detectors {
					rain := computeSurfaceRain(hitPoint, incidentNormal, remainingEnergy, scattering, detector, tracer, r.Scene.BandSpec.CenterFreqs, pathLength, r.Config.SpeedOfSound)
					if rain != nil && rain.ArrivalTime <= r.Config.MaxTimeSeconds {
						histograms[detectorIndex].Add(rain.ArrivalTime, rain.BandEnergy)
					}
				}
			}

			scatterCoefficient := averageCoeff(scattering)
			specularDirection := SpecularReflect(currentRay.Direction, incidentNormal)
			diffuseDirection := LambertDirection(incidentNormal, rng)

			switch r.Config.ReflectionStrategy {
			case ReflectionStrategyDeterministicBlend:
				nextDirection := chooseBlendDirection(specularDirection, diffuseDirection, scatterCoefficient)
				currentRay = geometry.NewRay(hitPoint.Add(nextDirection.Scale(wallEpsilon)), nextDirection)
				state.Energy = remainingEnergy
				rainCovered = r.Config.DiffuseRain
			case ReflectionStrategyRussianRoulette:
				specularBranch, survives := russianRouletteEnergy(specEnergy, energyThreshold, rng)
				if survives {
					specularRay := geometry.NewRay(hitPoint.Add(specularDirection.Scale(wallEpsilon)), specularDirection)
					states = append(states, RayState{Ray: specularRay, Energy: specularBranch, PathLength: pathLength, Bounces: bounce + 1})
				}

				diffuseBranch, survives := russianRouletteEnergy(diffuseEnergy, energyThreshold, rng)
				if survives {
					diffuseRay := geometry.NewRay(hitPoint.Add(diffuseDirection.Scale(wallEpsilon)), diffuseDirection)
					states = append(states, RayState{Ray: diffuseRay, Energy: diffuseBranch, PathLength: pathLength, Bounces: bounce + 1, RainCovered: r.Config.DiffuseRain})
				}

				currentRay = geometry.Ray{}
				state.Energy = nil
			default:
				nextDirection, nextEnergy, diffuse := sampleProbabilisticReflection(specularDirection, diffuseDirection, remainingEnergy, scatterCoefficient, rng)
				state.Energy = nextEnergy
				currentRay = geometry.NewRay(hitPoint.Add(nextDirection.Scale(wallEpsilon)), nextDirection)
				rainCovered = diffuse && r.Config.DiffuseRain
			}

			if len(state.Energy) > 0 && maxEnergy(state.Energy) <= energyThreshold {
				break
			}
		}
	}

	return histograms, nil
}
