package raytrace

import (
	"errors"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// TracePaths performs geometry-only ray tracing: it launches rays from the
// first source and records every bounce as a PathStep. No energy computation,
// material lookup, or receiver intersection check is performed. The result
// can be cached and later replayed with different materials via EvaluatePaths.
func (r *RayTracer) TracePaths() (*PathCache, error) {
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

	tracer, err := r.sceneTracer()
	if err != nil {
		return nil, err
	}

	source := r.Scene.Sources[0]
	maxPathLength := r.Config.MaxTimeSeconds * r.Config.SpeedOfSound

	rays := LaunchRays(source.Position, r.Config)
	paths := make([]TracedPath, len(rays))

	hasDGs := len(r.DirectivityGroups) > 0

	for i, ray := range rays {
		tp := TracedPath{
			LaunchDir: ray.Direction,
			DGIndex:   -1,
			Steps:     make([]PathStep, 0, r.Config.MaxBounces),
		}

		if hasDGs {
			tp.DGIndex = ClassifyDirection(r.DirectivityGroups, ray.Direction)
		}

		currentRay := ray
		var pathLength float64

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

			tp.Steps = append(tp.Steps, PathStep{
				HitPoint:      hitPoint,
				Normal:        hitNormal,
				WallIndex:     wallIdx,
				SegmentLength: segmentLength,
			})

			pathLength += segmentLength
			if pathLength >= maxPathLength {
				break
			}

			// Advance with specular reflection only.
			nextDir := SpecularReflect(currentRay.Direction, hitNormal)
			currentRay = geometry.NewRay(hitPoint.Add(nextDir.Scale(wallEpsilon)), nextDir)
		}

		paths[i] = tp
	}

	receiverRadius := effectiveReceiverRadius(r.ReceiverRadius)

	return &PathCache{
		Paths:          paths,
		GeometryHash:   r.Scene.GeometryHash(),
		ReceiverRadius: receiverRadius,
		MaxBounces:     r.Config.MaxBounces,
		MaxPathLength:  maxPathLength,
	}, nil
}
