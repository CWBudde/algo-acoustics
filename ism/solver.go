package ism

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

// ISMConfig configures the shoebox image-source solver.
type ISMConfig struct {
	MaxOrder     int
	SpeedOfSound float64
	BandSpec     acoustics.BandSpec
}

// ISMSolver emits sparse direct and specular image-source events.
type ISMSolver struct{}

// Solve computes mono-oriented direct and specular events for one receiver.
func (ISMSolver) Solve(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if cfg.MaxOrder < 0 {
		return nil, errors.New("max order must be non-negative")
	}

	err := scene.Validate(sc)
	if err != nil {
		return nil, fmt.Errorf("validate scene: %w", err)
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("ISM solver requires at least one source")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("ISM solver requires exactly one receiver")
	}

	if len(sc.Receivers) > 1 {
		return nil, fmt.Errorf("ISM solver currently supports exactly one receiver, got %d", len(sc.Receivers))
	}

	switch sc.Room.Kind {
	case scene.RoomKindMesh:
		if sc.Room.Mesh == nil {
			return nil, errors.New("ISM solver requires a mesh for mesh rooms")
		}

		return solveMesh(sc, cfg)
	case scene.RoomKindShoebox:
		if sc.Room.Shoebox == nil {
			return nil, errors.New("ISM solver requires shoebox dimensions")
		}
	default:
		return nil, fmt.Errorf("ISM solver does not support room kind %q", sc.Room.Kind)
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

		for _, imageSource := range GenerateImageSources(source.Position, sc.Room.Shoebox, cfg.MaxOrder) {
			if imageSource.Order == 0 {
				continue
			}

			path, ok := reflectionPath(imageSource, receiver.Position)
			if !ok {
				continue
			}

			event, ok := specularEvent(source, receiver, imageSource, path, sc.Room.Shoebox, sc.Materials, bandSpec, speedOfSound)
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

func directEvent(source scene.Source, receiver scene.Receiver, bandSpec acoustics.BandSpec, speedOfSound float64) (ir.Event, bool) {
	distance := source.Position.Distance(receiver.Position)
	if distance <= pathEpsilon {
		return ir.Event{}, false
	}

	bandGain := directivityBandGain(source, bandSpec, receiver.Position)
	if bandGainSilent(bandGain) {
		return ir.Event{}, false
	}

	return ir.Event{
		TimeSeconds:    distance / speedOfSound,
		Amplitude:      sourceAmplitude(source) / distance,
		Direction:      source.Position.Sub(receiver.Position).Normalize(),
		DistanceMeters: distance,
		BandGain:       bandGain,
		Kind:           ir.EventDirect,
	}, true
}

func specularEvent(source scene.Source, receiver scene.Receiver, imgSrc ImageSource, path []reflectionPoint, room *scene.Shoebox, materials map[string]scene.Material, bandSpec acoustics.BandSpec, speedOfSound float64) (ir.Event, bool) {
	distance := receiver.Position.Distance(imgSrc.Position)
	if distance <= pathEpsilon {
		return ir.Event{}, false
	}

	orderedPath := sourceOrderedPath(path)

	target := receiver.Position
	if len(orderedPath) > 0 {
		target = orderedPath[0].Point
	}

	bandGain := directivityBandGain(source, bandSpec, target)
	if bandGainSilent(bandGain) {
		return ir.Event{}, false
	}

	for bandIndex := range bandGain {
		bandGain[bandIndex] *= pathPressureReflectance(orderedPath, source.Position, room, materials, bandIndex)
	}

	if bandGainSilent(bandGain) {
		return ir.Event{}, false
	}

	return ir.Event{
		TimeSeconds:    distance / speedOfSound,
		Amplitude:      sourceAmplitude(source) / distance,
		Direction:      imgSrc.Position.Sub(receiver.Position).Normalize(),
		DistanceMeters: distance,
		BandGain:       bandGain,
		Kind:           ir.EventSpecular,
	}, true
}

func sourceAmplitude(source scene.Source) float64 {
	return acoustics.DecibelToLinear(source.GainDB)
}

func directivityBandGain(source scene.Source, bandSpec acoustics.BandSpec, target geometry.Vec3) []float64 {
	bandCount := bandSpec.BandCount()
	if bandCount == 0 {
		return nil
	}

	bandGain := make([]float64, bandCount)
	if source.Directivity == nil {
		for index := range bandGain {
			bandGain[index] = 1
		}

		return bandGain
	}

	direction := source.DirectionTo(target)
	for index, centerFrequency := range bandSpec.CenterFreqs {
		bandGain[index] = source.Directivity.GainLinear(centerFrequency, direction)
	}

	return bandGain
}

func bandGainSilent(bandGain []float64) bool {
	if len(bandGain) == 0 {
		return false
	}

	for _, value := range bandGain {
		if math.Abs(value) > pathEpsilon {
			return false
		}
	}

	return true
}

func sourceOrderedPath(path []reflectionPoint) []reflectionPoint {
	if len(path) <= 1 {
		return append([]reflectionPoint(nil), path...)
	}

	ordered := make([]reflectionPoint, len(path))
	for i := range path {
		ordered[i] = path[len(path)-1-i]
	}

	return ordered
}

func pathPressureReflectance(path []reflectionPoint, source geometry.Vec3, room *scene.Shoebox, materials map[string]scene.Material, bandIndex int) float64 {
	if len(path) == 0 {
		return 1
	}

	pressure := 1.0
	previous := source

	for _, reflection := range path {
		cosAngle := reflectionCosine(previous, reflection.Point, reflection.Wall)
		material := materials[room.WallMaterials[reflection.Wall]]
		pressure *= wayverbPressureReflectance(material.AbsorptionAt(bandIndex), cosAngle)
		previous = reflection.Point
	}

	return pressure
}

func reflectionCosine(previous, point geometry.Vec3, wall int) float64 {
	direction := point.Sub(previous).Normalize()
	if direction == geometry.Vec3Zero {
		return 0
	}

	cosAngle := math.Abs(direction.Dot(wallNormal(wall)))
	if cosAngle < 0 {
		return 0
	}

	if cosAngle > 1 {
		return 1
	}

	return cosAngle
}

func wallNormal(wall int) geometry.Vec3 {
	switch wall {
	case wallNegX:
		return geometry.Vec3{X: 1}
	case wallPosX:
		return geometry.Vec3{X: -1}
	case wallNegY:
		return geometry.Vec3{Y: 1}
	case wallPosY:
		return geometry.Vec3{Y: -1}
	case wallNegZ:
		return geometry.Vec3{Z: 1}
	case wallPosZ:
		return geometry.Vec3{Z: -1}
	default:
		return geometry.Vec3Zero
	}
}

func wayverbPressureReflectance(absorption, cosAngle float64) float64 {
	if absorption < 0 {
		absorption = 0
	}

	if absorption > 1 {
		absorption = 1
	}

	if cosAngle < 0 {
		cosAngle = 0
	}

	if cosAngle > 1 {
		cosAngle = 1
	}

	magnitude := math.Sqrt(math.Max(0, 1-absorption))
	if magnitude >= 1-1e-12 {
		return 1
	}

	impedance := (1 + magnitude) / (1 - magnitude)
	tmp := impedance * cosAngle

	return (tmp - 1) / (tmp + 1)
}
