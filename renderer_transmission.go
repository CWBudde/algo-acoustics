package algoacoustics

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

const transmissionPortalOffsetMeters = 1e-5

// CrossRoomEngine renders a source and receiver separated by one or more
// portals between the same adjacent room pair. Its method set intentionally
// matches BinauralLateBufferEngine, while the alias documents field semantics.
type CrossRoomEngine = BinauralLateBufferEngine

// TransmissionRendererConfig configures the one-hop secondary-source model.
type TransmissionRendererConfig struct {
	ISM      ism.ISMConfig
	Raytrace RaytraceEngineConfig
	Hybrid   hybrid.HybridConfig
}

// TransmissionRenderer implements the Phase 21 one-hop secondary-source
// model. Full acoustic scene-graph traversal remains a Phase 25 concern.
type TransmissionRenderer struct {
	Config TransmissionRendererConfig
}

// NewTransmissionRenderer constructs the built-in one-hop renderer.
func NewTransmissionRenderer(cfg TransmissionRendererConfig) *TransmissionRenderer {
	return &TransmissionRenderer{Config: cfg}
}

// SolveEarly renders transmitted direct and specular pressure events.
func (r *TransmissionRenderer) SolveEarly(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error) {
	if r == nil {
		return nil, errors.New("transmission renderer is nil")
	}

	context, err := prepareTransmission(sc)
	if err != nil {
		return nil, err
	}

	ismConfig := r.Config.ISM
	if ismConfig.SpeedOfSound <= 0 {
		ismConfig.SpeedOfSound = acoustics.SpeedOfSound
	}

	if ismConfig.BandSpec.BandCount() == 0 {
		ismConfig.BandSpec = sc.BandSpec
	}

	bandCount := ismConfig.BandSpec.BandCount()
	emissions := make([]ir.PressureEmission, 0)
	solver := ism.ISMSolver{}

	for _, path := range context.portals {
		sourceScene, sourceOrigin := localizedISMScene(sc, context.sourceRoom)
		sourceScene.Sources = translatedSources(sc.Sources, sourceOrigin.Scale(-1))
		sourceScene.Receivers = []scene.Receiver{{Position: path.sourcePosition.Sub(sourceOrigin), Type: scene.ReceiverOmni}}

		incident, solveErr := solver.Solve(sourceScene, ismConfig)
		if solveErr != nil {
			return nil, fmt.Errorf("solve source room to portal %d: %w", path.index, solveErr)
		}

		for _, event := range incident {
			pressure := make([]float64, bandCount)
			for bandIndex := range pressure {
				gain := 1.0

				if len(event.BandGain) > 0 {
					if len(event.BandGain) != bandCount {
						return nil, fmt.Errorf("portal %d incident event band count = %d, want %d", path.index, len(event.BandGain), bandCount)
					}

					gain = event.BandGain[bandIndex]
				}

				tau := path.portal.TransmissionAt(sc.Materials, bandIndex)
				pressure[bandIndex] = event.Amplitude * gain * math.Sqrt(tau)
			}

			emissions = append(emissions, ir.PressureEmission{
				Position:     path.destinationPosition,
				TimeSeconds:  event.TimeSeconds,
				BandPressure: pressure,
				PhaseRadians: event.PhaseRadians,
			})
		}
	}

	destinationScene, destinationOrigin := localizedISMScene(sc, context.destinationRoom)
	destinationScene.Sources = nil
	destinationScene.Receivers = translatedReceivers(sc.Receivers, destinationOrigin.Scale(-1))

	for index := range emissions {
		emissions[index].Position = emissions[index].Position.Sub(destinationOrigin)
	}

	events, err := solver.SolveSecondary(destinationScene, ismConfig, emissions)
	if err != nil {
		return nil, fmt.Errorf("solve receiving-room secondary sources: %w", err)
	}

	return events, nil
}

// RenderLateMono traces transmitted diffuse energy and synthesizes a mono IR.
func (r *TransmissionRenderer) RenderLateMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	histogram, _, err := r.traceLate(sc, cfg, false)
	if err != nil {
		return nil, err
	}

	return hybrid.HistogramToBuffer(histogram, cfg.SampleRate), nil
}

// RenderLateBinaural traces transmitted directional energy and applies HRTFs.
func (r *TransmissionRenderer) RenderLateBinaural(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
	if receiver.HRTF == nil {
		return nil, nil, errors.New("binaural receiver is missing an HRTF")
	}

	histogram, tracer, err := r.traceLate(sc, cfg, true)
	if err != nil {
		return nil, nil, err
	}

	context, err := prepareTransmission(sc)
	if err != nil {
		return nil, nil, err
	}

	destinationRoom, _ := sc.RoomAt(context.destinationRoom)
	volume, ok := destinationRoom.Volume()

	if !ok {
		return nil, nil, errors.New("derive receiving-room volume for binaural late field")
	}

	bins := make([]ir.EnergyBin, len(histogram.Bins))
	for index, bin := range histogram.Bins {
		bins[index] = ir.EnergyBin{TimeSeconds: bin.TimeSeconds, BandEnergy: append([]float64(nil), bin.BandEnergy...)}
	}

	directions := make([]geometry.Vec3, len(tracer.DirectivityGroups))
	for index, group := range tracer.DirectivityGroups {
		directions[index] = receiver.WorldToHeadDir(group.Direction)
	}

	left, right, err = ir.RenderBinauralPoisson(ir.BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     histogram.BinDuration,
		Volume:          volume,
		BandSpec:        sc.BandSpec,
		SampleRate:      cfg.SampleRate,
		HRTF:            receiver.HRTF,
		DGDirections:    directions,
		DGProbabilities: raytrace.DGHitProbabilities(tracer.DirectivityGroups),
	}, rand.New(rand.NewSource(1))) //nolint:gosec // Reproducible acoustic synthesis.
	if err != nil {
		return nil, nil, fmt.Errorf("render transmitted binaural late field: %w", err)
	}

	return left, right, nil
}

// RenderMono renders the complete transmitted hybrid response.
func (r *TransmissionRenderer) RenderMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	events, err := r.SolveEarly(sc, cfg)
	if err != nil {
		return nil, err
	}

	early, err := ir.RenderMono(events, cfg)
	if err != nil {
		return nil, fmt.Errorf("render transmitted early field: %w", err)
	}

	late, err := r.RenderLateMono(sc, cfg)
	if err != nil {
		return nil, err
	}

	late = hybrid.AlignLateTail(late, events, r.Config.Hybrid)
	combined := hybrid.CombineBuffers(early, late, r.Config.Hybrid)

	if combined == nil {
		return nil, errors.New("combine transmitted mono field")
	}

	return combined, nil
}

// RenderBinaural renders the complete transmitted hybrid BRIR.
func (r *TransmissionRenderer) RenderBinaural(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
	events, err := r.SolveEarly(sc, cfg)
	if err != nil {
		return nil, nil, err
	}

	headEvents := append([]ir.Event(nil), events...)
	for index := range headEvents {
		headEvents[index].Direction = receiver.WorldToHeadDir(headEvents[index].Direction)
	}

	earlyLeft, earlyRight, err := ir.RenderBinaural(headEvents, receiver.HRTF, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("render transmitted binaural early field: %w", err)
	}

	lateLeft, lateRight, err := r.RenderLateBinaural(sc, receiver, cfg)
	if err != nil {
		return nil, nil, err
	}

	lateLeft = hybrid.AlignLateTail(lateLeft, events, r.Config.Hybrid)
	lateRight = hybrid.AlignLateTail(lateRight, events, r.Config.Hybrid)
	left = hybrid.CombineBuffers(earlyLeft, lateLeft, r.Config.Hybrid)
	right = hybrid.CombineBuffers(earlyRight, lateRight, r.Config.Hybrid)

	if left == nil || right == nil {
		return nil, nil, errors.New("combine transmitted binaural field")
	}

	return left, right, nil
}

func (r *TransmissionRenderer) traceLate(
	sc *scene.Scene,
	cfg ir.RenderConfig,
	directional bool,
) (*raytrace.EnergyHistogram, *raytrace.RayTracer, error) {
	if r == nil {
		return nil, nil, errors.New("transmission renderer is nil")
	}

	context, err := prepareTransmission(sc)
	if err != nil {
		return nil, nil, err
	}

	launch := r.Config.Raytrace.Launch
	if launch.MaxTimeSeconds <= 0 {
		launch.MaxTimeSeconds = cfg.DurationSeconds
	}

	if launch.SpeedOfSound <= 0 {
		launch.SpeedOfSound = acoustics.SpeedOfSound
	}

	incident, err := r.tracePortalSurfaces(sc, context, launch)
	if err != nil {
		return nil, nil, err
	}

	emissions := make([]ir.EnergyEmission, 0)
	bandCount := sc.BandSpec.BandCount()

	for index, path := range context.portals {
		transmission := make([]float64, bandCount)
		for bandIndex := range transmission {
			transmission[bandIndex] = path.portal.TransmissionAt(sc.Materials, bandIndex)
		}

		filtered, filterErr := raytrace.FilterTransmission(incident[index], transmission)
		if filterErr != nil {
			return nil, nil, fmt.Errorf("filter portal %d transmission: %w", path.index, filterErr)
		}

		emissions = append(emissions, raytrace.EnergyEmissions(filtered, path.destinationPosition)...)
	}

	destinationScene := singleRoomScene(sc, context.destinationRoom)
	destinationScene.Sources = nil

	destinationScene.Receivers = append([]scene.Receiver(nil), sc.Receivers...)

	destinationTracer := r.newTransmissionDestinationTracer(destinationScene, launch, directional)

	histogram, err := destinationTracer.TraceSecondary(emissions)
	if err != nil {
		return nil, nil, fmt.Errorf("trace receiving-room secondary sources: %w", err)
	}

	return histogram, destinationTracer, nil
}

func (r *TransmissionRenderer) tracePortalSurfaces(
	sc *scene.Scene,
	context transmissionContext,
	launch raytrace.LaunchConfig,
) ([]*raytrace.EnergyHistogram, error) {
	sourceScene := singleRoomScene(sc, context.sourceRoom)
	sourceScene.Sources = append([]scene.Source(nil), sc.Sources...)
	sourceScene.Receivers = nil
	sourceTracer := &raytrace.RayTracer{
		Config:             launch,
		Scene:              sourceScene,
		BinDurationSeconds: r.Config.Raytrace.BinDurationSeconds,
	}

	detectors := make([]raytrace.SurfaceReceiver, len(context.portals))
	for index, path := range context.portals {
		polygon := append([]geometry.Vec3(nil), path.portal.Polygon...)
		if path.portal.RoomIndices[0] == context.sourceRoom {
			reverseVec3s(polygon)
		}

		detector, err := raytrace.NewSurfaceReceiver(polygon)
		if err != nil {
			return nil, fmt.Errorf("construct portal %d surface receiver: %w", path.index, err)
		}

		detectors[index] = detector
	}

	incident, err := sourceTracer.TraceSurfaces(detectors)
	if err != nil {
		return nil, fmt.Errorf("trace source-room portal surfaces: %w", err)
	}

	return incident, nil
}

func (r *TransmissionRenderer) newTransmissionDestinationTracer(
	destinationScene *scene.Scene,
	launch raytrace.LaunchConfig,
	directional bool,
) *raytrace.RayTracer {
	tracer := &raytrace.RayTracer{
		Config:             launch,
		Scene:              destinationScene,
		ReceiverRadius:     r.Config.Raytrace.ReceiverRadius,
		BinDurationSeconds: r.Config.Raytrace.BinDurationSeconds,
	}
	if !directional {
		return tracer
	}

	azimuth := r.Config.Raytrace.DirectionGroupAzimuth
	if azimuth <= 0 {
		azimuth = defaultDirectionGroupAzimuth
	}

	elevation := r.Config.Raytrace.DirectionGroupElevation
	if elevation <= 0 {
		elevation = defaultDirectionGroupElevation
	}

	tracer.DirectivityGroups = raytrace.NewDirectivityGroups(azimuth, elevation)

	return tracer
}

type transmissionPortalPath struct {
	index               int
	portal              scene.Portal
	sourcePosition      geometry.Vec3
	destinationPosition geometry.Vec3
}

type transmissionContext struct {
	sourceRoom      int
	destinationRoom int
	portals         []transmissionPortalPath
}

func prepareTransmission(sc *scene.Scene) (transmissionContext, error) {
	if sc == nil {
		return transmissionContext{}, errors.New("scene is nil")
	}

	err := scene.Validate(sc)
	if err != nil {
		return transmissionContext{}, fmt.Errorf("validate transmission scene: %w", err)
	}

	if sc.RoomCount() < 2 {
		return transmissionContext{}, errors.New("cross-room transmission requires a multi-room scene")
	}

	sourceRoom, destinationRoom, err := transmissionRoomPair(sc)
	if err != nil {
		return transmissionContext{}, err
	}

	for _, roomIndex := range []int{sourceRoom, destinationRoom} {
		room, found := sc.RoomAt(roomIndex)
		if !found || room.Kind != scene.RoomKindShoebox || room.Shoebox == nil {
			return transmissionContext{}, errors.New("phase 21 cross-room transmission supports shoebox rooms only")
		}
	}

	context := transmissionContext{sourceRoom: sourceRoom, destinationRoom: destinationRoom}
	for index, portal := range sc.Portals {
		connects := portal.RoomIndices == [2]int{sourceRoom, destinationRoom} ||
			portal.RoomIndices == [2]int{destinationRoom, sourceRoom}
		if !connects {
			continue
		}

		direction := portal.Normal()
		if portal.RoomIndices[0] != sourceRoom {
			direction = direction.Scale(-1)
		}

		center := portal.Center()
		context.portals = append(context.portals, transmissionPortalPath{
			index:               index,
			portal:              portal,
			sourcePosition:      center.Sub(direction.Scale(transmissionPortalOffsetMeters)),
			destinationPosition: center.Add(direction.Scale(transmissionPortalOffsetMeters)),
		})
	}

	if len(context.portals) == 0 {
		return transmissionContext{}, errors.New("source and receiver rooms are not connected by a portal")
	}

	return context, nil
}

func transmissionRoomPair(sc *scene.Scene) (sourceRoom, destinationRoom int, err error) {
	if len(sc.Sources) != 1 {
		return 0, 0, fmt.Errorf("cross-room transmission requires exactly one source, got %d", len(sc.Sources))
	}

	if len(sc.Receivers) != 1 {
		return 0, 0, fmt.Errorf("cross-room transmission requires exactly one receiver, got %d", len(sc.Receivers))
	}

	sourceRoom, ok := sc.RoomIndexAt(sc.Sources[0].Position)
	if !ok {
		return 0, 0, errors.New("source must belong to exactly one room")
	}

	destinationRoom, ok = sc.RoomIndexAt(sc.Receivers[0].Position)
	if !ok {
		return 0, 0, errors.New("receiver must belong to exactly one room")
	}

	if sourceRoom == destinationRoom {
		return 0, 0, errors.New("source and receiver are in the same room")
	}

	return sourceRoom, destinationRoom, nil
}

func singleRoomScene(sc *scene.Scene, roomIndex int) *scene.Scene {
	copyScene := *sc
	room, _ := sc.RoomAt(roomIndex)
	copyScene.Room = *room
	copyScene.Rooms = nil
	copyScene.Portals = nil

	return &copyScene
}

// localizedISMScene extracts a room and translates shoebox geometry to the
// origin expected by the current shoebox image-source implementation.
func localizedISMScene(sc *scene.Scene, roomIndex int) (*scene.Scene, geometry.Vec3) {
	copyScene := singleRoomScene(sc, roomIndex)
	if copyScene.Room.Shoebox == nil {
		return copyScene, geometry.Vec3Zero
	}

	roomCopy := copyScene.Room
	shoeboxCopy := *roomCopy.Shoebox
	origin := shoeboxCopy.Origin
	shoeboxCopy.Origin = geometry.Vec3Zero
	roomCopy.Shoebox = &shoeboxCopy
	copyScene.Room = roomCopy

	return copyScene, origin
}

func translatedSources(sources []scene.Source, offset geometry.Vec3) []scene.Source {
	translated := append([]scene.Source(nil), sources...)
	for index := range translated {
		translated[index].Position = translated[index].Position.Add(offset)
	}

	return translated
}

func translatedReceivers(receivers []scene.Receiver, offset geometry.Vec3) []scene.Receiver {
	translated := append([]scene.Receiver(nil), receivers...)
	for index := range translated {
		translated[index].Position = translated[index].Position.Add(offset)
	}

	return translated
}

func reverseVec3s(values []geometry.Vec3) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
