package scene_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// chainScene builds a row of rooms along x, each 4 m x 4 m x 3 m, joined by a
// door between neighbours. The named material governs every door.
//
//	room 0 (x 0..4) | room 1 (x 4..8) | room 2 (x 8..12) ...
func chainScene(t *testing.T, roomCount int, doorMaterial string) *scene.Scene {
	t.Helper()

	const (
		size   = 4.0
		height = 3.0
	)

	rooms := make([]scene.Room, 0, roomCount)
	for index := range roomCount {
		rooms = append(rooms, shoeboxAt(geometry.Vec3{X: float64(index) * size}, size, size, height))
	}

	portals := make([]scene.Portal, 0, roomCount-1)
	for index := range roomCount - 1 {
		portals = append(portals, scene.Portal{
			RoomIndices: [2]int{index, index + 1},
			Polygon:     doorOnX(float64(index+1)*size, 1.4, 2.6, 2.1),
			Material:    doorMaterial,
			State:       scene.PortalClosed,
		})
	}

	sc := &scene.Scene{
		Rooms:     rooms,
		Portals:   portals,
		Materials: pathTestMaterials(),
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: float64(roomCount-1)*size + 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("chain fixture is invalid: %v", err)
	}

	return sc
}

func pathTestMaterials() map[string]scene.Material {
	materials := graphTestMaterials()
	materials["quiet_door"] = scene.Material{
		Name: "quiet_door", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{40},
	}
	materials["light_door"] = scene.Material{
		Name: "light_door", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{20},
	}
	materials["sloped_door"] = scene.Material{
		Name:                "sloped_door",
		AbsorptionByBand:    []float64{0.08},
		SoundReductionIndex: []float64{5, 10, 20, 40, 70, 90},
	}

	return materials
}

// flankingScene is an L-shaped three-room scene. The receiver in room 1 is
// reachable both directly from room 0 and by flanking through room 2, which is
// the topology the Phase 25.2 flanking test needs.
//
//	room 2 (x 0..8, y 4..8)
//	room 0 (x 0..4, y 0..4) | room 1 (x 4..8, y 0..4)
func flankingScene(t *testing.T) *scene.Scene {
	t.Helper()

	sc := &scene.Scene{
		Rooms: []scene.Room{
			shoeboxAt(geometry.Vec3Zero, 4, 4, 3),
			shoeboxAt(geometry.Vec3{X: 4}, 4, 4, 3),
			shoeboxAt(geometry.Vec3{Y: 4}, 8, 4, 3),
		},
		Portals: []scene.Portal{
			{
				RoomIndices: [2]int{0, 1},
				Polygon:     doorOnX(4, 1.4, 2.6, 2.1),
				Material:    "light_door",
				State:       scene.PortalClosed,
			},
			{
				RoomIndices: [2]int{0, 2},
				Polygon:     doorOnY(4, 1.4, 2.6, 2.1),
				Material:    "light_door",
				State:       scene.PortalClosed,
			},
			{
				RoomIndices: [2]int{1, 2},
				Polygon:     doorOnY(4, 5.4, 6.6, 2.1),
				Material:    "light_door",
				State:       scene.PortalClosed,
			},
		},
		Materials: pathTestMaterials(),
		Sources: []scene.Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position:    geometry.Vec3{X: 6, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("flanking fixture is invalid: %v", err)
	}

	return sc
}

func searchFromSource(t *testing.T, sc *scene.Scene, cfg scene.PathSearchConfig) *scene.PathSearchTree {
	t.Helper()

	graph := newGraph(t, sc)

	sourceGroup, ok := graph.GroupOfPosition(sc.Sources[0].Position)
	if !ok {
		t.Fatal("the source must belong to a group")
	}

	tree, err := graph.SearchPaths(sourceGroup, cfg)
	if err != nil {
		t.Fatalf("SearchPaths: %v", err)
	}

	return tree
}

func TestSearchPathsFindsDirectAndFlankingPathsInThreeRoomChain(t *testing.T) {
	t.Parallel()

	tree := searchFromSource(t, flankingScene(t), scene.PathSearchConfig{})

	if len(tree.Leaves) != 2 {
		t.Fatalf("got %d leaves, want the direct and the flanking path", len(tree.Leaves))
	}

	var direct, flanking *scene.PathNode

	for _, leaf := range tree.Leaves {
		node := &tree.Nodes[leaf]
		switch node.Depth {
		case 1:
			direct = node
		case 2:
			flanking = node
		default:
			t.Fatalf("unexpected leaf depth %d", node.Depth)
		}
	}

	if direct == nil {
		t.Fatal("the direct one-portal path is missing")
	}

	if flanking == nil {
		t.Fatal("the flanking two-portal path is missing")
	}

	// Crossing two doors must attenuate more than crossing one, in every band.
	for band := range direct.Transmission {
		if flanking.Transmission[band] >= direct.Transmission[band] {
			t.Fatalf("band %d: flanking transmission %v is not below direct %v",
				band, flanking.Transmission[band], direct.Transmission[band])
		}
	}

	if flanking.ReductionDB <= direct.ReductionDB {
		t.Fatalf("flanking reduction %v dB is not above direct %v dB",
			flanking.ReductionDB, direct.ReductionDB)
	}

	if tree.Truncated {
		t.Fatal("a three-room scene must not truncate the search")
	}
}

func TestSearchPathsPortalSequenceMatchesTheChain(t *testing.T) {
	t.Parallel()

	sc := chainScene(t, 3, "light_door")
	tree := searchFromSource(t, sc, scene.PathSearchConfig{})

	if len(tree.Leaves) != 1 {
		t.Fatalf("got %d leaves, want one", len(tree.Leaves))
	}

	portals := tree.PortalSequence(tree.Leaves[0])
	if len(portals) != 2 || portals[0] != 0 || portals[1] != 1 {
		t.Fatalf("portal sequence = %v, want [0 1]", portals)
	}

	nodes := tree.PathTo(tree.Leaves[0])
	if len(nodes) != 3 || nodes[0] != tree.Root {
		t.Fatalf("path nodes = %v, want three starting at the root", nodes)
	}
}

func TestSearchPathsCycleDetectionTerminates(t *testing.T) {
	t.Parallel()

	// Four rooms in a ring: 0-1, 1-2, 2-3, 3-0. Without cycle detection the
	// search would loop forever.
	sc := &scene.Scene{
		Rooms: []scene.Room{
			shoeboxAt(geometry.Vec3Zero, 4, 4, 3),
			shoeboxAt(geometry.Vec3{X: 4}, 4, 4, 3),
			shoeboxAt(geometry.Vec3{X: 4, Y: 4}, 4, 4, 3),
			shoeboxAt(geometry.Vec3{Y: 4}, 4, 4, 3),
		},
		Portals: []scene.Portal{
			{RoomIndices: [2]int{0, 1}, Polygon: doorOnX(4, 1.4, 2.6, 2.1), Material: "door", State: scene.PortalClosed},
			{RoomIndices: [2]int{1, 2}, Polygon: doorOnY(4, 5.4, 6.6, 2.1), Material: "door", State: scene.PortalClosed},
			{RoomIndices: [2]int{3, 2}, Polygon: doorOnX(4, 5.4, 6.6, 2.1), Material: "door", State: scene.PortalClosed},
			{RoomIndices: [2]int{0, 3}, Polygon: doorOnY(4, 1.4, 2.6, 2.1), Material: "door", State: scene.PortalClosed},
		},
		Materials: pathTestMaterials(),
		Sources: []scene.Source{{
			Position: geometry.Vec3{X: 2, Y: 2, Z: 1.5}, Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []scene.Receiver{{
			Position: geometry.Vec3{X: 6, Y: 6, Z: 1.5}, Orientation: geometry.QuatIdentity(), Type: scene.ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := scene.Validate(sc)
	if err != nil {
		t.Fatalf("ring fixture is invalid: %v", err)
	}

	tree := searchFromSource(t, sc, scene.PathSearchConfig{PruneFloorDB: -300})

	// Every path must be simple: no group may repeat along it.
	for index := range tree.Nodes {
		seen := map[scene.GroupID]bool{}

		for _, node := range tree.PathTo(index) {
			group := tree.Nodes[node].Group
			if seen[group] {
				t.Fatalf("group %d repeats along the path to node %d", group, index)
			}

			seen[group] = true
		}
	}

	if tree.Truncated {
		t.Fatal("a four-group ring must be searched exhaustively")
	}
}

func TestSearchPathsPrunesBelowFloor(t *testing.T) {
	t.Parallel()

	// Each door reduces by 40 dB, so two hops exceed the -60 dB floor.
	sc := chainScene(t, 3, "quiet_door")

	tree := searchFromSource(t, sc, scene.PathSearchConfig{})
	if len(tree.Leaves) != 0 {
		t.Fatalf("got %d leaves, want none past the -60 dB floor", len(tree.Leaves))
	}

	if !tree.Truncated {
		t.Fatal("a pruned branch must mark the tree as truncated")
	}

	// Lowering the floor must reveal the same path.
	deep := searchFromSource(t, sc, scene.PathSearchConfig{PruneFloorDB: -200})
	if len(deep.Leaves) != 1 {
		t.Fatalf("got %d leaves with a -200 dB floor, want one", len(deep.Leaves))
	}
}

func TestSearchPathsRespectsMaxDepth(t *testing.T) {
	t.Parallel()

	sc := chainScene(t, 4, "light_door")

	shallow := searchFromSource(t, sc, scene.PathSearchConfig{MaxDepth: 2, PruneFloorDB: -300})
	if len(shallow.Leaves) != 0 {
		t.Fatalf("got %d leaves at depth 2, want none for a three-portal chain", len(shallow.Leaves))
	}

	if !shallow.Truncated {
		t.Fatal("hitting MaxDepth must mark the tree as truncated")
	}

	deep := searchFromSource(t, sc, scene.PathSearchConfig{MaxDepth: 3, PruneFloorDB: -300})
	if len(deep.Leaves) != 1 {
		t.Fatalf("got %d leaves at depth 3, want one", len(deep.Leaves))
	}
}

func TestSearchPathsTruncatedOnMaxNodes(t *testing.T) {
	t.Parallel()

	sc := chainScene(t, 4, "light_door")

	tree := searchFromSource(t, sc, scene.PathSearchConfig{MaxNodes: 2, PruneFloorDB: -300})
	if !tree.Truncated {
		t.Fatal("hitting MaxNodes must mark the tree as truncated")
	}

	if len(tree.Nodes) > 2 {
		t.Fatalf("got %d nodes, want at most the 2 requested", len(tree.Nodes))
	}
}

func TestSourceEliminationMasksFullyAttenuatedBands(t *testing.T) {
	t.Parallel()

	// The door's reduction rises from 5 dB at 125 Hz to 90 dB at 4 kHz, so the
	// top bands fall below the floor while the bottom bands survive.
	sc := chainScene(t, 2, "sloped_door")

	tree := searchFromSource(t, sc, scene.PathSearchConfig{})
	if len(tree.Leaves) != 1 {
		t.Fatalf("got %d leaves, want one", len(tree.Leaves))
	}

	active := tree.Nodes[tree.Leaves[0]].ActiveBands
	if !active[0] {
		t.Fatal("the 125 Hz band must survive a 5 dB reduction")
	}

	if active[5] {
		t.Fatal("the 4 kHz band must be eliminated by a 90 dB reduction")
	}
}

func TestSearchPathsRootIsALeafWhenSourceAndReceiverShareAGroup(t *testing.T) {
	t.Parallel()

	sc := chainScene(t, 2, "door")
	graph := newGraph(t, sc)
	openPortals(t, graph, 0)

	sourceGroup, _ := graph.GroupOfPosition(sc.Sources[0].Position)

	tree, err := graph.SearchPaths(sourceGroup, scene.PathSearchConfig{})
	if err != nil {
		t.Fatalf("SearchPaths: %v", err)
	}

	if len(tree.Leaves) != 1 || tree.Leaves[0] != tree.Root {
		t.Fatalf("leaves = %v, want just the root once the door is open", tree.Leaves)
	}

	if len(tree.Nodes) != 1 {
		t.Fatalf("got %d nodes, want one: an open portal is no longer an edge", len(tree.Nodes))
	}
}

func TestSearchPathsRejectsUnknownSourceGroup(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, chainScene(t, 2, "door"))

	_, err := graph.SearchPaths(scene.GroupID(99), scene.PathSearchConfig{})
	if err == nil {
		t.Fatal("SearchPaths accepted a group that does not exist")
	}
}

func TestWeightedReductionIndexDBMatchesAFlatPartition(t *testing.T) {
	t.Parallel()

	// With a flat spectrum the weighted average must return the underlying
	// single-number reduction exactly, whatever the weights are.
	for _, reduction := range []float64{0, 10, 35, 60} {
		tau := scene.TransmissionFromSoundReductionIndex(reduction)

		transmission := make([]float64, 6)
		for index := range transmission {
			transmission[index] = tau
		}

		got := scene.WeightedReductionIndexDB(transmission, nil)
		if math.Abs(got-reduction) > 1e-9 {
			t.Fatalf("WeightedReductionIndexDB(flat %v dB) = %v", reduction, got)
		}
	}
}

func TestWeightedReductionIndexDBIsMonotonic(t *testing.T) {
	t.Parallel()

	previous := math.Inf(-1)

	for _, reduction := range []float64{0, 5, 10, 20, 40, 80} {
		tau := scene.TransmissionFromSoundReductionIndex(reduction)

		transmission := make([]float64, 6)
		for index := range transmission {
			transmission[index] = tau
		}

		got := scene.WeightedReductionIndexDB(transmission, nil)
		if got <= previous {
			t.Fatalf("reduction index %v is not above the previous %v", got, previous)
		}

		previous = got
	}
}

func TestDefaultReductionWeightsSumToOne(t *testing.T) {
	t.Parallel()

	for _, bandCount := range []int{1, 6, 8} {
		weights := scene.DefaultReductionWeights(bandCount)
		if len(weights) != bandCount {
			t.Fatalf("DefaultReductionWeights(%d) has length %d", bandCount, len(weights))
		}

		total := 0.0

		for _, weight := range weights {
			if weight <= 0 {
				t.Fatalf("DefaultReductionWeights(%d) has a non-positive weight", bandCount)
			}

			total += weight
		}

		if math.Abs(total-1) > 1e-12 {
			t.Fatalf("DefaultReductionWeights(%d) sums to %v, want 1", bandCount, total)
		}
	}
}

func TestOpenPortalsDoNotAppearAsGroupEdges(t *testing.T) {
	t.Parallel()

	sc := chainScene(t, 3, "light_door")
	graph := newGraph(t, sc)

	before, _ := graph.GroupOf(0)
	if got := len(graph.GroupPortalViews(before)); got != 1 {
		t.Fatalf("closed chain: room 0's group has %d edges, want 1", got)
	}

	openPortals(t, graph, 0)

	after, _ := graph.GroupOf(0)

	views := graph.GroupPortalViews(after)
	for _, view := range views {
		if view.PortalIndex == 0 {
			t.Fatal("an opened portal must not remain a group edge")
		}
	}

	if got := len(views); got != 1 {
		t.Fatalf("merged group has %d edges, want the single remaining closed door", got)
	}
}
