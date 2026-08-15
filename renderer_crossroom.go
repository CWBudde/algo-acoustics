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
		return NewTransmissionRenderer(TransmissionRendererConfig(cfg))
	}

	return NewNetworkRenderer(NetworkRendererConfig{
		ISM:      cfg.ISM,
		Raytrace: cfg.Raytrace,
		Hybrid:   cfg.Hybrid,
	})
}

// sceneMatchesOneHopTransmission reports whether the Phase 21 renderer can
// handle a scene: one source and one receiver, in two different shoebox rooms
// that a portal joins directly.
func sceneMatchesOneHopTransmission(sc *scene.Scene) bool {
	if sc == nil || sc.RoomCount() < 2 || len(sc.Sources) != 1 || len(sc.Receivers) != 1 {
		return false
	}

	sourceRoom, ok := sc.RoomIndexAt(sc.Sources[0].Position)
	if !ok {
		return false
	}

	receiverRoom, ok := sc.RoomIndexAt(sc.Receivers[0].Position)
	if !ok || sourceRoom == receiverRoom {
		return false
	}

	for _, roomIndex := range []int{sourceRoom, receiverRoom} {
		room, found := sc.RoomAt(roomIndex)
		if !found || room.Kind != scene.RoomKindShoebox || room.Shoebox == nil {
			return false
		}
	}

	for _, portal := range sc.Portals {
		if portal.RoomIndices == [2]int{sourceRoom, receiverRoom} ||
			portal.RoomIndices == [2]int{receiverRoom, sourceRoom} {
			return true
		}
	}

	return false
}
