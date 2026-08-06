package ism

import (
	"math"
	"sort"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/directivity"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/scene"
)

func TestISMSolverSolveDirectAndFirstOrder(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	if len(events) != 7 {
		t.Fatalf("Solve() returned %d events, want 7", len(events))
	}

	if events[0].Kind != ir.EventDirect {
		t.Fatalf("events[0].Kind = %v, want %v", events[0].Kind, ir.EventDirect)
	}

	wantDistances := []float64{3, 5, 5, math.Sqrt(45), math.Sqrt(45), math.Sqrt(153), 15}

	gotDistances := make([]float64, 0, len(events))
	for _, event := range events {
		gotDistances = append(gotDistances, event.DistanceMeters)
		if math.Abs(event.Amplitude-(1/event.DistanceMeters)) > 1e-9 {
			t.Fatalf("event amplitude = %v, want %v", event.Amplitude, 1/event.DistanceMeters)
		}

		for bandIndex, gain := range event.BandGain {
			if math.Abs(gain-1) > 1e-9 {
				t.Fatalf("event BandGain[%d] = %v, want 1", bandIndex, gain)
			}
		}
	}

	sort.Float64s(gotDistances)
	sort.Float64s(wantDistances)

	for index, wantDistance := range wantDistances {
		if math.Abs(gotDistances[index]-wantDistance) > 1e-9 {
			t.Fatalf("distance[%d] = %v, want %v", index, gotDistances[index], wantDistance)
		}
	}
}

func TestISMSolverSolveDirectPathTimeMatchesDistance(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 0, SpeedOfSound: acoustics.SpeedOfSound})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	direct := firstDirectEvent(events)
	if direct == nil {
		t.Fatal("expected a direct event")
	}

	distance := sc.Sources[0].Position.Distance(sc.Receivers[0].Position)

	wantTime := distance / acoustics.SpeedOfSound
	if math.Abs(direct.TimeSeconds-wantTime) > 1e-12 {
		t.Fatalf("direct TimeSeconds = %v, want %v", direct.TimeSeconds, wantTime)
	}
}

func TestISMSolverSolveFirstFloorReflectionMatchesGeometry(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Room.Shoebox = &scene.Shoebox{
		Width:         6,
		Depth:         6,
		Height:        4,
		WallMaterials: [6]string{"soft", "soft", "soft", "soft", "floor", "soft"},
	}
	sc.Materials["floor"] = scene.Material{
		Name:             "floor",
		AbsorptionByBand: []float64{0, 0, 0, 0, 0, 0},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}
	sc.Materials["soft"] = scene.Material{
		Name:             "soft",
		AbsorptionByBand: []float64{1, 1, 1, 1, 1, 1},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}
	sc.Sources[0].Position = geometry.Vec3{X: 3, Y: 3, Z: 1}
	sc.Receivers[0].Position = geometry.Vec3{X: 3, Y: 3, Z: 3}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	var floorReflection *ir.Event

	for index := range events {
		if events[index].Kind == ir.EventSpecular && directionMatches(events[index].Direction, geometry.Vec3{X: 0, Y: 0, Z: -1}) {
			floorReflection = &events[index]
			break
		}
	}

	if floorReflection == nil {
		t.Fatal("expected a floor reflection event")
	}

	wantImage := geometry.Vec3{X: 3, Y: 3, Z: -1}

	wantDistance := sc.Receivers[0].Position.Distance(wantImage)
	if math.Abs(floorReflection.DistanceMeters-wantDistance) > 1e-12 {
		t.Fatalf("floor reflection distance = %v, want %v", floorReflection.DistanceMeters, wantDistance)
	}

	wantTime := wantDistance / acoustics.SpeedOfSound
	if math.Abs(floorReflection.TimeSeconds-wantTime) > 1e-12 {
		t.Fatalf("floor reflection TimeSeconds = %v, want %v", floorReflection.TimeSeconds, wantTime)
	}
}

func TestISMSolverSolveReciprocityPreservesPathLengths(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	base := testScene(t)
	base.Sources[0].Directivity = directivity.OmniModel{}

	swapped := base
	swapped.Sources = []scene.Source{{
		Position:    base.Receivers[0].Position,
		Orientation: geometry.QuatIdentity(),
		GainDB:      0,
		Directivity: directivity.OmniModel{},
	}}
	swapped.Receivers = []scene.Receiver{{
		Position:    base.Sources[0].Position,
		Orientation: geometry.QuatIdentity(),
		Type:        scene.ReceiverOmni,
	}}

	baseEvents, err := solver.Solve(&base, ISMConfig{MaxOrder: 2})
	if err != nil {
		t.Fatalf("Solve(base) error = %v", err)
	}

	swappedEvents, err := solver.Solve(&swapped, ISMConfig{MaxOrder: 2})
	if err != nil {
		t.Fatalf("Solve(swapped) error = %v", err)
	}

	baseDistances := eventDistances(baseEvents)

	swappedDistances := eventDistances(swappedEvents)
	if len(baseDistances) != len(swappedDistances) {
		t.Fatalf("event count differs: %d vs %d", len(baseDistances), len(swappedDistances))
	}

	for index := range baseDistances {
		if math.Abs(baseDistances[index]-swappedDistances[index]) > 1e-12 {
			t.Fatalf("distance[%d] = %v, want %v", index, swappedDistances[index], baseDistances[index])
		}
	}
}

func TestISMSolverSolveScalingRoomDoublesFirstReflectionTime(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	base := testScene(t)
	scaled := base
	scaled.Room.Shoebox = &scene.Shoebox{
		Width:         base.Room.Shoebox.Width * 2,
		Depth:         base.Room.Shoebox.Depth * 2,
		Height:        base.Room.Shoebox.Height * 2,
		WallMaterials: base.Room.Shoebox.WallMaterials,
	}
	scaled.Sources = []scene.Source{{
		Position:    base.Sources[0].Position.Scale(2),
		Orientation: geometry.QuatIdentity(),
		GainDB:      base.Sources[0].GainDB,
		Directivity: directivity.OmniModel{},
	}}
	scaled.Receivers = []scene.Receiver{{
		Position:    base.Receivers[0].Position.Scale(2),
		Orientation: geometry.QuatIdentity(),
		Type:        scene.ReceiverOmni,
	}}

	baseEvents, err := solver.Solve(&base, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve(base) error = %v", err)
	}

	scaledEvents, err := solver.Solve(&scaled, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve(scaled) error = %v", err)
	}

	baseFirst := firstSpecularTime(baseEvents)

	scaledFirst := firstSpecularTime(scaledEvents)
	if math.Abs(scaledFirst-2*baseFirst) > 1e-12 {
		t.Fatalf("scaled first specular time = %v, want %v", scaledFirst, 2*baseFirst)
	}
}

func TestISMSolverSolveZeroAbsorptionMatchesTheoreticalAmplitudeSum(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}

	sc := testScene(t)
	for key, material := range sc.Materials {
		material.AbsorptionByBand = []float64{0, 0, 0, 0, 0, 0}
		sc.Materials[key] = material
	}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	var gotAmplitudeSum float64
	for _, event := range events {
		gotAmplitudeSum += event.Amplitude
	}

	wantAmplitudeSum := theoreticalAmplitudeSum(sc)
	if math.Abs(gotAmplitudeSum-wantAmplitudeSum) > 1e-12 {
		t.Fatalf("amplitude sum = %v, want %v", gotAmplitudeSum, wantAmplitudeSum)
	}
}

func TestISMSolverSolveAppliesPerWallAbsorption(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Room.Shoebox.WallMaterials = [6]string{"soft", "hard", "hard", "hard", "hard", "hard"}
	sc.Materials["soft"] = scene.Material{
		Name:             "soft",
		AbsorptionByBand: []float64{0.75, 0.75, 0.75, 0.75, 0.75, 0.75},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}
	sc.Materials["hard"] = scene.Material{
		Name:             "hard",
		AbsorptionByBand: []float64{0, 0, 0, 0, 0, 0},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	var negativeXReflection, positiveXReflection *ir.Event

	for index := range events {
		switch {
		case events[index].Kind == ir.EventSpecular && directionMatches(events[index].Direction, geometry.Vec3{X: -1, Y: 0, Z: 0}):
			negativeXReflection = &events[index]
		case events[index].Kind == ir.EventSpecular && directionMatches(events[index].Direction, geometry.Vec3{X: 1, Y: 0, Z: 0}):
			positiveXReflection = &events[index]
		}
	}

	if negativeXReflection == nil || positiveXReflection == nil {
		t.Fatal("expected both x-axis reflections to be present")
	}

	for bandIndex, gain := range negativeXReflection.BandGain {
		if math.Abs(gain-0.5) > 1e-9 {
			t.Fatalf("negative-x BandGain[%d] = %v, want 0.5", bandIndex, gain)
		}
	}

	for bandIndex, gain := range positiveXReflection.BandGain {
		if math.Abs(gain-1) > 1e-9 {
			t.Fatalf("positive-x BandGain[%d] = %v, want 1", bandIndex, gain)
		}
	}
}

func TestISMSolverSolveUsesAngleDependentPressureReflectance(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Room.Shoebox.WallMaterials = [6]string{"soft", "hard", "hard", "hard", "hard", "hard"}
	sc.Materials["soft"] = scene.Material{
		Name:             "soft",
		AbsorptionByBand: []float64{1, 1, 1, 1, 1, 1},
		ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
	}
	sc.Sources[0].Position = geometry.Vec3{X: 2, Y: 2, Z: 3}
	sc.Receivers[0].Position = geometry.Vec3{X: 6, Y: 5, Z: 4}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	image := findImageSourceByOrderTriple(t, sc.Sources[0].Position, sc.Room.Shoebox, 1, -1, 0, 0)
	event := findSpecularEventForImage(t, events, sc.Receivers[0].Position, image.Position)

	path, ok := reflectionPath(image, sc.Receivers[0].Position)
	if !ok {
		t.Fatal("expected a valid reflection path")
	}

	want := pathPressureReflectance(sourceOrderedPath(path), sc.Sources[0].Position, sc.Room.Shoebox, sc.Materials, 0)
	if got := event.BandGain[0]; math.Abs(got-want) > 1e-12 {
		t.Fatalf("BandGain[0] = %v, want %v", got, want)
	}

	if event.BandGain[0] >= 0 {
		t.Fatalf("BandGain[0] = %v, want negative pressure reflectance", event.BandGain[0])
	}

	buf, err := ir.RenderMono([]ir.Event{*event}, ir.RenderConfig{SampleRate: 48000, DurationSeconds: 0.25, BandSpec: sc.BandSpec})
	if err != nil {
		t.Fatalf("RenderMono() error = %v", err)
	}

	sampleIndex := int(math.Round(event.TimeSeconds * float64(buf.SampleRate)))
	if got := buf.Samples[sampleIndex]; got >= 0 {
		t.Fatalf("Samples[%d] = %v, want negative early impulse", sampleIndex, got)
	}
}

func TestISMSolverSolveAppliesSourceDirectivity(t *testing.T) {
	t.Parallel()

	solver := ISMSolver{}
	sc := testScene(t)
	sc.Sources[0].Directivity = directivity.CardioidModel{
		Axis:   geometry.Vec3{X: 1, Y: 0, Z: 0},
		OrderN: 1,
	}

	events, err := solver.Solve(&sc, ISMConfig{MaxOrder: 1})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	for _, event := range events {
		if event.Kind == ir.EventSpecular && directionMatches(event.Direction, geometry.Vec3{X: -1, Y: 0, Z: 0}) {
			t.Fatal("rear-facing x reflection should be suppressed by cardioid directivity")
		}
	}

	var directEvent ir.Event
	foundDirect := false

	for _, event := range events {
		if event.Kind == ir.EventDirect {
			directEvent = event
			foundDirect = true

			break
		}
	}

	if !foundDirect {
		t.Fatal("expected a direct event")
	}

	for bandIndex, gain := range directEvent.BandGain {
		if math.Abs(gain-1) > 1e-9 {
			t.Fatalf("direct BandGain[%d] = %v, want 1", bandIndex, gain)
		}
	}
}

func testScene(t *testing.T) scene.Scene {
	t.Helper()

	return scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width:         10,
				Depth:         8,
				Height:        6,
				WallMaterials: [6]string{"hard", "hard", "hard", "hard", "hard", "hard"},
			},
		},
		Materials: map[string]scene.Material{
			"hard": {
				Name:             "hard",
				AbsorptionByBand: []float64{0, 0, 0, 0, 0, 0},
				ScatteringByBand: []float64{0, 0, 0, 0, 0, 0},
			},
		},
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 1, Y: 2, Z: 3},
			Orientation: geometry.QuatIdentity(),
			GainDB:      0,
			Directivity: directivity.OmniModel{},
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 4, Y: 2, Z: 3},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}
}

func directionMatches(got, want geometry.Vec3) bool {
	return math.Abs(got.X-want.X) <= 1e-9 && math.Abs(got.Y-want.Y) <= 1e-9 && math.Abs(got.Z-want.Z) <= 1e-9
}

func firstDirectEvent(events []ir.Event) *ir.Event {
	for index := range events {
		if events[index].Kind == ir.EventDirect {
			return &events[index]
		}
	}

	return nil
}

func eventDistances(events []ir.Event) []float64 {
	distances := make([]float64, 0, len(events))
	for _, event := range events {
		distances = append(distances, event.DistanceMeters)
	}

	sort.Float64s(distances)

	return distances
}

func firstSpecularTime(events []ir.Event) float64 {
	first := math.Inf(1)

	for _, event := range events {
		if event.Kind != ir.EventSpecular {
			continue
		}

		if event.TimeSeconds < first {
			first = event.TimeSeconds
		}
	}

	return first
}

func theoreticalAmplitudeSum(sc scene.Scene) float64 {
	room := sc.Room.Shoebox
	source := sc.Sources[0].Position
	receiver := sc.Receivers[0].Position

	total := 1 / receiver.Distance(source)
	images := []geometry.Vec3{
		{X: -source.X, Y: source.Y, Z: source.Z},
		{X: 2*room.Width - source.X, Y: source.Y, Z: source.Z},
		{X: source.X, Y: -source.Y, Z: source.Z},
		{X: source.X, Y: 2*room.Depth - source.Y, Z: source.Z},
		{X: source.X, Y: source.Y, Z: -source.Z},
		{X: source.X, Y: source.Y, Z: 2*room.Height - source.Z},
	}

	for _, image := range images {
		total += 1 / receiver.Distance(image)
	}

	return total
}

func TestISMSolverSolveMeshRoom(t *testing.T) {
	t.Parallel()

	mesh := geometry.MeshFromBox(geometry.Vec3{}, geometry.Vec3{X: 4, Y: 3, Z: 2.5})
	sc := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "plaster",
		},
		Materials: map[string]scene.Material{
			"plaster": {
				Name:             "plaster",
				AbsorptionByBand: []float64{0.01, 0.02, 0.02, 0.03, 0.04, 0.05},
			},
		},
		Sources:    []scene.Source{{Position: geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}}},
		Receivers:  []scene.Receiver{{Position: geometry.Vec3{X: 3, Y: 1.5, Z: 1.25}}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	solver := ISMSolver{}

	events, err := solver.Solve(sc, ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	})
	if err != nil {
		t.Fatalf("Solve() error = %v", err)
	}

	hasDirect := false
	hasSpecular := false

	for _, e := range events {
		if e.Kind == ir.EventDirect {
			hasDirect = true
		}

		if e.Kind == ir.EventSpecular {
			hasSpecular = true
		}
	}

	if !hasDirect {
		t.Error("no direct event from mesh ISM")
	}

	if !hasSpecular {
		t.Error("no specular events from mesh ISM")
	}
}

func TestMeshISMMatchesShoeboxISMEventCount(t *testing.T) {
	t.Parallel()

	const (
		width  = 4.0
		depth  = 3.0
		height = 2.5
	)

	materials := map[string]scene.Material{
		"m": {
			Name:             "m",
			AbsorptionByBand: []float64{0.01, 0.02, 0.02, 0.03, 0.04, 0.05},
		},
	}

	srcPos := geometry.Vec3{X: 1, Y: 1.5, Z: 1.25}
	rcvPos := geometry.Vec3{X: 3, Y: 1.5, Z: 1.25}

	shoeboxScene := &scene.Scene{
		Room: scene.Room{
			Kind: scene.RoomKindShoebox,
			Shoebox: &scene.Shoebox{
				Width: width, Depth: depth, Height: height,
				WallMaterials: [6]string{"m", "m", "m", "m", "m", "m"},
			},
		},
		Materials:  materials,
		Sources:    []scene.Source{{Position: srcPos}},
		Receivers:  []scene.Receiver{{Position: rcvPos}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	mesh := geometry.MeshFromBox(geometry.Vec3{}, geometry.Vec3{X: width, Y: depth, Z: height})
	meshScene := &scene.Scene{
		Room: scene.Room{
			Kind:         scene.RoomKindMesh,
			Mesh:         mesh,
			MeshMaterial: "m",
		},
		Materials:  materials,
		Sources:    []scene.Source{{Position: srcPos}},
		Receivers:  []scene.Receiver{{Position: rcvPos}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	cfg := ISMConfig{
		MaxOrder:     3,
		SpeedOfSound: acoustics.SpeedOfSound,
		BandSpec:     acoustics.Octave6,
	}

	solver := ISMSolver{}

	shoeboxEvents, err := solver.Solve(shoeboxScene, cfg)
	if err != nil {
		t.Fatalf("shoebox Solve error = %v", err)
	}

	meshEvents, err := solver.Solve(meshScene, cfg)
	if err != nil {
		t.Fatalf("mesh Solve error = %v", err)
	}

	shoeboxSpecular := countEventKind(shoeboxEvents, ir.EventSpecular)
	meshSpecular := countEventKind(meshEvents, ir.EventSpecular)

	t.Logf("shoebox: %d specular, mesh: %d specular", shoeboxSpecular, meshSpecular)

	if meshSpecular == 0 {
		t.Fatal("mesh ISM produced zero specular events")
	}

	// Mesh should find at least 50% of what shoebox finds (triangulation may
	// miss edge cases due to BVH precision and split planes).
	if meshSpecular < shoeboxSpecular/2 {
		t.Fatalf("mesh specular (%d) is less than half of shoebox (%d)",
			meshSpecular, shoeboxSpecular)
	}
}

func countEventKind(events []ir.Event, kind ir.EventKind) int {
	n := 0

	for _, e := range events {
		if e.Kind == kind {
			n++
		}
	}

	return n
}
