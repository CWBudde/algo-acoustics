package scene

import "fmt"

// GroupScene returns a single-room scene covering one room group, so that every
// existing single-room engine — the image-source solvers, the ray tracer, the
// hybrid combiner — can simulate a group without knowing the graph exists.
//
// A group that is exactly one shoebox room with no open portal keeps its
// shoebox representation, so the analytic shoebox solvers stay in play and
// single-room scenes behave exactly as before. Any other group becomes a mesh
// room carrying the merged boundary and its per-triangle materials.
//
// Sources and receivers are filtered to those inside the group. Coordinates
// stay in world space; callers that need an origin-anchored shoebox translate
// separately.
func (g *AcousticSceneGraph) GroupScene(id GroupID) (*Scene, error) {
	rooms, ok := g.GroupRooms(id)
	if !ok {
		return nil, fmt.Errorf("group %d does not exist", id)
	}

	sub := *g.scene
	sub.Rooms = nil
	sub.Portals = nil
	sub.Sources = g.sourcesInGroup(id)
	sub.Receivers = g.receiversInGroup(id)

	if room, ok := g.soleShoeboxRoom(rooms); ok {
		sub.Room = *room

		return &sub, nil
	}

	group, err := g.GroupGeometry(id)
	if err != nil {
		return nil, err
	}

	sub.Room = Room{
		Kind:              RoomKindMesh,
		Mesh:              group.Mesh,
		TriangleMaterials: group.TriangleMaterials,
	}

	return &sub, nil
}

// soleShoeboxRoom reports the group's only room when it is a shoebox that no
// open portal cuts into, which is the fast path that keeps existing single-room
// scenes on the analytic solvers.
func (g *AcousticSceneGraph) soleShoeboxRoom(rooms []int) (*Room, bool) {
	if len(rooms) != 1 {
		return nil, false
	}

	room, ok := g.scene.RoomAt(rooms[0])
	if !ok || room.Kind != RoomKindShoebox || room.Shoebox == nil {
		return nil, false
	}

	for _, view := range g.incident[rooms[0]] {
		if g.scene.Portals[view.PortalIndex].State == PortalOpen {
			return nil, false
		}
	}

	return room, true
}

func (g *AcousticSceneGraph) sourcesInGroup(id GroupID) []Source {
	var sources []Source

	for _, source := range g.scene.Sources {
		if found, ok := g.GroupOfPosition(source.Position); ok && found == id {
			sources = append(sources, source)
		}
	}

	return sources
}

func (g *AcousticSceneGraph) receiversInGroup(id GroupID) []Receiver {
	var receivers []Receiver

	for _, receiver := range g.scene.Receivers {
		if found, ok := g.GroupOfPosition(receiver.Position); ok && found == id {
			receivers = append(receivers, receiver)
		}
	}

	return receivers
}
