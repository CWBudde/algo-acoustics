package algoacoustics

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/metrics"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestTransmissionRendererEarlyUsesPressureTransmission(t *testing.T) {
	t.Parallel()

	closed := transmissionTestScene(0.25)
	open := transmissionTestScene(1)
	renderer := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})
	cfg := transmissionTestRenderConfig(closed)

	closedEvents, err := renderer.SolveEarly(closed, cfg)
	if err != nil {
		t.Fatalf("SolveEarly(closed) error = %v", err)
	}

	openEvents, err := renderer.SolveEarly(open, cfg)
	if err != nil {
		t.Fatalf("SolveEarly(open) error = %v", err)
	}

	if len(closedEvents) != 1 || len(openEvents) != 1 {
		t.Fatalf("event counts = %d, %d, want one direct transmission each", len(closedEvents), len(openEvents))
	}

	closedPressure := closedEvents[0].Amplitude * closedEvents[0].BandGain[0]
	openPressure := openEvents[0].Amplitude * openEvents[0].BandGain[0]

	if diff := math.Abs(closedPressure/openPressure - 0.5); diff > 1e-9 {
		t.Fatalf("closed/open pressure ratio = %v, want 0.5", closedPressure/openPressure)
	}

	if closedEvents[0].Kind != ir.EventTransmission {
		t.Fatalf("event kind = %v, want transmission", closedEvents[0].Kind)
	}
}

func TestTransmissionRendererEarlyIsTranslationInvariant(t *testing.T) {
	t.Parallel()

	base := transmissionTestScene(0.25)
	shifted := shiftedTransmissionScene(base, geometry.Vec3{X: 10, Y: -3, Z: 2})
	renderer := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}})

	baseEvents, err := renderer.SolveEarly(base, transmissionTestRenderConfig(base))
	if err != nil {
		t.Fatalf("SolveEarly(base) error = %v", err)
	}

	shiftedEvents, err := renderer.SolveEarly(shifted, transmissionTestRenderConfig(shifted))
	if err != nil {
		t.Fatalf("SolveEarly(shifted) error = %v", err)
	}

	if len(baseEvents) <= 1 || len(shiftedEvents) != len(baseEvents) {
		t.Fatalf("event counts = base %d shifted %d", len(baseEvents), len(shiftedEvents))
	}

	// Match as a multiset rather than index-by-index.  Translating the scene
	// perturbs each arrival time by up to an ulp, and distinct events here are
	// separated by less than that, so the solver's (time, kind, distance) sort
	// legitimately permutes them.  Requiring a bijection keeps the physical
	// claim -- every event survives translation, none appears or vanishes --
	// without asserting an ordering the arithmetic cannot guarantee.
	unmatched := make([]bool, len(shiftedEvents))

	for index, baseEvent := range baseEvents {
		matched := -1

		for candidate, taken := range unmatched {
			if taken {
				continue
			}

			if eventsAgreeAfterTranslation(baseEvent, shiftedEvents[candidate]) {
				matched = candidate

				break
			}
		}

		if matched < 0 {
			t.Fatalf("base event[%d] has no counterpart after translation: %#v", index, baseEvent)
		}

		unmatched[matched] = true
	}
}

// eventsAgreeAfterTranslation reports whether two events describe the same
// arrival, up to the rounding a rigid translation of the scene introduces.
func eventsAgreeAfterTranslation(a, b ir.Event) bool {
	if a.Kind != b.Kind ||
		math.Abs(a.TimeSeconds-b.TimeSeconds) > 1e-12 ||
		math.Abs(a.Amplitude-b.Amplitude) > 1e-12 ||
		a.Direction.Distance(b.Direction) > 1e-12 {
		return false
	}

	if len(a.BandGain) != len(b.BandGain) {
		return false
	}

	for bandIndex := range a.BandGain {
		if math.Abs(a.BandGain[bandIndex]-b.BandGain[bandIndex]) > 1e-12 {
			return false
		}
	}

	return true
}

func TestTransmissionRendererValidatesOriginalScene(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	reverseVec3s(sc.Portals[0].Polygon)

	renderer := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	_, err := renderer.SolveEarly(sc, transmissionTestRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "validate transmission scene") {
		t.Fatalf("SolveEarly() error = %v, want scene-validation error", err)
	}
}

func TestRendererRoutesMultiRoomMonoAndBinaural(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.5)
	renderCfg := transmissionTestRenderConfig(sc)
	transmission := NewTransmissionRenderer(TransmissionRendererConfig{
		ISM: ism.ISMConfig{MaxOrder: 0},
		Raytrace: RaytraceEngineConfig{
			Launch: raytrace.LaunchConfig{
				NumRays:        256,
				MaxBounces:     2,
				MaxTimeSeconds: renderCfg.DurationSeconds,
				SpeedOfSound:   acoustics.SpeedOfSound,
			},
			ReceiverRadius:     0.3,
			BinDurationSeconds: 0.005,
		},
		Hybrid: hybrid.HybridConfig{
			CrossoverMode:        hybrid.TimeBased,
			CrossoverTimeSeconds: 0.03,
			SmoothenCrossover:    true,
		},
	})
	renderer := Renderer{Transmission: transmission}

	mono, err := renderer.RenderMono(sc, renderCfg)
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	if !hasNonZeroSample(mono) {
		t.Fatal("RenderMono() returned silence")
	}

	left, right, err := renderer.RenderStereo(sc, renderCfg)
	if err != nil {
		t.Fatalf("RenderStereo() error = %v", err)
	}

	if !hasNonZeroSample(left) || !hasNonZeroSample(right) {
		t.Fatal("RenderStereo() returned a silent channel")
	}
}

func TestRendererRejectsNilCrossRoomBuffers(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.5)
	renderer := Renderer{Transmission: nilCrossRoomEngine{}}

	_, err := renderer.RenderMono(sc, transmissionTestRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "nil mono buffer") {
		t.Fatalf("RenderMono() error = %v, want nil-buffer error", err)
	}

	_, _, err = renderer.RenderStereo(sc, transmissionTestRenderConfig(sc))
	if err == nil || !strings.Contains(err.Error(), "nil binaural buffer") {
		t.Fatalf("RenderStereo() error = %v, want nil-buffer error", err)
	}
}

type nilCrossRoomEngine struct{}

func (nilCrossRoomEngine) RenderMono(*scene.Scene, ir.RenderConfig) (*ir.Buffer, error) {
	return nil, nil //nolint:nilnil // Deliberately violates the engine contract to verify Renderer validation.
}

func (nilCrossRoomEngine) RenderBinaural(
	*scene.Scene,
	scene.Receiver,
	ir.RenderConfig,
) (*ir.Buffer, *ir.Buffer, error) {
	return nil, nil, nil
}

func TestTransmissionValidationReductionIndexAcrossBands(t *testing.T) {
	t.Parallel()

	centers := []float64{
		315, 400, 500, 630, 800, 1000, 1250, 1600, 2000,
		2500, 3150, 4000, 5000, 6300, 8000, 10000, 12500, 16000,
	}
	wantedReduction := []float64{
		22, 24, 26, 28, 30, 32, 34, 36, 38,
		40, 42, 43, 44, 45, 46, 47, 48, 49,
	}
	bandSpec := thirdOctaveBandSpec(centers)
	closed := transmissionValidationScene(bandSpec, wantedReduction, scene.PortalClosed)
	open := transmissionValidationScene(bandSpec, wantedReduction, scene.PortalOpen)

	for name, sc := range map[string]*scene.Scene{"closed": closed, "open": open} {
		err := scene.Validate(sc)
		if err != nil {
			t.Fatalf("Validate(%s) error = %v", name, err)
		}

		for roomIndex := range sc.Rooms {
			volume, ok := sc.Rooms[roomIndex].Volume()
			if !ok || math.Abs(volume-90) > 1e-12 {
				t.Fatalf("room %d volume = %v, want 90", roomIndex, volume)
			}
		}
	}

	if area := closed.Portals[0].Area(); math.Abs(area-16) > 1e-12 {
		t.Fatalf("partition area = %v, want 16", area)
	}

	renderer := NewTransmissionRenderer(TransmissionRendererConfig{
		ISM: ism.ISMConfig{MaxOrder: 0, BandSpec: bandSpec},
	})
	cfg := ir.RenderConfig{SampleRate: closed.SampleRate, DurationSeconds: 0.1, BandSpec: bandSpec}

	closedEvents, err := renderer.SolveEarly(closed, cfg)
	if err != nil {
		t.Fatalf("SolveEarly(closed) error = %v", err)
	}

	openEvents, err := renderer.SolveEarly(open, cfg)
	if err != nil {
		t.Fatalf("SolveEarly(open) error = %v", err)
	}

	if len(closedEvents) != 1 || len(openEvents) != 1 {
		t.Fatalf("event counts = %d, %d, want one", len(closedEvents), len(openEvents))
	}

	for bandIndex, wanted := range wantedReduction {
		closedPressure := math.Abs(closedEvents[0].Amplitude * closedEvents[0].BandGain[bandIndex])
		openPressure := math.Abs(openEvents[0].Amplitude * openEvents[0].BandGain[bandIndex])
		sourceLevel := 20 * math.Log10(openPressure)
		receiverLevel := 20 * math.Log10(closedPressure)
		got := metrics.ApparentSoundReductionIndex(sourceLevel, receiverLevel, 16, 16)

		if errorDB := math.Abs(got - wanted); errorDB > 2.5 {
			t.Errorf("%g Hz apparent reduction = %.3f dB, want %.3f dB within 2.5 dB", centers[bandIndex], got, wanted)
		}
	}
}

func transmissionTestScene(tau float64) *scene.Scene {
	wallMaterials := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	return &scene.Scene{
		Rooms: []scene.Room{
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Width: 4, Depth: 3, Height: 2.5, WallMaterials: wallMaterials}},
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{Origin: geometry.Vec3{X: 4}, Width: 4, Depth: 3, Height: 2.5, WallMaterials: wallMaterials}},
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: 4, Y: 0, Z: 0},
				{X: 4, Y: 3, Z: 0},
				{X: 4, Y: 3, Z: 2.5},
				{X: 4, Y: 0, Z: 2.5},
			},
			Material: "portal",
			State:    scene.PortalClosed,
		}},
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{tau}},
		},
		Sources: []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: 7, Y: 1.5, Z: 1.25},
			Type:     scene.ReceiverBinaural,
			HRTF:     hrtf.NoopDataset{SampleRateHz: 8000},
		}},
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{500},
			LowerEdges:  []float64{350},
			UpperEdges:  []float64{700},
		},
		SampleRate: 8000,
	}
}

func transmissionTestRenderConfig(sc *scene.Scene) ir.RenderConfig {
	return ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.08, BandSpec: sc.BandSpec}
}

func transmissionValidationScene(
	bandSpec acoustics.BandSpec,
	reduction []float64,
	state scene.PortalState,
) *scene.Scene {
	const (
		roomWidth              = 5.625
		roomDepth              = 4.0
		roomHeight             = 4.0
		partitionArea          = roomDepth * roomHeight
		receiverAbsorptionArea = 16.0
	)
	wallArea := 2 * (roomWidth*roomDepth + roomWidth*roomHeight + partitionArea)
	wallAbsorption := receiverAbsorptionArea / wallArea
	walls := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	return &scene.Scene{
		Rooms: []scene.Room{
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
				Width: roomWidth, Depth: roomDepth, Height: roomHeight, WallMaterials: walls,
			}},
			{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
				Origin: geometry.Vec3{X: roomWidth},
				Width:  roomWidth, Depth: roomDepth, Height: roomHeight, WallMaterials: walls,
			}},
		},
		Portals: []scene.Portal{{
			RoomIndices: [2]int{0, 1},
			Polygon: []geometry.Vec3{
				{X: roomWidth, Y: 0, Z: 0},
				{X: roomWidth, Y: roomDepth, Z: 0},
				{X: roomWidth, Y: roomDepth, Z: roomHeight},
				{X: roomWidth, Y: 0, Z: roomHeight},
			},
			Material: "partition",
			State:    state,
		}},
		Materials: map[string]scene.Material{
			"wall": {
				Name:             "wall",
				AbsorptionByBand: []float64{wallAbsorption},
			},
			"partition": {
				Name:                "partition",
				AbsorptionByBand:    []float64{0},
				SoundReductionIndex: append([]float64(nil), reduction...),
			},
		},
		Sources: []scene.Source{{Position: geometry.Vec3{
			X: roomWidth / 2, Y: roomDepth / 2, Z: roomHeight / 2,
		}}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: roomWidth * 1.5, Y: roomDepth / 2, Z: roomHeight / 2},
			Type:     scene.ReceiverOmni,
		}},
		BandSpec:   bandSpec,
		SampleRate: 48000,
	}
}

func thirdOctaveBandSpec(centers []float64) acoustics.BandSpec {
	lower := make([]float64, len(centers))
	upper := make([]float64, len(centers))
	halfBandRatio := math.Pow(2, 1.0/6.0)

	for index, center := range centers {
		lower[index] = center / halfBandRatio
		upper[index] = center * halfBandRatio
	}

	return acoustics.BandSpec{
		CenterFreqs: append([]float64(nil), centers...),
		LowerEdges:  lower,
		UpperEdges:  upper,
	}
}

func hasNonZeroSample(samples []float64) bool {
	for _, sample := range samples {
		if sample != 0 {
			return true
		}
	}

	return false
}

func shiftedTransmissionScene(sc *scene.Scene, offset geometry.Vec3) *scene.Scene {
	shifted := *sc
	shifted.Rooms = append([]scene.Room(nil), sc.Rooms...)

	for index := range shifted.Rooms {
		if shifted.Rooms[index].Shoebox == nil {
			continue
		}

		shoebox := *shifted.Rooms[index].Shoebox
		shoebox.Origin = shoebox.Origin.Add(offset)
		shifted.Rooms[index].Shoebox = &shoebox
	}

	shifted.Portals = append([]scene.Portal(nil), sc.Portals...)
	for portalIndex := range shifted.Portals {
		shifted.Portals[portalIndex].Polygon = append([]geometry.Vec3(nil), shifted.Portals[portalIndex].Polygon...)
		for vertexIndex := range shifted.Portals[portalIndex].Polygon {
			shifted.Portals[portalIndex].Polygon[vertexIndex] = shifted.Portals[portalIndex].Polygon[vertexIndex].Add(offset)
		}
	}

	shifted.Sources = append([]scene.Source(nil), sc.Sources...)
	for index := range shifted.Sources {
		shifted.Sources[index].Position = shifted.Sources[index].Position.Add(offset)
	}

	shifted.Receivers = append([]scene.Receiver(nil), sc.Receivers...)
	for index := range shifted.Receivers {
		shifted.Receivers[index].Position = shifted.Receivers[index].Position.Add(offset)
	}

	return &shifted
}
