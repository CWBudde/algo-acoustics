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

// SolveWithDiffraction returns the standard ISM events plus first-order
// diffraction events accumulated from the room mesh.
func (s ISMSolver) SolveWithDiffraction(sc *scene.Scene, cfg ISMConfig) ([]ir.Event, error) {
	events, err := s.Solve(sc, cfg)
	if err != nil {
		return nil, err
	}

	if sc == nil || sc.Room.Mesh == nil || len(sc.Sources) == 0 || len(sc.Receivers) == 0 {
		return events, nil
	}

	bandSpec := cfg.BandSpec
	if bandSpec.BandCount() == 0 {
		bandSpec = sc.BandSpec
	}

	speedOfSound := cfg.SpeedOfSound
	if speedOfSound <= 0 {
		speedOfSound = acoustics.SpeedOfSound
	}

	edges := geometry.ExtractDiffractionEdges(sc.Room.Mesh)
	if len(edges) == 0 {
		return events, nil
	}

	receiver := sc.Receivers[0]
	for _, source := range sc.Sources {
		events = append(events, DiffractionEvents(source, receiver, edges, sc.Room.Mesh, bandSpec, speedOfSound)...)
	}

	sortIR(events)
	return events, nil
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

	phi, phiPrime, betaZero := diffractionAngles(path)
	spreadingFactor := geometry.WedgeSpreadingFactor(path.SourceDistance, path.ReceiverDistance)
	if spreadingFactor <= 0 {
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

		k := 2 * math.Pi * centerFreq / speedOfSound
		l := geometry.WedgeDistanceParameter(path.SourceDistance, path.ReceiverDistance, betaZero)
		diffraction := geometry.WedgeDiffraction(phi, phiPrime, betaZero, path.Edge.WedgeIndex, k, l)
		if math.IsNaN(real(diffraction)) || math.IsNaN(imag(diffraction)) || math.IsInf(real(diffraction), 0) || math.IsInf(imag(diffraction), 0) {
			continue
		}

		magnitude := cmplx.Abs(diffraction)
		if math.IsNaN(magnitude) || math.IsInf(magnitude, 0) || magnitude <= 0 {
			continue
		}

		estimatedAmplitude := sourceAmplitude * launchGain * magnitude * spreadingFactor / math.Sqrt(distance)
		if estimatedAmplitude <= 0 {
			continue
		}

		referenceAmplitude := sourceAmplitude * referenceGain / directDistance
		if referenceAmplitude > 0 && estimatedAmplitude/referenceAmplitude < minRelativeLinear {
			continue
		}

		events = append(events, ir.Event{
			TimeSeconds:    distance / speedOfSound,
			Amplitude:      estimatedAmplitude,
			Direction:      pathDirection,
			DistanceMeters: distance,
			PhaseRadians:   -k*distance + cmplx.Phase(diffraction),
			Kind:           ir.EventDiffraction,
		})
	}

	return events
}

func diffractionAngles(path geometry.DiffractionPath) (phi, phiPrime, betaZero float64) {
	edge := path.Edge
	if edge.WedgeIndex <= 0 {
		return 0, 0, 0
	}

	betaZero = math.Pi / edge.WedgeIndex

	phi = edgeAngle(edge, path.Receiver.Sub(path.Point))
	phiPrime = edgeAngle(edge, path.Source.Sub(path.Point))

	return phi, phiPrime, betaZero
}

func edgeAngle(edge geometry.DiffractionEdge, vector geometry.Vec3) float64 {
	axis := edge.Direction.Normalize()
	if axis == geometry.Vec3Zero {
		return 0
	}

	transverse := vector.Sub(axis.Scale(vector.Dot(axis)))
	if transverse.Norm() <= pathEpsilon {
		return 0
	}

	reference := edge.FaceONormal.Normalize()
	if reference == geometry.Vec3Zero {
		return 0
	}

	basis := axis.Cross(reference).Normalize()
	if basis == geometry.Vec3Zero {
		return 0
	}

	x := transverse.Dot(reference)
	y := transverse.Dot(basis)
	angle := math.Atan2(y, x)
	if angle < 0 {
		angle += 2 * math.Pi
	}

	return angle
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

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
