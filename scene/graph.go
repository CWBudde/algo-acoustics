package scene

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cwbudde/algo-acoustics/geometry"
)

// GroupID identifies a room group: the set of rooms an open portal chain joins
// into a single acoustic space.
type GroupID int

// NoGroup marks a room or position that belongs to no group.
const NoGroup GroupID = -1

// PortalView is a portal seen from one of the two rooms it joins. It takes the
// place of RAVEN's counter-portal pointer (docs/raven.md section 10): rather
// than storing a second portal object per room, the opposite view is produced
// on demand by Counter.
type PortalView struct {
	PortalIndex int
	FromRoom    int
	ToRoom      int
	// Reversed reports that FromRoom is the portal's second room, so the
	// polygon winding points against this view's direction.
	Reversed bool
}

// Counter returns the same portal seen from the adjacent room.
func (v PortalView) Counter() PortalView {
	return PortalView{
		PortalIndex: v.PortalIndex,
		FromRoom:    v.ToRoom,
		ToRoom:      v.FromRoom,
		Reversed:    !v.Reversed,
	}
}

// Normal returns the portal normal oriented from FromRoom toward ToRoom,
// regardless of how the polygon was wound.
func (v PortalView) Normal(s *Scene) geometry.Vec3 {
	portal, ok := v.Portal(s)
	if !ok {
		return geometry.Vec3Zero
	}

	normal := portal.Normal()
	if v.Reversed {
		return normal.Scale(-1)
	}

	return normal
}

// Portal resolves the underlying portal.
func (v PortalView) Portal(s *Scene) (Portal, bool) {
	if s == nil || v.PortalIndex < 0 || v.PortalIndex >= len(s.Portals) {
		return Portal{}, false
	}

	return s.Portals[v.PortalIndex], true
}

// GroupPortalView is a portal view lifted to the group graph. Only closed
// portals produce these: an open portal has already merged its two rooms into
// one group, so it is no longer an edge.
type GroupPortalView struct {
	PortalView

	FromGroup GroupID
	ToGroup   GroupID
}

// AcousticSceneGraph is the multi-room scene graph of docs/raven.md section 5.1:
// rooms are nodes, portals are edges, and rooms joined by open portals collapse
// into room groups that are simulated as one acoustic space.
//
// The graph holds a reference to the scene rather than a copy, and SetPortalState
// mutates that scene's portal state. It is not safe for concurrent mutation; the
// internal caches are guarded so concurrent reads of derived geometry are safe.
type AcousticSceneGraph struct {
	scene    *Scene
	incident [][]PortalView

	groupOfRoom  []GroupID
	roomsOfGroup [][]int

	// Derived geometry is keyed on the hash of a group's rooms and portals
	// rather than on its GroupID, because opening a portal renumbers the
	// groups. Hashing is what lets a door toggle in one part of a building
	// leave every unaffected group warm.
	cacheMu   sync.RWMutex
	geomCache map[uint64]*GroupGeometry
	bvhCache  map[uint64]*geometry.BVHNode
}

// NewAcousticSceneGraph validates the scene and builds its room graph. Room
// groups are computed before returning, so the graph is immediately usable.
func NewAcousticSceneGraph(s *Scene) (*AcousticSceneGraph, error) {
	if s == nil {
		return nil, errors.New("scene is nil")
	}

	err := Validate(s)
	if err != nil {
		return nil, fmt.Errorf("validate scene graph: %w", err)
	}

	roomCount := s.RoomCount()
	if roomCount == 0 {
		return nil, errors.New("scene graph requires at least one room")
	}

	graph := &AcousticSceneGraph{
		scene:     s,
		incident:  make([][]PortalView, roomCount),
		geomCache: map[uint64]*GroupGeometry{},
		bvhCache:  map[uint64]*geometry.BVHNode{},
	}

	for index, portal := range s.Portals {
		first, second := portal.RoomIndices[0], portal.RoomIndices[1]
		graph.incident[first] = append(graph.incident[first], PortalView{
			PortalIndex: index, FromRoom: first, ToRoom: second,
		})
		graph.incident[second] = append(graph.incident[second], PortalView{
			PortalIndex: index, FromRoom: second, ToRoom: first, Reversed: true,
		})
	}

	graph.UpdateRoomGroups()

	return graph, nil
}

// Scene returns the scene the graph was built from.
func (g *AcousticSceneGraph) Scene() *Scene {
	if g == nil {
		return nil
	}

	return g.scene
}

// RoomCount returns the number of rooms in the graph.
func (g *AcousticSceneGraph) RoomCount() int {
	if g == nil {
		return 0
	}

	return len(g.incident)
}

// PortalViews returns every portal incident to a room, oriented outward from it.
func (g *AcousticSceneGraph) PortalViews(roomIndex int) []PortalView {
	if g == nil || roomIndex < 0 || roomIndex >= len(g.incident) {
		return nil
	}

	return append([]PortalView(nil), g.incident[roomIndex]...)
}

// SetPortalState changes a portal's state and recomputes the room groups.
func (g *AcousticSceneGraph) SetPortalState(portalIndex int, state PortalState) error {
	if g == nil || g.scene == nil {
		return errors.New("scene graph is nil")
	}

	if portalIndex < 0 || portalIndex >= len(g.scene.Portals) {
		return fmt.Errorf("portal index %d is out of range", portalIndex)
	}

	if state != PortalOpen && state != PortalClosed {
		return fmt.Errorf("unsupported portal state %q", state)
	}

	if g.scene.Portals[portalIndex].State == state {
		return nil
	}

	g.scene.Portals[portalIndex].State = state
	g.UpdateRoomGroups()

	return nil
}

// UpdateRoomGroups recomputes the connected components of rooms joined by open
// portals. Rooms are visited in ascending index and groups are numbered in the
// order their lowest-indexed room is reached, so group identifiers are stable
// for a given portal configuration.
//
// Cached group geometry is keyed on content, so a group whose rooms and
// portals are unchanged keeps its entry across a toggle elsewhere in the
// building.
func (g *AcousticSceneGraph) UpdateRoomGroups() {
	if g == nil {
		return
	}

	roomCount := len(g.incident)
	groupOfRoom := make([]GroupID, roomCount)

	for index := range groupOfRoom {
		groupOfRoom[index] = NoGroup
	}

	var roomsOfGroup [][]int

	queue := make([]int, 0, roomCount)

	for start := range roomCount {
		if groupOfRoom[start] != NoGroup {
			continue
		}

		id := GroupID(len(roomsOfGroup))
		groupOfRoom[start] = id
		queue = append(queue[:0], start)
		members := []int{start}

		for len(queue) > 0 {
			room := queue[0]
			queue = queue[1:]

			for _, view := range g.incident[room] {
				portal := g.scene.Portals[view.PortalIndex]
				if portal.State != PortalOpen || groupOfRoom[view.ToRoom] != NoGroup {
					continue
				}

				groupOfRoom[view.ToRoom] = id
				members = append(members, view.ToRoom)
				queue = append(queue, view.ToRoom)
			}
		}

		slicesSortInts(members)
		roomsOfGroup = append(roomsOfGroup, members)
	}

	g.groupOfRoom = groupOfRoom
	g.roomsOfGroup = roomsOfGroup
	g.evictUnreachableGeometry()
}

// GroupCount returns the number of room groups.
func (g *AcousticSceneGraph) GroupCount() int {
	if g == nil {
		return 0
	}

	return len(g.roomsOfGroup)
}

// GroupOf returns the group a room belongs to.
func (g *AcousticSceneGraph) GroupOf(roomIndex int) (GroupID, bool) {
	if g == nil || roomIndex < 0 || roomIndex >= len(g.groupOfRoom) {
		return NoGroup, false
	}

	return g.groupOfRoom[roomIndex], true
}

// GroupRooms returns the ascending room indices of a group.
func (g *AcousticSceneGraph) GroupRooms(id GroupID) ([]int, bool) {
	if g == nil || id < 0 || int(id) >= len(g.roomsOfGroup) {
		return nil, false
	}

	return append([]int(nil), g.roomsOfGroup[id]...), true
}

// GroupOfPosition returns the group containing a world-space position.
//
// It is deliberately more forgiving than Scene.RoomIndexAt, which reports a
// point on a shared boundary as ambiguous. A source standing in a doorway lies
// in two rooms, but if those rooms share a group the point is not ambiguous at
// group level. Ambiguity between different groups remains an error.
func (g *AcousticSceneGraph) GroupOfPosition(p geometry.Vec3) (GroupID, bool) {
	if g == nil || g.scene == nil {
		return NoGroup, false
	}

	match := NoGroup

	for roomIndex := range g.scene.RoomCount() {
		room, ok := g.scene.RoomAt(roomIndex)
		if !ok {
			continue
		}

		bounds, ok := room.Bounds()
		if !ok || !bounds.Contains(p) {
			continue
		}

		id, ok := g.GroupOf(roomIndex)
		if !ok {
			continue
		}

		if match != NoGroup && match != id {
			return NoGroup, false
		}

		match = id
	}

	return match, match != NoGroup
}

// GroupPortalViews returns the closed portals leading out of a group, which are
// the edges of the group graph that the propagation path search walks. Open
// portals are absent by construction: they have already been merged away into
// the group itself.
func (g *AcousticSceneGraph) GroupPortalViews(from GroupID) []GroupPortalView {
	rooms, ok := g.GroupRooms(from)
	if !ok {
		return nil
	}

	var views []GroupPortalView

	for _, roomIndex := range rooms {
		for _, view := range g.incident[roomIndex] {
			if g.scene.Portals[view.PortalIndex].State == PortalOpen {
				continue
			}

			target, ok := g.GroupOf(view.ToRoom)
			if !ok || target == from {
				continue
			}

			views = append(views, GroupPortalView{PortalView: view, FromGroup: from, ToGroup: target})
		}
	}

	return views
}

// evictUnreachableGeometry drops cached entries that no current group can reach.
// Correctness comes from the hash key alone; this only keeps the cache from
// growing without bound as portals are toggled.
func (g *AcousticSceneGraph) evictUnreachableGeometry() {
	live := make(map[uint64]bool, len(g.roomsOfGroup))
	for _, rooms := range g.roomsOfGroup {
		live[g.scene.RoomGroupHash(rooms)] = true
	}

	g.cacheMu.Lock()
	defer g.cacheMu.Unlock()

	for key := range g.geomCache {
		if !live[key] {
			delete(g.geomCache, key)
			delete(g.bvhCache, key)
		}
	}

	for key := range g.bvhCache {
		if !live[key] {
			delete(g.bvhCache, key)
		}
	}
}

// slicesSortInts sorts a small ascending index list in place. Groups hold a
// handful of rooms, so an insertion sort avoids pulling in a dependency for no
// measurable gain.
func slicesSortInts(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j-1] > values[j]; j-- {
			values[j-1], values[j] = values[j], values[j-1]
		}
	}
}
