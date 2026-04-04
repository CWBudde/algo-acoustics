package raytrace

import (
	"errors"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

const defaultBinDurationSeconds = 0.01

// RayTracer launches rays and accumulates receiver energy into time bins.
type RayTracer struct {
	Config             LaunchConfig
	Scene              *scene.Scene
	ReceiverRadius     float64
	BinDurationSeconds float64
}

// Trace runs the Monte Carlo ray tracer and returns a banded energy histogram.
func (r *RayTracer) Trace() (*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	if len(r.Scene.Sources) == 0 {
		return nil, errors.New("scene has no sources")
	}

	if len(r.Scene.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
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

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	hist := NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)

	tracer, err := r.sceneTracer()
	if err != nil {
		return nil, err
	}

	diffractionIndex := r.diffractionEdgeIndex()

	source := r.Scene.Sources[0]
	receiverData := r.Scene.Receivers[0]

	receiverRadius := r.ReceiverRadius
	if receiverRadius <= 0 {
		receiverRadius = 0.25
	}

	receiver := SphereReceiver{Center: receiverData.Position, Radius: receiverRadius}

	rng := rand.New(rand.NewSource(1))

	rays := LaunchRays(source.Position, r.Config)
	if len(rays) == 0 {
		return hist, nil
	}

	launchEnergy := calibratedRayLaunchEnergy(source.GainDB, source.Position, receiverData.Position, receiverRadius, len(rays))
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

		for bounce := state.Bounces; bounce <= r.Config.MaxBounces; bounce++ {
			if pathLength >= maxPathLength {
				break
			}

			hitPoint, hitNormal, wallIdx, ok := tracer.NextHit(currentRay)
			if !ok {
				break
			}

			segmentLength := currentRay.Origin.Distance(hitPoint)
			if segmentLength <= 0 {
				break
			}

			if tHit, hit := receiver.Intersects(currentRay, wallEpsilon, segmentLength); hit {
				arrivalTime := (pathLength + tHit) / r.Config.SpeedOfSound
				if arrivalTime <= r.Config.MaxTimeSeconds {
					hitEnergy := attenuateEnergyByAir(state.Energy, r.Scene.BandSpec.CenterFreqs, tHit, defaultAirTemperatureC, defaultRelativeHumidity)

					capture := receiver.AngularWeight(currentRay.Direction)
					for bandIndex := range hitEnergy {
						hitEnergy[bandIndex] *= capture
					}

					hist.Add(arrivalTime, hitEnergy)
				}
			}

			if diffractionIndex != nil && r.Config.DiffractionAngularThreshold > 0 {
				branches := spawnDiffractionBranches(state, currentRay, hitPoint, segmentLength, diffractionIndex.edges, diffractionIndex, r.Config, rng, launchEnergy, r.Scene.BandSpec.CenterFreqs)
				states = append(states, branches...)
			}

			pathLength += segmentLength
			if pathLength >= maxPathLength {
				break
			}

			state.Energy = attenuateEnergyByAir(state.Energy, r.Scene.BandSpec.CenterFreqs, segmentLength, defaultAirTemperatureC, defaultRelativeHumidity)

			material := r.sceneMaterialForWall(wallIdx)

			absorption := make([]float64, bandCount)
			for bandIndex := range absorption {
				absorption[bandIndex] = material.AbsorptionAt(bandIndex)
			}

			scattering := material.ScatteringCoefficients(bandCount)
			specEnergy, diffuseEnergy, remainingEnergy := splitReflectionEnergy(state.Energy, absorption, scattering)
			scatterCoeff := averageCoeff(scattering)
			specularDir := SpecularReflect(currentRay.Direction, hitNormal)
			diffuseDir := LambertDirection(hitNormal, rng)

			switch r.Config.ReflectionStrategy {
			case ReflectionStrategyDeterministicBlend:
				nextDir := chooseBlendDirection(specularDir, diffuseDir, scatterCoeff)
				currentRay = geometry.NewRay(hitPoint.Add(nextDir.Scale(wallEpsilon)), nextDir)
				state.Energy = remainingEnergy
			case ReflectionStrategyRussianRoulette:
				specBranch, ok := russianRouletteEnergy(specEnergy, energyThreshold, rng)
				if ok {
					specRay := geometry.NewRay(hitPoint.Add(specularDir.Scale(wallEpsilon)), specularDir)
					states = append(states, RayState{Ray: specRay, Energy: specBranch, PathLength: pathLength, Bounces: bounce + 1})
				}

				diffBranch, ok := russianRouletteEnergy(diffuseEnergy, energyThreshold, rng)
				if ok {
					diffRay := geometry.NewRay(hitPoint.Add(diffuseDir.Scale(wallEpsilon)), diffuseDir)
					states = append(states, RayState{Ray: diffRay, Energy: diffBranch, PathLength: pathLength, Bounces: bounce + 1})
				}

				currentRay = geometry.Ray{}
				state.Energy = nil

				break
			default:
				if scatterCoeff >= 1 || (scatterCoeff > 0 && rng.Float64() < scatterCoeff) {
					state.Energy = diffuseEnergy
					currentRay = geometry.NewRay(hitPoint.Add(diffuseDir.Scale(wallEpsilon)), diffuseDir)
				} else {
					state.Energy = specEnergy
					currentRay = geometry.NewRay(hitPoint.Add(specularDir.Scale(wallEpsilon)), specularDir)
				}
			}

			if len(state.Energy) > 0 && maxEnergy(state.Energy) <= energyThreshold {
				break
			}
		}
	}

	return hist, nil
}

func initialRayEnergy(source scene.Source, dir geometry.Vec3, launchEnergy float64, bandCount int, centerFreqs []float64) []float64 {
	energy := make([]float64, bandCount)
	for bandIndex := range energy {
		freqHz := float64(bandIndex)
		if bandIndex < len(centerFreqs) {
			freqHz = centerFreqs[bandIndex]
		}

		gain := 1.0
		if source.Directivity != nil {
			gain = source.Directivity.GainLinear(freqHz, dir)
		}

		energy[bandIndex] = launchEnergy * gain * gain
	}

	return energy
}

func (r *RayTracer) sceneMaterialForWall(wallIdx int) scene.Material {
	if wallIdx < 0 || r.Scene == nil || r.Scene.Room.Shoebox == nil {
		return scene.MaterialFullyReflective()
	}

	room := r.Scene.Room.Shoebox
	if wallIdx >= len(room.WallMaterials) {
		return scene.MaterialFullyReflective()
	}

	name := room.WallMaterials[wallIdx]
	if material, ok := r.Scene.Materials[name]; ok {
		return material
	}

	return scene.MaterialFullyReflective()
}

func (r *RayTracer) sceneTracer() (Tracer, error) {
	if r == nil || r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	switch r.Scene.Room.Kind {
	case scene.RoomKindShoebox:
		if r.Scene.Room.Shoebox == nil {
			return nil, errors.New("shoebox room is nil")
		}

		return NewShoeboxTracer(r.Scene.Room.Shoebox)
	case scene.RoomKindMesh:
		if r.Scene.Room.Mesh == nil {
			return nil, errors.New("mesh room is nil")
		}

		return NewMeshTracer(r.Scene.Room.Mesh, nil)
	default:
		return nil, errors.New("raytrace requires a shoebox or mesh room")
	}
}

func (r *RayTracer) diffractionEdgeIndex() *DiffractionEdgeIndex {
	if r == nil || r.Scene == nil || r.Scene.Room.Mesh == nil || r.Config.DiffractionAngularThreshold <= 0 {
		return nil
	}

	return NewDiffractionEdgeIndex(r.Scene.Room.Mesh)
}

func calibratedRayLaunchEnergy(sourceGainDB float64, sourcePosition, receiverPosition geometry.Vec3, receiverRadius float64, rayCount int) float64 {
	if rayCount <= 0 || receiverRadius <= 0 {
		return 0
	}

	sourceIntensity := math.Pow(10, sourceGainDB/10)

	distance := sourcePosition.Distance(receiverPosition)
	if distance <= receiverRadius {
		return sourceIntensity / float64(rayCount)
	}

	ratio := receiverRadius / distance
	if ratio < 0 {
		ratio = 0
	}

	if ratio > 1 {
		ratio = 1
	}

	cosGamma := math.Sqrt(1 - ratio*ratio)

	denominator := 2 * math.Pi * distance * distance * float64(rayCount) * (1 - cosGamma)
	if denominator <= 0 {
		return sourceIntensity / float64(rayCount)
	}

	return sourceIntensity / denominator
}
