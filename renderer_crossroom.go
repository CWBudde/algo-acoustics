package algoacoustics

import (
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
)

// TransmissionEarlyEngine is the optional ability to emit the sparse early
// events of a cross-room render, which the CLI and pipeline use for event
// dumps. Both cross-room engines implement it.
type TransmissionEarlyEngine interface {
	SolveEarly(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error)
}

// CrossRoomLateEngine exposes the late field on its own, without the early
// field folded in. CrossRoomEngine.RenderMono and RenderBinaural return the
// complete hybrid response, so callers that assemble the crossover themselves
// need these instead. Both cross-room engines implement it.
type CrossRoomLateEngine interface {
	RenderLateMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error)
	RenderLateBinaural(sc *scene.Scene, receiver scene.Receiver, cfg ir.RenderConfig) (left, right *ir.Buffer, err error)
}

// CrossRoomEngineConfig gathers the settings shared by both cross-room engines.
type CrossRoomEngineConfig struct {
	ISM      ism.ISMConfig
	Raytrace RaytraceEngineConfig
	Hybrid   hybrid.HybridConfig
	// OnTruncation reports a filter-network render that is not exhaustive. The
	// Phase 21 one-hop renderer never truncates, so it ignores this.
	OnTruncation func(NetworkTruncation)
}

// NewCrossRoomEngine picks the cross-room engine that suits a scene.
//
// The Phase 21 TransmissionRenderer is chosen for exactly the shape it was
// built for — one source and one receiver in two directly adjacent shoebox
// rooms, joined by portals between that same pair — so its output stays
// bit-identical wherever it already applied. Everything else, portal chains
// above all, goes to the filter network.
func NewCrossRoomEngine(sc *scene.Scene, cfg CrossRoomEngineConfig) CrossRoomEngine {
	if sceneMatchesOneHopTransmission(sc) {
		return NewTransmissionRenderer(TransmissionRendererConfig{
			ISM:      cfg.ISM,
			Raytrace: cfg.Raytrace,
			Hybrid:   cfg.Hybrid,
		})
	}

	return NewNetworkRenderer(NetworkRendererConfig{
		ISM:          cfg.ISM,
		Raytrace:     cfg.Raytrace,
		Hybrid:       cfg.Hybrid,
		OnTruncation: cfg.OnTruncation,
	})
}

// sceneMatchesOneHopTransmission reports whether the Phase 21 renderer can
// handle a scene: one source and one receiver, in two different shoebox rooms
// that a portal joins directly.
//
// The room count must be exactly two. TransmissionRenderer collects only the
// portals joining the source and receiver rooms, so a third room would have its
// flanking paths dropped without a trace even when a direct portal also exists.
// Anything above two rooms therefore belongs to the filter network.
func sceneMatchesOneHopTransmission(sc *scene.Scene) bool {
	sourceRoom, receiverRoom, ok := oneHopRoomPair(sc)
	if !ok {
		return false
	}

	connected := false

	for _, portal := range sc.Portals {
		joinsThePair := portal.RoomIndices == [2]int{sourceRoom, receiverRoom} ||
			portal.RoomIndices == [2]int{receiverRoom, sourceRoom}
		if !joinsThePair {
			continue
		}

		// An open portal must go to the filter network. The Phase 21 renderer
		// models "open" as a fully transmissive partition, tau = 1, with both
		// rooms still geometrically separate; only the scene graph physically
		// merges the two volumes into one cavity, which is a different — and
		// correct — response.
		if portal.State == scene.PortalOpen {
			return false
		}

		connected = true
	}

	return connected
}

// oneHopRoomPair returns the source and receiver rooms when the scene has the
// single-source, single-receiver, two-shoebox shape the Phase 21 renderer needs.
func oneHopRoomPair(sc *scene.Scene) (sourceRoom, receiverRoom int, ok bool) {
	if sc == nil || sc.RoomCount() != 2 || len(sc.Sources) != 1 || len(sc.Receivers) != 1 {
		return 0, 0, false
	}

	sourceRoom, ok = sc.RoomIndexAt(sc.Sources[0].Position)
	if !ok {
		return 0, 0, false
	}

	receiverRoom, ok = sc.RoomIndexAt(sc.Receivers[0].Position)
	if !ok || sourceRoom == receiverRoom {
		return 0, 0, false
	}

	for _, roomIndex := range []int{sourceRoom, receiverRoom} {
		room, found := sc.RoomAt(roomIndex)
		if !found || room.Kind != scene.RoomKindShoebox || room.Shoebox == nil {
			return 0, 0, false
		}
	}

	return sourceRoom, receiverRoom, true
}
