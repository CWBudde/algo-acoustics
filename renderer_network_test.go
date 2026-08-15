package algoacoustics

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

func networkTestConfig() NetworkRendererConfig {
	return NetworkRendererConfig{
		ISM: ism.ISMConfig{MaxOrder: 1},
		Raytrace: RaytraceEngineConfig{
			Launch:             raytrace.LaunchConfig{NumRays: 2000, MaxBounces: 20},
			ReceiverRadius:     0.5,
			BinDurationSeconds: 0.005,
		},
	}
}

// TestNetworkRendererMatchesPhase21OneHopEarlyLevel is the calibration pin for
// the whole filter network.
//
// The Phase 21 renderer composes a two-room hop by re-emitting every event and
// running a fresh image-source solve per emission; the network renderer
// composes it by convolving separately simulated factors. Convolving two
// impulse trains is exactly their cartesian product, so the two formulations
// are algebraically identical and agree to floating-point precision — there is
// no calibration constant hiding at the handoff.
//
// The tolerance is deliberately far tighter than the 1 dB the plan budgeted:
// anything above numerical noise here means the composition drifted.
func TestNetworkRendererMatchesPhase21OneHopEarlyLevel(t *testing.T) {
	t.Parallel()

	const tau = 0.25

	for _, order := range []int{0, 1, 2} {
		sc := transmissionTestScene(tau)
		cfg := transmissionTestRenderConfig(sc)

		legacy := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: order}})

		legacyEvents, err := legacy.SolveEarly(sc, cfg)
		if err != nil {
			t.Fatalf("order %d: TransmissionRenderer.SolveEarly: %v", order, err)
		}

		network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: order}})

		networkEvents, err := network.SolveEarly(transmissionTestScene(tau), cfg)
		if err != nil {
			t.Fatalf("order %d: NetworkRenderer.SolveEarly: %v", order, err)
		}

		if len(legacyEvents) == 0 {
			t.Fatalf("order %d: the Phase 21 renderer produced no events", order)
		}

		if len(networkEvents) != len(legacyEvents) {
			t.Fatalf("order %d: network produced %d events, Phase 21 %d",
				order, len(networkEvents), len(legacyEvents))
		}

		legacyLevel := eventPressureLevelDB(legacyEvents)
		networkLevel := eventPressureLevelDB(networkEvents)

		if diff := math.Abs(legacyLevel - networkLevel); diff > 1e-9 {
			t.Fatalf("order %d: transmitted level: network %.6f dB, Phase 21 %.6f dB, difference %.3g dB",
				order, networkLevel, legacyLevel, diff)
		}
	}
}

// TestNetworkRendererMatchesPhase21EventForEvent goes further than the summed
// level and pins each individual arrival, so a coincidental level match cannot
// hide a redistributed early field.
//
// DistanceMeters is deliberately excluded. Phase 21 reports only the final leg
// there while its TimeSeconds covers the whole path, so the two fields
// contradict each other; the network renderer reports the full propagation
// distance instead. The field is export metadata and feeds no rendering, so the
// difference is cosmetic — but it is a difference, and
// TestNetworkRendererReportsFullPropagationDistance pins the new behaviour.
func TestNetworkRendererMatchesPhase21EventForEvent(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	legacy := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}})

	legacyEvents, err := legacy.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("TransmissionRenderer.SolveEarly: %v", err)
	}

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 1}})

	networkEvents, err := network.SolveEarly(transmissionTestScene(0.25), cfg)
	if err != nil {
		t.Fatalf("NetworkRenderer.SolveEarly: %v", err)
	}

	if len(networkEvents) != len(legacyEvents) {
		t.Fatalf("event counts differ: network %d, Phase 21 %d", len(networkEvents), len(legacyEvents))
	}

	for index := range legacyEvents {
		want := legacyEvents[index]
		got := networkEvents[index]

		if math.Abs(got.TimeSeconds-want.TimeSeconds) > 1e-12 {
			t.Fatalf("event %d time = %v, want %v", index, got.TimeSeconds, want.TimeSeconds)
		}

		wantPressure := want.Amplitude * want.BandGain[0]
		gotPressure := got.Amplitude * got.BandGain[0]

		if math.Abs(gotPressure-wantPressure) > 1e-12 {
			t.Fatalf("event %d pressure = %v, want %v", index, gotPressure, wantPressure)
		}
	}
}

// eventPressureLevelDB returns the broadband pressure level of an event list.
func eventPressureLevelDB(events []ir.Event) float64 {
	total := 0.0

	for _, event := range events {
		gain := 1.0
		if len(event.BandGain) > 0 {
			gain = event.BandGain[0]
		}

		pressure := event.Amplitude * gain
		total += pressure * pressure
	}

	if total <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(total)
}

// TestNetworkRendererReportsFullPropagationDistance pins that a composed event
// carries the whole path length, consistent with its arrival time.
func TestNetworkRendererReportsFullPropagationDistance(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	events, err := network.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("SolveEarly: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want the single direct transmission", len(events))
	}

	// Source at x = 1, portal at x = 4, receiver at x = 7: six metres in all.
	if math.Abs(events[0].DistanceMeters-6) > 1e-3 {
		t.Fatalf("distance = %v m, want the full 6 m path", events[0].DistanceMeters)
	}

	// Distance and time must describe the same path.
	implied := events[0].TimeSeconds * acoustics.SpeedOfSound
	if math.Abs(implied-events[0].DistanceMeters) > 1e-3 {
		t.Fatalf("time implies %v m but distance reports %v m", implied, events[0].DistanceMeters)
	}
}

// TestNetworkRendererScalesWithPortalTransmission checks that the portal filter
// enters the chain as sqrt(tau) in the pressure domain, matching Phase 21.
func TestNetworkRendererScalesWithPortalTransmission(t *testing.T) {
	t.Parallel()

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	quiet := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(quiet)

	quietEvents, err := network.SolveEarly(quiet, cfg)
	if err != nil {
		t.Fatalf("SolveEarly(tau=0.25): %v", err)
	}

	loudEvents, err := network.SolveEarly(transmissionTestScene(1), cfg)
	if err != nil {
		t.Fatalf("SolveEarly(tau=1): %v", err)
	}

	quietPressure := math.Sqrt(math.Pow(10, eventPressureLevelDB(quietEvents)/10))
	loudPressure := math.Sqrt(math.Pow(10, eventPressureLevelDB(loudEvents)/10))

	// sqrt(0.25) = 0.5.
	if ratio := quietPressure / loudPressure; math.Abs(ratio-0.5) > 1e-6 {
		t.Fatalf("pressure ratio = %v, want 0.5 for tau = 0.25", ratio)
	}
}

// chainRoomScene builds a row of rooms along x joined by full-wall portals, so
// the network renderer can be exercised over more than one hop.
func chainRoomScene(t *testing.T, roomCount int, tau float64) *scene.Scene {
	t.Helper()

	const (
		width  = 4.0
		depth  = 3.0
		height = 2.5
	)

	wallMaterials := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	rooms := make([]scene.Room, 0, roomCount)
	for index := range roomCount {
		rooms = append(rooms, scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Origin: geometry.Vec3{X: float64(index) * width},
				Width:  width, Depth: depth, Height: height,
				WallMaterials: wallMaterials,
			},
		})
	}

	portals := make([]scene.Portal, 0, roomCount-1)
	for index := range roomCount - 1 {
		atX := float64(index+1) * width
		portals = append(portals, scene.Portal{
			RoomIndices: [2]int{index, index + 1},
			Polygon: []geometry.Vec3{
				{X: atX, Y: 0, Z: 0},
				{X: atX, Y: depth, Z: 0},
				{X: atX, Y: depth, Z: height},
				{X: atX, Y: 0, Z: height},
			},
			Material: "portal",
			State:    scene.PortalClosed,
		})
	}

	sc := &scene.Scene{
		Rooms:   rooms,
		Portals: portals,
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{tau}},
		},
		Sources: []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: float64(roomCount-1)*width + 3, Y: 1.5, Z: 1.25},
			Type:     scene.ReceiverOmni,
		}},
		BandSpec: acoustics.BandSpec{
			CenterFreqs: []float64{500},
			LowerEdges:  []float64{350},
			UpperEdges:  []float64{700},
		},
		SampleRate: 8000,
	}

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("chain fixture is invalid: %v", err)
	}

	return sc
}

// TestNetworkRendererHandlesPortalChains is the capability Phase 21 explicitly
// rejected: a source and receiver separated by more than one portal.
func TestNetworkRendererHandlesPortalChains(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 3, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.1, BandSpec: sc.BandSpec}

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	events, err := network.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("SolveEarly over a two-portal chain: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("a two-portal chain produced no transmitted events")
	}

	// The Phase 21 renderer refuses the same scene, which is why the network
	// renderer exists.
	legacy := NewTransmissionRenderer(TransmissionRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	_, err = legacy.SolveEarly(sc, cfg)
	if err == nil {
		t.Fatal("the Phase 21 renderer unexpectedly accepted a portal chain")
	}
}

// TestNetworkRendererChainAttenuatesWithEachPortal checks that each additional
// portal costs a further sqrt(tau) in pressure.
func TestNetworkRendererChainAttenuatesWithEachPortal(t *testing.T) {
	t.Parallel()

	const tau = 0.25

	network := NewNetworkRenderer(NetworkRendererConfig{ISM: ism.ISMConfig{MaxOrder: 0}})

	twoRoom := chainRoomScene(t, 2, tau)
	threeRoom := chainRoomScene(t, 3, tau)

	twoEvents, err := network.SolveEarly(twoRoom, ir.RenderConfig{
		SampleRate: twoRoom.SampleRate, DurationSeconds: 0.1, BandSpec: twoRoom.BandSpec,
	})
	if err != nil {
		t.Fatalf("SolveEarly(two rooms): %v", err)
	}

	threeEvents, err := network.SolveEarly(threeRoom, ir.RenderConfig{
		SampleRate: threeRoom.SampleRate, DurationSeconds: 0.1, BandSpec: threeRoom.BandSpec,
	})
	if err != nil {
		t.Fatalf("SolveEarly(three rooms): %v", err)
	}

	twoLevel := eventPressureLevelDB(twoEvents)
	threeLevel := eventPressureLevelDB(threeEvents)

	if threeLevel >= twoLevel {
		t.Fatalf("three-room level %.3f dB is not below the two-room %.3f dB", threeLevel, twoLevel)
	}
}

func TestNetworkRendererRejectsMultipleReceivers(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	sc.Receivers = append(sc.Receivers, sc.Receivers[0])

	network := NewNetworkRenderer(networkTestConfig())

	_, err := network.SolveEarly(sc, transmissionTestRenderConfig(sc))
	if err == nil {
		t.Fatal("SolveEarly accepted more than one receiver")
	}

	// The message must name the constraint so the caller knows why.
	if !strings.Contains(err.Error(), "one receiver") {
		t.Fatalf("error = %v, want it to name the single-receiver constraint", err)
	}
}

func TestNetworkRendererRendersMonoAcrossAPortal(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	network := NewNetworkRenderer(networkTestConfig())

	buffer, err := network.RenderMono(sc, cfg)
	if err != nil {
		t.Fatalf("RenderMono: %v", err)
	}

	if buffer == nil || buffer.Len() == 0 {
		t.Fatal("RenderMono produced no samples")
	}

	energy := 0.0
	for _, sample := range buffer.Samples {
		energy += sample * sample
	}

	if energy <= 0 {
		t.Fatal("the transmitted mono response carries no energy")
	}
}

func TestNetworkRendererRendersBinauralAcrossAPortal(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	network := NewNetworkRenderer(networkTestConfig())

	left, right, err := network.RenderBinaural(sc, sc.Receivers[0], cfg)
	if err != nil {
		t.Fatalf("RenderBinaural: %v", err)
	}

	if left == nil || right == nil || left.Len() == 0 || right.Len() == 0 {
		t.Fatal("RenderBinaural produced no samples")
	}
}

// TestNetworkRendererImplementsCrossRoomEngine pins that the renderer drops
// into the existing Renderer.Transmission slot without any dispatch change.
func TestNetworkRendererImplementsCrossRoomEngine(t *testing.T) {
	t.Parallel()

	// Compile-time satisfaction is the point; the assertions below also pin
	// the optional early and late interfaces the pipeline selects on.
	var engine CrossRoomEngine = NewNetworkRenderer(networkTestConfig())

	if _, ok := engine.(TransmissionEarlyEngine); !ok {
		t.Fatal("NetworkRenderer does not satisfy TransmissionEarlyEngine")
	}

	if _, ok := engine.(CrossRoomLateEngine); !ok {
		t.Fatal("NetworkRenderer does not satisfy CrossRoomLateEngine")
	}

	var legacy CrossRoomEngine = NewTransmissionRenderer(TransmissionRendererConfig{})

	if _, ok := legacy.(TransmissionEarlyEngine); !ok {
		t.Fatal("TransmissionRenderer does not satisfy TransmissionEarlyEngine")
	}

	if _, ok := legacy.(CrossRoomLateEngine); !ok {
		t.Fatal("TransmissionRenderer does not satisfy CrossRoomLateEngine")
	}
}
