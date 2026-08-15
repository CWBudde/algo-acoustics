package algoacoustics

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// LowFreqSceneForMultiRoom builds the single-room scene whose modal response
// stands in for a multi-room render's low-frequency content.
//
// Modal behaviour below the Schroeder frequency is dominated by the room the
// listener is actually in, so the transfer function is computed for the
// receiver's own room group. The group holds no primary source, so an
// omnidirectional substitute is placed just inside the portal that admits sound
// to it — the same secondary-source idea the geometric path already uses, at a
// frequency range where the portal is small compared with the wavelength and
// therefore behaves like a point source.
//
// This is an approximation and is documented as one: it captures the receiving
// room's modes and their excitation position, not the coupled modal behaviour
// of two volumes sharing an aperture. It is refused rather than fudged when the
// receiver's group is not a single shoebox, since the solver has no mesh
// formulation.
func LowFreqSceneForMultiRoom(sc *scene.Scene) (*scene.Scene, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
	}

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		return nil, fmt.Errorf("build scene graph: %w", err)
	}

	group, ok := graph.GroupOfPosition(sc.Receivers[0].Position)
	if !ok {
		return nil, errors.New("receiver must belong to exactly one room group")
	}

	sub, err := graph.GroupScene(group)
	if err != nil {
		return nil, fmt.Errorf("extract the receiver's room group: %w", err)
	}

	if sub.Room.Kind != scene.RoomKindShoebox || sub.Room.Shoebox == nil {
		return nil, errors.New(
			"low-frequency blending needs the receiver's room group to be a single shoebox; " +
				"the modal solver has no mesh formulation",
		)
	}

	position, err := lowFreqSubstituteSourcePosition(graph, group)
	if err != nil {
		return nil, err
	}

	sub.Sources = []scene.Source{{Position: position, Orientation: geometry.QuatIdentity()}}

	return sub, nil
}

// lowFreqSubstituteSourcePosition returns a point just inside the receiver's
// group, at the centre of the lowest-indexed portal bounding it. The lowest
// index keeps the choice deterministic across runs.
func lowFreqSubstituteSourcePosition(graph *scene.AcousticSceneGraph, group scene.GroupID) (geometry.Vec3, error) {
	views := graph.GroupPortalViews(group)
	if len(views) == 0 {
		return geometry.Vec3Zero, fmt.Errorf("group %d has no portal to excite it", group)
	}

	best := views[0]
	for _, view := range views[1:] {
		if view.PortalIndex < best.PortalIndex {
			best = view
		}
	}

	portal, ok := best.Portal(graph.Scene())
	if !ok {
		return geometry.Vec3Zero, fmt.Errorf("portal %d is missing", best.PortalIndex)
	}

	// best points out of the group, so step against its normal to land inside.
	inward := best.Normal(graph.Scene()).Scale(-networkPortalOffsetMeters)

	return portal.Center().Add(inward), nil
}
