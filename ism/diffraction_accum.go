package ism

import (
	"math"
	"math/cmplx"
	"sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

const diffractionCullingLevelDB = -60.0

// SolveWithDiffraction returns the standard ISM events plus diffraction events
// accumulated from the room mesh. It enables diffraction regardless of the
// value of cfg.EnableDiffraction.
func (s ISMSolver) SolveWithDiffraction(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	cfg.EnableDiffraction = true

	return s.Solve(sc, cfg)
}

// DiffractionEvents accumulates first-order diffraction contributions for one
// source/receiver pair. The returned events are band-specific and can be
// rendered independently.
func DiffractionEvents(source scene.Source, receiver scene.Receiver, edges []geometry.DiffractionEdge, mesh *geometry.Mesh, bandSpec acoustics.BandSpec, speedOfSound float64) []ir.Event {
	paths := EnumerateFirstOrderDiffractionPaths(source.Position, receiver.Position, edges, mesh)
	if len(paths) == 0 {
		return nil
	}

	events := make([]ir.Event, 0, len(paths)*max(1, bandSpec.BandCount()))
	for _, path := range paths {
		events = append(events, diffractionEventsForPath(source, receiver, path, bandSpec, speedOfSound, diffractionCullingLevelDB)...)
	}

	sortIR(events)

	return events
}

func diffractionEventsForPath(source scene.Source, receiver scene.Receiver, path geometry.DiffractionPath, bandSpec acoustics.BandSpec, speedOfSound, minRelativeLevelDB float64) []ir.Event {
	bandCount := bandSpec.BandCount()
	if bandCount <= 0 {
		return nil
	}

	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	distance := path.TotalDistance
	if distance <= pathEpsilon {
		return nil
	}

	sourceAmplitude := sourceAmplitude(source)
	if sourceAmplitude <= 0 {
		return nil
	}

	sourceBandGain := directivityBandGain(source, bandSpec, path.Point)
	receiverBandGain := directivityBandGain(source, bandSpec, receiver.Position)

	if bandGainSilent(sourceBandGain) {
		return nil
	}

	directDistance := source.Position.Distance(receiver.Position)
	if directDistance <= pathEpsilon {
		directDistance = distance
	}

	minRelativeLinear := math.Pow(10, minRelativeLevelDB/20)
	pathDirection := path.Receiver.Sub(path.Point).Normalize()
	events := make([]ir.Event, 0, bandCount)

	for bandIndex, centerFreq := range bandSpec.CenterFreqs {
		if bandIndex >= len(sourceBandGain) || bandIndex >= len(receiverBandGain) {
			break
		}

		launchGain := sourceBandGain[bandIndex]
		if launchGain <= 0 {
			continue
		}

		referenceGain := receiverBandGain[bandIndex]
		if referenceGain <= 0 {
			referenceGain = 1
		}

		diffraction, err := geometry.BTMETransfer(path.Source, path.Receiver, path.Edge, centerFreq, speedOfSound)
		if err != nil {
			continue
		}

		if math.IsNaN(real(diffraction)) || math.IsNaN(imag(diffraction)) || math.IsInf(real(diffraction), 0) || math.IsInf(imag(diffraction), 0) {
			continue
		}

		magnitude := cmplx.Abs(diffraction)
		if math.IsNaN(magnitude) || math.IsInf(magnitude, 0) || magnitude <= 0 {
			continue
		}

		estimatedAmplitude := sourceAmplitude * launchGain * magnitude
		if estimatedAmplitude <= 0 {
			continue
		}

		referenceAmplitude := sourceAmplitude * referenceGain / directDistance
		if referenceAmplitude > 0 && estimatedAmplitude/referenceAmplitude < minRelativeLinear {
			continue
		}

		bandGain := make([]float64, bandCount)
		bandGain[bandIndex] = 1
		events = append(events, ir.Event{
			TimeSeconds:    distance / speedOfSound,
			Amplitude:      estimatedAmplitude,
			Direction:      pathDirection,
			DistanceMeters: distance,
			BandGain:       bandGain,
			PhaseRadians:   cmplx.Phase(diffraction),
			Kind:           ir.EventDiffraction,
		})
	}

	return events
}

func secondOrderDiffractionEvents(
	source scene.Source,
	receiver scene.Receiver,
	paths []geometry.SecondOrderDiffractionPath,
	bandSpec acoustics.BandSpec,
	speedOfSound float64,
) []ir.Event {
	bandCount := bandSpec.BandCount()
	if bandCount <= 0 {
		return nil
	}

	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	sourceAmplitude := sourceAmplitude(source)
	if sourceAmplitude <= 0 {
		return nil
	}

	directDistance := source.Position.Distance(receiver.Position)
	if directDistance <= pathEpsilon {
		return nil
	}

	minRelativeLinear := math.Pow(10, diffractionCullingLevelDB/20)
	events := make([]ir.Event, 0, len(paths)*bandCount)

	for _, path := range paths {
		launchGain := directivityBandGain(source, bandSpec, path.Point1)
		if bandGainSilent(launchGain) {
			continue
		}

		intermediate := path.Point1.Add(path.Point2).Scale(0.5)
		arrivalDirection := path.Point2.Sub(receiver.Position).Normalize()

		for bandIndex, centerFreq := range bandSpec.CenterFreqs {
			if bandIndex >= len(launchGain) || launchGain[bandIndex] <= 0 {
				continue
			}

			first, err := geometry.BTMETransfer(source.Position, intermediate, path.Edge1, centerFreq, speedOfSound)
			if err != nil {
				continue
			}

			second, err := geometry.BTMETransfer(intermediate, receiver.Position, path.Edge2, centerFreq, speedOfSound)
			if err != nil {
				continue
			}

			transfer := first * second
			magnitude := cmplx.Abs(transfer)
			amplitude := sourceAmplitude * launchGain[bandIndex] * magnitude

			if amplitude <= 0 || math.IsNaN(amplitude) || math.IsInf(amplitude, 0) {
				continue
			}

			if amplitude/(sourceAmplitude/directDistance) < minRelativeLinear {
				continue
			}

			bandGain := make([]float64, bandCount)
			bandGain[bandIndex] = 1
			events = append(events, ir.Event{
				TimeSeconds:    path.TotalDistance / speedOfSound,
				Amplitude:      amplitude,
				Direction:      arrivalDirection,
				DistanceMeters: path.TotalDistance,
				BandGain:       bandGain,
				PhaseRadians:   cmplx.Phase(transfer),
				Kind:           ir.EventDiffraction,
			})
		}
	}

	return events
}

func sortIR(events []ir.Event) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}

		if events[i].DistanceMeters != events[j].DistanceMeters {
			return events[i].DistanceMeters < events[j].DistanceMeters
		}

		if events[i].Amplitude != events[j].Amplitude {
			return events[i].Amplitude < events[j].Amplitude
		}

		return events[i].PhaseRadians < events[j].PhaseRadians
	})
}
