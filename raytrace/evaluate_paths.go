package raytrace

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// EvaluatePaths replays cached geometric paths with the current scene materials,
// producing an EnergyHistogram without re-tracing geometry. The cache must have
// been produced by TracePaths on a scene with matching geometry and effective
// receiver radius. Geometry or radius changes require tracing a new cache.
func (r *RayTracer) EvaluatePaths(cache *PathCache) (*EnergyHistogram, error) {
	if r == nil {
		return nil, errors.New("raytracer is nil")
	}

	if r.Scene == nil {
		return nil, errors.New("scene is nil")
	}

	if cache == nil {
		return nil, errors.New("path cache is nil")
	}

	if len(r.Scene.Sources) == 0 {
		return nil, errors.New("scene has no sources")
	}

	if len(r.Scene.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
	}

	if r.Config.SpeedOfSound <= 0 {
		return nil, errors.New("SpeedOfSound must be positive")
	}

	if r.Config.MaxTimeSeconds <= 0 {
		return nil, errors.New("MaxTimeSeconds must be positive")
	}

	receiverRadius := effectiveReceiverRadius(r.ReceiverRadius)
	if !cache.ValidFor(r.Scene, receiverRadius) {
		return nil, errors.New("path cache is stale for current scene geometry or receiver radius")
	}

	bandCount := r.Scene.BandSpec.BandCount()
	if bandCount <= 0 {
		bandCount = 1
	}

	centerFreqs := r.Scene.BandSpec.CenterFreqs

	binDuration := r.BinDurationSeconds
	if binDuration <= 0 {
		binDuration = defaultBinDurationSeconds
	}

	hist := NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)

	for i := range r.DirectivityGroups {
		r.DirectivityGroups[i].Histogram = NewEnergyHistogram(r.Config.MaxTimeSeconds, binDuration, bandCount)
	}

	source := r.Scene.Sources[0]
	receiverData := r.Scene.Receivers[0]

	receiver := SphereReceiver{Center: receiverData.Position, Radius: receiverRadius}

	launchEnergy := calibratedRayLaunchEnergy(source.GainDB, source.Position, receiverData.Position, receiverRadius, len(cache.Paths))
	energyThreshold := r.Config.EnergyTerminationThreshold

	for _, tp := range cache.Paths {
		energy := initialRayEnergy(source, tp.LaunchDir, launchEnergy, bandCount, centerFreqs)
		origin := source.Position
		var pathLength float64

		for _, step := range tp.Steps {
			rayDir := step.HitPoint.Sub(origin)
			norm := rayDir.Norm()

			if norm <= 0 {
				break
			}

			rayDir = rayDir.Scale(1 / norm)
			ray := geometry.NewRay(origin, rayDir)

			if tHit, hit := receiver.Intersects(ray, wallEpsilon, step.SegmentLength); hit {
				arrivalTime := (pathLength + tHit) / r.Config.SpeedOfSound
				if arrivalTime <= r.Config.MaxTimeSeconds {
					hitEnergy := attenuateEnergyByAir(energy, centerFreqs, tHit)

					capture := receiver.AngularWeight(rayDir)
					for bi := range hitEnergy {
						hitEnergy[bi] *= capture
					}

					hist.Add(arrivalTime, hitEnergy)

					if len(r.DirectivityGroups) > 0 {
						arrivalDir := rayDir.Scale(-1)
						dgIdx := ClassifyDirection(r.DirectivityGroups, arrivalDir)
						r.DirectivityGroups[dgIdx].Histogram.Add(arrivalTime, hitEnergy)
					}
				}
			}

			pathLength += step.SegmentLength

			energy = attenuateEnergyByAir(energy, centerFreqs, step.SegmentLength)

			material := r.sceneMaterialForWall(step.WallIndex)

			absorption := make([]float64, bandCount)
			for bi := range absorption {
				absorption[bi] = material.AbsorptionAt(bi)
			}

			scattering := material.ScatteringCoefficients(bandCount)
			_, _, remainingEnergy := splitReflectionEnergy(energy, absorption, scattering)

			energy = remainingEnergy

			if maxEnergy(energy) <= energyThreshold {
				break
			}

			origin = step.HitPoint
		}
	}

	return hist, nil
}
