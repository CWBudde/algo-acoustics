package scene

import (
	"strings"
	"testing"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/geometry"
)

const graphTestEps = 1e-9

func graphTestMaterials() map[string]Material {
	return map[string]Material{
		"wall":    {Name: "wall", AbsorptionByBand: []float64{0.1}, ScatteringByBand: []float64{0.05}},
		"floor":   {Name: "floor", AbsorptionByBand: []float64{0.05}, ScatteringByBand: []float64{0.05}},
		"ceiling": {Name: "ceiling", AbsorptionByBand: []float64{0.2}, ScatteringByBand: []float64{0.05}},
		"door":    {Name: "door", AbsorptionByBand: []float64{0.08}, SoundReductionIndex: []float64{30}},
	}
}

// shoeboxAt builds a room whose walls use distinguishable materials so that
// merged-group triangle materials can be checked per face.
func shoeboxAt(origin geometry.Vec3, width, depth, height float64) Room { //nolint:unparam // Kept general so fixtures can vary every dimension.
	return Room{
		Kind: RoomKindShoebox,
		Shoebox: &Shoebox{
			Origin: origin,
			Width:  width, Depth: depth, Height: height,
			WallMaterials: [6]string{"wall", "wall", "wall", "wall", "floor", "ceiling"},
		},
	}
}

// doorOnX builds a floor-standing door polygon on the plane x = atX, wound from
// the room on the negative side toward the room on the positive side. The door
// deliberately reaches the floor, matching the shipped two-room fixture.
func doorOnX(atX, yMin, yMax, height float64) []geometry.Vec3 {
	return []geometry.Vec3{
		{X: atX, Y: yMin, Z: 0},
		{X: atX, Y: yMax, Z: 0},
		{X: atX, Y: yMax, Z: height},
		{X: atX, Y: yMin, Z: height},
	}
}

// doorOnY builds a floor-standing door polygon on the plane y = atY, wound from
// the room on the negative side toward the room on the positive side.
func doorOnY(atY, xMin, xMax, height float64) []geometry.Vec3 {
	return []geometry.Vec3{
		{X: xMax, Y: atY, Z: 0},
		{X: xMin, Y: atY, Z: 0},
		{X: xMin, Y: atY, Z: height},
		{X: xMax, Y: atY, Z: height},
	}
}

// officeFloorScene builds eight 4 m x 4 m x 3 m offices in two rows of four,
// joined by ten doors: three along each row and four across the corridor
// between the rows.
//
//	row 1 (y = 4..8):  4  5  6  7
//	row 0 (y = 0..4):  0  1  2  3
func officeFloorScene(t *testing.T) *Scene {
	t.Helper()

	const (
		size   = 4.0
		height = 3.0
	)

	rooms := make([]Room, 0, 8)

	for row := range 2 {
		for column := range 4 {
			rooms = append(rooms, shoeboxAt(
				geometry.Vec3{X: float64(column) * size, Y: float64(row) * size},
				size, size, height,
			))
		}
	}

	portals := make([]Portal, 0, 10)

	// Three doors along each row, on the shared x planes.
	for row := range 2 {
		for column := range 3 {
			first := row*4 + column
			portals = append(portals, Portal{
				RoomIndices: [2]int{first, first + 1},
				Polygon:     doorOnX(float64(column+1)*size, float64(row)*size+1.4, float64(row)*size+2.6, 2.1),
				Material:    "door",
				State:       PortalClosed,
			})
		}
	}

	// Four doors across the corridor, on the shared y = 4 plane.
	for column := range 4 {
		portals = append(portals, Portal{
			RoomIndices: [2]int{column, column + 4},
			Polygon:     doorOnY(size, float64(column)*size+1.4, float64(column)*size+2.6, 2.1),
			Material:    "door",
			State:       PortalClosed,
		})
	}

	sc := &Scene{
		Rooms:     rooms,
		Portals:   portals,
		Materials: graphTestMaterials(),
		Sources: []Source{{
			Position:    geometry.Vec3{X: 2, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
		}},
		Receivers: []Receiver{{
			Position:    geometry.Vec3{X: 6, Y: 2, Z: 1.5},
			Orientation: geometry.QuatIdentity(),
			Type:        ReceiverOmni,
		}},
		BandSpec:   acoustics.Octave6,
		SampleRate: 48000,
	}

	err := Validate(sc)
	if err != nil {
		t.Fatalf("office floor fixture is invalid: %v", err)
	}

	return sc
}

func newGraph(t *testing.T, sc *Scene) *AcousticSceneGraph {
	t.Helper()

	graph, err := NewAcousticSceneGraph(sc)
	if err != nil {
		t.Fatalf("NewAcousticSceneGraph: %v", err)
	}

	return graph
}

func openPortals(t *testing.T, graph *AcousticSceneGraph, indices ...int) {
	t.Helper()

	for _, index := range indices {
		err := graph.SetPortalState(index, PortalOpen)
		if err != nil {
			t.Fatalf("SetPortalState(%d, open): %v", index, err)
		}
	}
}

// groupSets returns the group memberships as a comparable string so that tests
// can assert the partition rather than particular identifiers.
func groupSets(t *testing.T, graph *AcousticSceneGraph) []string {
	t.Helper()

	sets := make([]string, 0, graph.GroupCount())

	for id := range graph.GroupCount() {
		rooms, ok := graph.GroupRooms(GroupID(id))
		if !ok {
			t.Fatalf("GroupRooms(%d) is missing", id)
		}

		parts := make([]string, 0, len(rooms))
		for _, room := range rooms {
			parts = append(parts, string(rune('0'+room)))
		}

		sets = append(sets, strings.Join(parts, ","))
	}

	return sets
}

func TestAcousticSceneGraphGroupsMergeRoomsJoinedByOpenPortals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		open  []int
		want  []string
		count int
	}{
		{
			name:  "all closed leaves every room alone",
			want:  []string{"0", "1", "2", "3", "4", "5", "6", "7"},
			count: 8,
		},
		{
			name:  "one open door merges its pair",
			open:  []int{0},
			want:  []string{"0,1", "2", "3", "4", "5", "6", "7"},
			count: 7,
		},
		{
			name:  "a chain along one row merges four rooms",
			open:  []int{0, 1, 2},
			want:  []string{"0,1,2,3", "4", "5", "6", "7"},
			count: 5,
		},
		{
			name:  "a corridor door bridges the two rows",
			open:  []int{0, 6},
			want:  []string{"0,1,4", "2", "3", "5", "6", "7"},
			count: 6,
		},
		{
			name:  "every door open collapses the floor into one group",
			open:  []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:  []string{"0,1,2,3,4,5,6,7"},
			count: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			graph := newGraph(t, officeFloorScene(t))
			openPortals(t, graph, test.open...)

			if got := graph.GroupCount(); got != test.count {
				t.Fatalf("GroupCount = %d, want %d", got, test.count)
			}

			got := groupSets(t, graph)
			if strings.Join(got, " | ") != strings.Join(test.want, " | ") {
				t.Fatalf("groups = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAcousticSceneGraphGroupIdentifiersAreDeterministic(t *testing.T) {
	t.Parallel()

	// Downstream caches key on group identity, so the same portal
	// configuration must always produce the same numbering.
	want := ""

	for range 8 {
		graph := newGraph(t, officeFloorScene(t))
		openPortals(t, graph, 6, 1, 3)

		got := strings.Join(groupSets(t, graph), " | ")
		if want == "" {
			want = got

			continue
		}

		if got != want {
			t.Fatalf("group numbering is not deterministic: %q vs %q", got, want)
		}
	}
}

func TestSetPortalStateRecomputesRoomGroups(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))
	openPortals(t, graph, 0, 1)

	if got := graph.GroupCount(); got != 6 {
		t.Fatalf("GroupCount after opening two doors = %d, want 6", got)
	}

	first, _ := graph.GroupOf(0)
	if third, _ := graph.GroupOf(2); first != third {
		t.Fatal("rooms 0 and 2 must share a group while the chain is open")
	}

	err := graph.SetPortalState(1, PortalClosed)
	if err != nil {
		t.Fatalf("SetPortalState: %v", err)
	}

	if got := graph.GroupCount(); got != 7 {
		t.Fatalf("GroupCount after closing the middle door = %d, want 7", got)
	}

	first, _ = graph.GroupOf(0)
	if third, _ := graph.GroupOf(2); first == third {
		t.Fatal("rooms 0 and 2 must fall into different groups once the chain is broken")
	}
}

func TestSetPortalStateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))

	err := graph.SetPortalState(99, PortalOpen)
	if err == nil {
		t.Fatal("SetPortalState accepted an out-of-range portal index")
	}

	err = graph.SetPortalState(0, PortalState("ajar"))
	if err == nil {
		t.Fatal("SetPortalState accepted an unsupported state")
	}
}

func TestPortalViewCounterIsInvolutiveAndFlipsTheNormal(t *testing.T) {
	t.Parallel()

	sc := officeFloorScene(t)
	graph := newGraph(t, sc)

	for _, view := range graph.PortalViews(1) {
		if view.Counter().Counter() != view {
			t.Fatalf("Counter is not involutive for %+v", view)
		}

		if view.Counter().FromRoom != view.ToRoom || view.Counter().ToRoom != view.FromRoom {
			t.Fatalf("Counter did not swap the rooms of %+v", view)
		}

		normal := view.Normal(sc)
		counterNormal := view.Counter().Normal(sc)

		if normal.Add(counterNormal).Norm() > graphTestEps {
			t.Fatalf("counter normal %+v is not the negation of %+v", counterNormal, normal)
		}
	}
}

func TestPortalViewNormalAlwaysPointsAwayFromItsRoom(t *testing.T) {
	t.Parallel()

	sc := officeFloorScene(t)
	graph := newGraph(t, sc)

	for roomIndex := range sc.RoomCount() {
		room, _ := sc.RoomAt(roomIndex)
		center := room.Shoebox.Bounds().Center()

		for _, view := range graph.PortalViews(roomIndex) {
			portal, _ := view.Portal(sc)
			toPortal := portal.Center().Sub(center)

			if view.Normal(sc).Dot(toPortal) <= 0 {
				t.Fatalf("portal %d normal does not lead out of room %d", view.PortalIndex, roomIndex)
			}
		}
	}
}

func TestGroupPortalViewsListOnlyClosedPortalsLeavingTheGroup(t *testing.T) {
	t.Parallel()

	graph := newGraph(t, officeFloorScene(t))
	openPortals(t, graph, 0)

	id, _ := graph.GroupOf(0)

	views := graph.GroupPortalViews(id)
	for _, view := range views {
		if view.FromGroup != id {
			t.Fatalf("view %+v does not start in group %d", view, id)
		}

		if view.ToGroup == id {
			t.Fatal("an open portal inside the group must not appear as a group edge")
		}

		if view.PortalIndex == 0 {
			t.Fatal("the opened portal must not appear as a group edge")
		}
	}

	// Rooms 0 and 1 are merged; their remaining closed doors are the one to
	// room 2 and the two corridor doors to rooms 4 and 5.
	if len(views) != 3 {
		t.Fatalf("got %d group edges, want 3", len(views))
	}
}
