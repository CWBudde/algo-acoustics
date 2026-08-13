package raytrace

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
)

// TraceSecondary traces delayed, band-energy point-source emissions in the
// receiving room. Exactly Config.NumRays particles are launched across all
// emissions using deterministic stratified sampling proportional to total
// emission energy.
//
//nolint:gocyclo,maintidx,nestif // Mirrors the coupled propagation loop in Trace.
func (r *RayTracer) TraceSecondary(emissions []ir.EnergyEmission) (*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	if len(r.Scene.Receivers) != 1 {
		return nil, fmt.Errorf("raytrace TraceSecondary requires exactly one receiver, got %d", len(r.Scene.Receivers))
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

	bandCount := r.Scene.BandSpec.BandCount()
	if bandCount <= 0 {
		bandCount = 1
	}

	for index, emission := range emissions {
		err := validateEnergyEmission(emission, bandCount)
		if err != nil {
			return nil, fmt.Errorf("secondary emission %d: %w", index, err)
		}
	}

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	histogram := NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	for index := range r.DirectivityGroups {
		r.DirectivityGroups[index].Histogram = NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	}

	receiverData := r.Scene.Receivers[0]
	receiverRadius := effectiveReceiverRadius(r.ReceiverRadius)
	states := secondaryLaunchStates(emissions, r.Config, receiverData.Position, receiverRadius, bandCount)

	if len(states) == 0 {
		return histogram, nil
	}

	tracer, err := r.sceneTracer()
	if err != nil {
		return nil, err
	}

	diffractionIndex := r.diffractionEdgeIndex()
	receiver := SphereReceiver{Center: receiverData.Position, Radius: receiverRadius}
	rng := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic acoustic simulation.
	energyThreshold := r.Config.EnergyTerminationThreshold

	for len(states) > 0 {
		state := states[len(states)-1]
		states = states[:len(states)-1]

		if maxEnergy(state.Energy) <= energyThreshold {
			continue
		}

		currentRay := state.Ray
		pathLength := state.PathLength
		rainCovered := state.RainCovered
		maxPathLength := (r.Config.MaxTimeSeconds - state.DelaySeconds) * r.Config.SpeedOfSound

		if maxPathLength <= 0 {
			continue
		}

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
				if tHit, hit := receiver.Intersects(currentRay, wallEpsilon, segmentLength); hit &&
					DetectionAllowedHybrid(state, r.Config.HybridDetection) {
					arrivalTime := state.DelaySeconds + (pathLength+tHit)/r.Config.SpeedOfSound
					if arrivalTime <= r.Config.MaxTimeSeconds {
						hitEnergy := attenuateEnergyByAir(state.Energy, r.Scene.BandSpec.CenterFreqs, tHit)
						histogram.Add(arrivalTime, hitEnergy)

						if len(r.DirectivityGroups) > 0 {
							arrivalDirection := currentRay.Direction.Scale(-1)
							groupIndex := ClassifyDirection(r.DirectivityGroups, arrivalDirection)
							r.DirectivityGroups[groupIndex].Histogram.Add(arrivalTime, hitEnergy)
						}
					}
				}
			}

			switch r.Config.effectiveDiffractionMode() {
			case DiffractionKellerCone:
				branches := spawnDiffractionBranches(state, currentRay, hitPoint, segmentLength, nil, diffractionIndex, r.Config, rng, maxEnergy(state.Energy), r.Scene.BandSpec.CenterFreqs)
				states = append(states, branches...)
			case DiffractionDAPDFRain:
				deposits := dapdfRainForSegment(state, currentRay, segmentLength, diffractionIndex, tracer, &receiver, nil, r.Config, r.Scene.BandSpec.CenterFreqs, maxEnergy(state.Energy))
				for _, deposit := range deposits {
					energy := oneBandEnergy(bandCount, deposit.BandIndex, deposit.Energy)
					histogram.Add(deposit.ArrivalTime, energy)

					if len(r.DirectivityGroups) > 0 {
						groupIndex := ClassifyDirection(r.DirectivityGroups, deposit.ArrivalDir)
						r.DirectivityGroups[groupIndex].Histogram.Add(deposit.ArrivalTime, energy)
					}
				}

				if len(deposits) > 0 {
					advanceStateAfterDiffraction(&state)
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
				rain := computeDiffuseRain(hitPoint, incidentNormal, remainingEnergy, scattering, receiver, tracer, r.Scene.BandSpec.CenterFreqs, pathLength, r.Config.SpeedOfSound)
				if rain != nil {
					arrivalTime := state.DelaySeconds + rain.ArrivalTime
					if arrivalTime <= r.Config.MaxTimeSeconds {
						histogram.Add(arrivalTime, rain.BandEnergy)

						if len(r.DirectivityGroups) > 0 {
							arrivalDirection := hitPoint.Sub(receiver.Center)
							groupIndex := ClassifyDirection(r.DirectivityGroups, arrivalDirection)
							r.DirectivityGroups[groupIndex].Histogram.Add(arrivalTime, rain.BandEnergy)
						}
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

				advanceStateAfterReflection(&state, scatterCoefficient > 0)
			case ReflectionStrategyRussianRoulette:
				specularBranch, survives := russianRouletteEnergy(specEnergy, energyThreshold, rng)
				if survives {
					specularRay := geometry.NewRay(hitPoint.Add(specularDirection.Scale(wallEpsilon)), specularDirection)
					specularState := reflectedChildState(state, false)
					specularState.RainCovered = false
					specularState.Ray = specularRay
					specularState.Energy = specularBranch
					specularState.PathLength = pathLength
					specularState.DelaySeconds = state.DelaySeconds
					specularState.Bounces = bounce + 1
					states = append(states, specularState)
				}

				diffuseBranch, survives := russianRouletteEnergy(diffuseEnergy, energyThreshold, rng)
				if survives {
					diffuseRay := geometry.NewRay(hitPoint.Add(diffuseDirection.Scale(wallEpsilon)), diffuseDirection)
					diffuseState := reflectedChildState(state, true)
					diffuseState.Ray = diffuseRay
					diffuseState.Energy = diffuseBranch
					diffuseState.PathLength = pathLength
					diffuseState.DelaySeconds = state.DelaySeconds
					diffuseState.Bounces = bounce + 1
					diffuseState.RainCovered = r.Config.DiffuseRain
					states = append(states, diffuseState)
				}

				currentRay = geometry.Ray{}
				state.Energy = nil
			default:
				nextDirection, nextEnergy, diffuse := sampleProbabilisticReflection(specularDirection, diffuseDirection, remainingEnergy, scatterCoefficient, rng)
				state.Energy = nextEnergy
				currentRay = geometry.NewRay(hitPoint.Add(nextDirection.Scale(wallEpsilon)), nextDirection)
				rainCovered = diffuse && r.Config.DiffuseRain
				advanceStateAfterReflection(&state, diffuse)
			}

			if len(state.Energy) > 0 && maxEnergy(state.Energy) <= energyThreshold {
				break
			}
		}
	}

	return histogram, nil
}

func secondaryLaunchStates(
	emissions []ir.EnergyEmission,
	config LaunchConfig,
	receiverPosition geometry.Vec3,
	receiverRadius float64,
	bandCount int,
) []RayState {
	weights := make([]float64, len(emissions))
	var totalWeight float64

	for emissionIndex, emission := range emissions {
		if emission.TimeSeconds >= config.MaxTimeSeconds {
			continue
		}

		for _, energy := range emission.BandEnergy {
			weights[emissionIndex] += energy
		}

		totalWeight += weights[emissionIndex]
	}

	if totalWeight <= 0 || config.NumRays <= 0 {
		return nil
	}

	directions := FibonacciSphere(config.NumRays)
	cumulativeWeights := make([]float64, len(weights))
	var cumulative float64

	for index, weight := range weights {
		cumulative += weight / totalWeight
		cumulativeWeights[index] = cumulative
	}

	cumulativeWeights[len(cumulativeWeights)-1] = 1

	states := make([]RayState, 0, len(directions))

	const goldenRatioConjugate = 0.6180339887498949

	for rayIndex, direction := range directions {
		// A golden-ratio sequence decouples emission selection from the ordered
		// Fibonacci directions while retaining deterministic low discrepancy.
		quantile := math.Mod((float64(rayIndex)+0.5)*goldenRatioConjugate, 1)
		emissionIndex := sort.SearchFloat64s(cumulativeWeights, quantile)

		if emissionIndex >= len(emissions) {
			emissionIndex = len(emissions) - 1
		}

		emission := emissions[emissionIndex]
		probability := weights[emissionIndex] / totalWeight

		if probability <= 0 {
			continue
		}

		baseEnergy := calibratedRayLaunchEnergy(0, emission.Position, receiverPosition, receiverRadius, len(directions))
		energy := make([]float64, bandCount)

		for bandIndex := range energy {
			energy[bandIndex] = baseEnergy * emission.BandEnergy[bandIndex] / probability
		}

		states = append(states, RayState{
			Ray:          geometry.NewRay(emission.Position, direction),
			Energy:       energy,
			DelaySeconds: emission.TimeSeconds,
		})
	}

	return states
}

func validateEnergyEmission(emission ir.EnergyEmission, bandCount int) error {
	if math.IsNaN(emission.TimeSeconds) || math.IsInf(emission.TimeSeconds, 0) || emission.TimeSeconds < 0 {
		return errors.New("time must be finite and non-negative")
	}

	if math.IsNaN(emission.Position.X) || math.IsInf(emission.Position.X, 0) ||
		math.IsNaN(emission.Position.Y) || math.IsInf(emission.Position.Y, 0) ||
		math.IsNaN(emission.Position.Z) || math.IsInf(emission.Position.Z, 0) {
		return errors.New("position must be finite")
	}

	if len(emission.BandEnergy) != bandCount {
		return fmt.Errorf("band energy count = %d, want %d", len(emission.BandEnergy), bandCount)
	}

	for index, energy := range emission.BandEnergy {
		if math.IsNaN(energy) || math.IsInf(energy, 0) || energy < 0 {
			return fmt.Errorf("band energy[%d] = %v, want finite and non-negative", index, energy)
		}
	}

	return nil
}
