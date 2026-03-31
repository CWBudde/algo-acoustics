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
	if r.Scene.Room.Kind != scene.RoomKindShoebox || r.Scene.Room.Shoebox == nil {
		return nil, errors.New("raytrace currently supports shoebox rooms only")
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

	tracer, err := NewShoeboxTracer(r.Scene.Room.Shoebox)
	if err != nil {
		return nil, err
	}

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

	launchEnergy := math.Pow(10, source.GainDB/10) / float64(len(rays))
	maxPathLength := r.Config.MaxTimeSeconds * r.Config.SpeedOfSound

	for _, ray := range rays {
		currentRay := ray
		pathLength := 0.0
		bandEnergy := make([]float64, bandCount)
		for bandIndex := range bandEnergy {
			freqHz := float64(bandIndex)
			if bandIndex < len(r.Scene.BandSpec.CenterFreqs) {
				freqHz = r.Scene.BandSpec.CenterFreqs[bandIndex]
			}
			gain := 1.0
			if source.Directivity != nil {
				gain = source.Directivity.GainLinear(freqHz, currentRay.Direction)
			}
			bandEnergy[bandIndex] = launchEnergy * gain * gain
		}

		for bounce := 0; bounce <= r.Config.MaxBounces; bounce++ {
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
					hitEnergy := make([]float64, bandCount)
					capture := receiver.AngularWeight(currentRay.Direction)
					distanceScale := 1 / math.Max(1, pathLength+tHit)
					distanceScale *= distanceScale
					for bandIndex := range hitEnergy {
						hitEnergy[bandIndex] = bandEnergy[bandIndex] * capture * distanceScale
					}
					hist.Add(arrivalTime, hitEnergy)
				}
			}

			pathLength += segmentLength
			if pathLength >= maxPathLength {
				break
			}

			material := r.sceneMaterialForWall(wallIdx)
			for bandIndex := range bandEnergy {
				absorption := material.AbsorptionAt(bandIndex)
				remaining := 1 - absorption
				if remaining < 0 {
					remaining = 0
				}
				if remaining > 1 {
					remaining = 1
				}
				bandEnergy[bandIndex] *= remaining
			}

			scatterCoeff := averageCoeff(material.ScatteringByBand)
			nextDir := SelectReflection(scatterCoeff, currentRay.Direction, hitNormal, rng)
			currentRay = geometry.NewRay(hitPoint.Add(nextDir.Scale(wallEpsilon)), nextDir)

			if totalEnergy(bandEnergy) <= 0 {
				break
			}
		}
	}

	return hist, nil
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

func totalEnergy(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}

	return sum
}
