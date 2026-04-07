package ism

import (
	"errors"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// EvaluateShoebox produces ir.Events from pre-computed shoebox image sources.
// The result is bit-identical to ISMSolver{}.Solve() for the same scene and
// image sources.
func EvaluateShoebox(sources []ImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if sc.Room.Shoebox == nil {
		return nil, errors.New("EvaluateShoebox requires shoebox dimensions")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("EvaluateShoebox requires at least one source")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("EvaluateShoebox requires at least one receiver")
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	receiver := sc.Receivers[0]
	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range sources {
			if imgSrc.Order == 0 {
				continue
			}

			path, ok := reflectionPath(imgSrc, receiver.Position)
			if !ok {
				continue
			}

			event, ok := specularEvent(source, receiver, imgSrc, path, sc.Room.Shoebox, sc.Materials, bandSpec, speedOfSound)
			if ok {
				events = append(events, event)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}

// EvaluateMesh produces ir.Events from pre-computed mesh image sources.
// The result is bit-identical to ISMSolver{}.Solve() for the same scene and
// image sources.
func EvaluateMesh(sources []MeshImageSource, sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if sc.Room.Mesh == nil {
		return nil, errors.New("EvaluateMesh requires a mesh room")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("EvaluateMesh requires at least one source")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("EvaluateMesh requires at least one receiver")
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	bvh := geometry.BuildBVH(sc.Room.Mesh)
	material := meshMaterial(sc)
	receiver := sc.Receivers[0]

	events := make([]ir.Event, 0)

	for _, source := range sc.Sources {
		direct, ok := directEvent(source, receiver, bandSpec, speedOfSound)
		if ok {
			events = append(events, direct)
		}

		for _, imgSrc := range sources {
			if imgSrc.Order == 0 {
				continue
			}

			event, ok := meshSpecularEvent(source, receiver, imgSrc, sc.Room.Mesh, bvh, material, bandSpec, speedOfSound)
			if ok {
				events = append(events, event)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}
