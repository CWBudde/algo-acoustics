package algoacoustics

import (
	"math"
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// threeRoomShortcutScene is a chain of three rooms whose end rooms are also
// joined directly, so the source and receiver are adjacent while a longer
// flanking path runs through the middle room.
func threeRoomShortcutScene() *scene.Scene {
	walls := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	room := func(originX float64) scene.Room {
		return scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
			Origin: geometry.Vec3{X: originX}, Width: 4, Depth: 3, Height: 2.5, WallMaterials: walls,
		}}
	}

	return &scene.Scene{
		Rooms: []scene.Room{room(0), room(4), room(8)},
		Portals: []scene.Portal{
			{
				RoomIndices: [2]int{0, 1},
				Polygon: []geometry.Vec3{
					{X: 4, Y: 0, Z: 0}, {X: 4, Y: 3, Z: 0}, {X: 4, Y: 3, Z: 2.5}, {X: 4, Y: 0, Z: 2.5},
				},
				Material: "portal",
				State:    scene.PortalClosed,
			},
			{
				RoomIndices: [2]int{1, 2},
				Polygon: []geometry.Vec3{
					{X: 8, Y: 0, Z: 0}, {X: 8, Y: 3, Z: 0}, {X: 8, Y: 3, Z: 2.5}, {X: 8, Y: 0, Z: 2.5},
				},
				Material: "portal",
				State:    scene.PortalClosed,
			},
		},
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{0.25}},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 11, Y: 1.5, Z: 1.25}, Type: scene.ReceiverOmni}},
		BandSpec:   transmissionTestScene(0.25).BandSpec,
		SampleRate: 8000,
	}
}

// TestCrossRoomEngineRejectsTheFastPathAboveTwoRooms pins that the Phase 21
// one-hop renderer is only chosen for the shape it can actually handle. It
// collects only the portals joining the source and receiver rooms, so a third
// room would have its flanking paths dropped without a trace.
func TestCrossRoomEngineRejectsTheFastPathAboveTwoRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scene *scene.Scene
		want  bool
	}{
		{name: "two adjacent rooms", scene: transmissionTestScene(0.25), want: true},
		{name: "three rooms in a chain", scene: threeRoomShortcutScene(), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := sceneMatchesOneHopTransmission(test.scene); got != test.want {
				t.Fatalf("sceneMatchesOneHopTransmission = %v, want %v", got, test.want)
			}

			_, isNetwork := NewCrossRoomEngine(test.scene, CrossRoomEngineConfig{}).(*NetworkRenderer)
			if isNetwork == test.want {
				t.Fatalf("NewCrossRoomEngine returned network = %v, want %v", isNetwork, !test.want)
			}
		})
	}
}

// TestCrossRoomEngineAddingARoomKeepsTheDirectPathOnTheNetwork pins the case
// the reviewers raised: a third room does not remove the direct portal, so the
// old predicate still accepted the scene and lost the flanking path.
func TestCrossRoomEngineAddingARoomKeepsTheDirectPathOnTheNetwork(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)

	// A third room, joined to the receiver's room, leaves the direct portal
	// between source and receiver rooms entirely intact.
	sc.Rooms = append(sc.Rooms, scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
		Origin: geometry.Vec3{X: 8}, Width: 4, Depth: 3, Height: 2.5,
		WallMaterials: [6]string{"wall", "wall", "wall", "wall", "wall", "wall"},
	}})
	sc.Portals = append(sc.Portals, scene.Portal{
		RoomIndices: [2]int{1, 2},
		Polygon: []geometry.Vec3{
			{X: 8, Y: 0, Z: 0}, {X: 8, Y: 3, Z: 0}, {X: 8, Y: 3, Z: 2.5}, {X: 8, Y: 0, Z: 2.5},
		},
		Material: "portal",
		State:    scene.PortalClosed,
	})

	if sceneMatchesOneHopTransmission(sc) {
		t.Fatal("a three-room scene still took the one-hop fast path")
	}
}

// TestNetworkRendererRejectsNilScene pins that the exported renderer fails
// predictably on invalid library input rather than panicking, as the other
// renderer entry points do.
func TestNetworkRendererRejectsNilScene(t *testing.T) {
	t.Parallel()

	_, err := NewNetworkRenderer(networkTestConfig()).SolveEarly(nil, ir.RenderConfig{})
	if err == nil {
		t.Fatal("SolveEarly accepted a nil scene")
	}

	if !strings.Contains(err.Error(), "scene is nil") {
		t.Fatalf("error = %q, want it to name the nil scene", err)
	}
}

// TestNetworkRendererReportsPathSearchTruncation pins that a search cut short
// by its own limits reaches the caller. A truncated render still looks entirely
// plausible, so this is the only symptom there is.
func TestNetworkRendererReportsPathSearchTruncation(t *testing.T) {
	t.Parallel()

	sc := officeFloorRenderScene(t, 1)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	var reported []NetworkTruncation

	network := NewNetworkRenderer(NetworkRendererConfig{
		ISM:         ism.ISMConfig{MaxOrder: 0},
		BandFloorDB: -90,
		// One hop reaches the neighbouring office, so the deeper branches are
		// abandoned at the depth limit and the tree is not exhaustive.
		MaxPathHops:  1,
		OnTruncation: func(truncation NetworkTruncation) { reported = append(reported, truncation) },
	})

	_, err := network.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("SolveEarly: %v", err)
	}

	if len(reported) != 1 {
		t.Fatalf("OnTruncation fired %d times, want 1", len(reported))
	}

	if !reported[0].PathSearch {
		t.Fatalf("truncation = %+v, want the path search flagged", reported[0])
	}

	if !strings.Contains(reported[0].String(), "not exhaustive") {
		t.Fatalf("truncation message = %q, want it to say the render is not exhaustive", reported[0].String())
	}
}

// TestNetworkRendererReportsDroppedComposedEvents pins that the per-path event
// cap is reported rather than silently thinning a rendered early field.
func TestNetworkRendererReportsDroppedComposedEvents(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	var reported NetworkTruncation

	network := NewNetworkRenderer(NetworkRendererConfig{
		ISM:                      ism.ISMConfig{MaxOrder: 2},
		MaxComposedEventsPerPath: 4,
		OnTruncation:             func(truncation NetworkTruncation) { reported = truncation },
	})

	events, err := network.SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("SolveEarly: %v", err)
	}

	if len(events) > 4 {
		t.Fatalf("the cap kept %d events, want at most 4", len(events))
	}

	if reported.EventsDropped == 0 {
		t.Fatalf("truncation = %+v, want the dropped events counted", reported)
	}
}

// TestNetworkRendererUncappedCompositionKeepsEveryEvent pins that a negative
// cap really removes the count limit, leaving only the relative level floor.
func TestNetworkRendererUncappedCompositionKeepsEveryEvent(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	capped, err := NewNetworkRenderer(NetworkRendererConfig{
		ISM:                      ism.ISMConfig{MaxOrder: 2},
		MaxComposedEventsPerPath: 4,
	}).SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("capped SolveEarly: %v", err)
	}

	uncapped, err := NewNetworkRenderer(NetworkRendererConfig{
		ISM:                      ism.ISMConfig{MaxOrder: 2},
		MaxComposedEventsPerPath: -1,
	}).SolveEarly(sc, cfg)
	if err != nil {
		t.Fatalf("uncapped SolveEarly: %v", err)
	}

	if len(uncapped) <= len(capped) {
		t.Fatalf("uncapped produced %d events, capped %d; want strictly more", len(uncapped), len(capped))
	}
}

// TestNetworkRendererSharesFactorsAcrossPaths pins that hops are memoised by
// their endpoints rather than by path, so a room group two paths both cross
// between the same portals is simulated once.
func TestNetworkRendererSharesFactorsAcrossPaths(t *testing.T) {
	t.Parallel()

	sc := twinPortalScene()

	network := NewNetworkRenderer(NetworkRendererConfig{
		ISM:         ism.ISMConfig{MaxOrder: 0},
		BandFloorDB: -90,
	})

	plan, err := network.prepare(sc)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	if len(plan.paths) < 2 {
		t.Fatalf("the fixture produced %d paths, want at least 2", len(plan.paths))
	}

	hops := 0

	for pathIndex := range plan.paths {
		_, err = network.renderPathFactors(plan, pathIndex, cfg, factorNeeds{early: true})
		if err != nil {
			t.Fatalf("renderPathFactors: %v", err)
		}

		hops += len(plan.paths[pathIndex].groups)
	}

	// Both paths cross the receiver's room through the same portal, so that hop
	// is one cache entry rather than one per path.
	if len(plan.factors) >= hops {
		t.Fatalf("cached %d factors for %d hops across %d paths, want the shared hop simulated once",
			len(plan.factors), hops, len(plan.paths))
	}
}

// twinPortalScene joins the first two rooms by two separate portals, so the two
// propagation paths differ only in their first hop and share the terminal one.
func twinPortalScene() *scene.Scene {
	walls := [6]string{"wall", "wall", "wall", "wall", "wall", "wall"}

	room := func(originX float64) scene.Room {
		return scene.Room{Kind: scene.RoomKindShoebox, Shoebox: &scene.Shoebox{
			Origin: geometry.Vec3{X: originX}, Width: 4, Depth: 3, Height: 2.5, WallMaterials: walls,
		}}
	}

	portal := func(x, yLow, yHigh float64, rooms [2]int) scene.Portal {
		return scene.Portal{
			RoomIndices: rooms,
			Polygon: []geometry.Vec3{
				{X: x, Y: yLow, Z: 0.2},
				{X: x, Y: yHigh, Z: 0.2},
				{X: x, Y: yHigh, Z: 2.2},
				{X: x, Y: yLow, Z: 2.2},
			},
			Material: "portal",
			State:    scene.PortalClosed,
		}
	}

	return &scene.Scene{
		Rooms: []scene.Room{room(0), room(4), room(8)},
		Portals: []scene.Portal{
			portal(4, 0.2, 1.2, [2]int{0, 1}),
			portal(4, 1.8, 2.8, [2]int{0, 1}),
			portal(8, 1.0, 2.0, [2]int{1, 2}),
		},
		Materials: map[string]scene.Material{
			"wall":   {Name: "wall", AbsorptionByBand: []float64{0.1}},
			"portal": {Name: "portal", AbsorptionByBand: []float64{0}, TransmissionByBand: []float64{0.5}},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 11, Y: 1.5, Z: 1.25}, Type: scene.ReceiverOmni}},
		BandSpec:   transmissionTestScene(0.5).BandSpec,
		SampleRate: 8000,
	}
}

// TestNetworkRendererLateOnlySkipsTheEarlySolve pins that RenderLateMono does
// not pay for the image-source field it was never asked for.
func TestNetworkRendererLateOnlySkipsTheEarlySolve(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	network := NewNetworkRenderer(networkTestConfig())

	plan, err := network.prepare(sc)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	_, err = network.renderLatePaths(plan, cfg)
	if err != nil {
		t.Fatalf("renderLatePaths: %v", err)
	}

	for key, cached := range plan.factors {
		if cached.factor.Early != nil || cached.factor.Events != nil {
			t.Fatalf("hop %+v solved its early field for a late-only render", key)
		}

		if cached.factor.LateEnergy == nil {
			t.Fatalf("hop %+v has no late field", key)
		}
	}
}

// TestNetworkRendererKeepsEarlyWorkWhenTheLateFieldIsAddedLater pins that a hop
// already carrying its early field only adds the missing ray trace instead of
// solving the image-source field a second time.
func TestNetworkRendererKeepsEarlyWorkWhenTheLateFieldIsAddedLater(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	cfg := transmissionTestRenderConfig(sc)

	network := NewNetworkRenderer(networkTestConfig())

	plan, err := network.prepare(sc)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	early, err := network.renderPathFactors(plan, 0, cfg, factorNeeds{early: true})
	if err != nil {
		t.Fatalf("early renderPathFactors: %v", err)
	}

	both, err := network.renderPathFactors(plan, 0, cfg, factorNeeds{early: true, late: true})
	if err != nil {
		t.Fatalf("combined renderPathFactors: %v", err)
	}

	for index := range early {
		if early[index] != both[index] {
			t.Fatalf("hop %d was solved into a new factor instead of being filled in place", index)
		}

		if both[index].LateEnergy == nil {
			t.Fatalf("hop %d did not gain its late field", index)
		}
	}
}

// TestNetworkRendererCombinesDirectionsFromEveryPath pins that the arrival
// directions of the late field come from every path, weighted by the energy it
// delivers, instead of from whichever path happened to be ranked first.
func TestNetworkRendererCombinesDirectionsFromEveryPath(t *testing.T) {
	t.Parallel()

	// One path arrives entirely through the first direction, the other entirely
	// through the second, and both deliver the same energy.
	first := []*raytrace.EnergyHistogram{directionalTestHistogram(4), directionalTestHistogram(0)}
	second := []*raytrace.EnergyHistogram{directionalTestHistogram(0), directionalTestHistogram(4)}

	fromFirstAlone := raytrace.DGProbabilitiesFromHistograms(first)
	if math.Abs(fromFirstAlone[0][0]-1) > 1e-12 {
		t.Fatalf("one path alone gives probability %v for its own direction, want 1", fromFirstAlone[0][0])
	}

	combined, err := accumulateDirectional(first, second)
	if err != nil {
		t.Fatalf("accumulateDirectional: %v", err)
	}

	probabilities := raytrace.DGProbabilitiesFromHistograms(combined)
	if len(probabilities) != 2 {
		t.Fatalf("got %d directivity groups, want 2", len(probabilities))
	}

	for group, probability := range probabilities {
		if math.Abs(probability[0]-0.5) > 1e-12 {
			t.Fatalf("group %d probability = %v, want 0.5 once both paths contribute", group, probability[0])
		}
	}
}

// directionalTestHistogram builds a one-bin histogram holding a single energy.
func directionalTestHistogram(energy float64) *raytrace.EnergyHistogram {
	histogram := raytrace.NewEnergyHistogram(0.01, 0.01, 1)
	histogram.Bins[0].BandEnergy[0] = energy

	return histogram
}
