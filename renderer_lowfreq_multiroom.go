package algoacoustics

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/scene"
)

// MultiRoomLowFreq is the modal stand-in for a multi-room render: the
// single-room scene whose modes are solved, plus the pressure gain of
// everything upstream of it.
type MultiRoomLowFreq struct {
	// Scene is the receiver's room group, localized to the origin and holding
	// the substitute source.
	Scene *scene.Scene
	// PressureGain is the accumulated portal transmission of the propagation
	// path that excites the group, in the pressure domain. The modal solve
	// itself knows nothing of the partitions upstream of it, so without this
	// factor a wall attenuating the geometric field by tens of dB would leave
	// the blended low-frequency field untouched.
	PressureGain float64
}

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
// Which portal that is comes from the propagation path search, ranked exactly as
// the geometric render ranks it, so the modal excitation sits on the path the
// rendered field actually arrives by. Choosing by portal index instead would
// happily excite a dead-end branch.
//
// This is an approximation and is documented as one. It captures the receiving
// room's modes, their excitation position, and the transmission loss upstream of
// them; it does not capture the coupled modal behaviour of two volumes sharing
// an aperture, nor the propagation delay and spreading loss upstream, which the
// blend's crossover region largely masks. It is refused rather than fudged when
// the receiver's group is not a single shoebox, since the solver has no mesh
// formulation.
func LowFreqSceneForMultiRoom(sc *scene.Scene) (*MultiRoomLowFreq, error) {
	if sc == nil {
		return nil, errors.New("scene is nil")
	}

	if len(sc.Receivers) == 0 {
		return nil, errors.New("scene has no receivers")
	}

	if len(sc.Sources) == 0 {
		return nil, errors.New("scene has no sources")
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

	source, gain, err := lowFreqSubstituteSource(graph, sc, group)
	if err != nil {
		return nil, err
	}

	sub.Sources = []scene.Source{source}

	// The Helmholtz solver indexes cells from a zero origin and ignores
	// Shoebox.Origin, so a group anywhere but at the origin would have its
	// source and receiver clamped to the wrong boundary cells — often the same
	// one. Localizing here is what the geometric factors already do.
	localized := networkLocalizedScene(sub)

	return &MultiRoomLowFreq{Scene: localized, PressureGain: gain}, nil
}

// lowFreqSubstituteSource returns the source that excites the receiver's group
// and the pressure gain of the path reaching it.
//
// When source and receiver already share a group there is nothing to substitute:
// the real source excites the modes directly and nothing attenuates it.
func lowFreqSubstituteSource(
	graph *scene.AcousticSceneGraph,
	sc *scene.Scene,
	receiverGroup scene.GroupID,
) (scene.Source, float64, error) {
	sourceGroup, ok := graph.GroupOfPosition(sc.Sources[0].Position)
	if !ok {
		return scene.Source{}, 0, errors.New("source must belong to exactly one room group")
	}

	if sourceGroup == receiverGroup {
		return sc.Sources[0], 1, nil
	}

	path, err := strongestPathToGroup(graph, sc, sourceGroup, receiverGroup)
	if err != nil {
		return scene.Source{}, 0, err
	}

	entry := path.portals[len(path.portals)-1]

	portal, ok := entry.Portal(graph.Scene())
	if !ok {
		return scene.Source{}, 0, fmt.Errorf("portal %d is missing", entry.PortalIndex)
	}

	// The view points out of the group it was traversed from, so stepping along
	// its normal lands inside the receiver's group.
	position := portal.Center().Add(entry.Normal(graph.Scene()).Scale(networkPortalOffsetMeters))

	return scene.Source{Position: position, Orientation: geometry.QuatIdentity()},
		lowFreqPressureGain(path),
		nil
}

// strongestPathToGroup returns the highest-energy propagation path reaching a
// group, ranked exactly as the geometric render ranks it.
func strongestPathToGroup(
	graph *scene.AcousticSceneGraph,
	sc *scene.Scene,
	sourceGroup, receiverGroup scene.GroupID,
) (networkPath, error) {
	renderer := NewNetworkRenderer(NetworkRendererConfig{})

	tree, err := graph.SearchPaths(sourceGroup, scene.PathSearchConfig{
		MaxDepth:     renderer.maxPathHops(),
		PruneFloorDB: renderer.bandFloorDB(),
		BandCount:    sc.BandSpec.BandCount(),
	})
	if err != nil {
		return networkPath{}, fmt.Errorf("search propagation paths: %w", err)
	}

	paths, _ := renderer.rankPaths(tree, receiverGroup)
	for _, path := range paths {
		if len(path.portals) > 0 {
			return path, nil
		}
	}

	return networkPath{}, errors.New(
		"no propagation path reaches the receiver's room group, so its modes have nothing to excite them",
	)
}

// lowFreqPressureGain converts a path's accumulated energy transmission into a
// pressure gain, read from the lowest band because that is the band the modal
// solve stands in for.
func lowFreqPressureGain(path networkPath) float64 {
	if len(path.transmission) == 0 {
		return 1
	}

	return math.Sqrt(math.Max(path.transmission[0], 0))
}
