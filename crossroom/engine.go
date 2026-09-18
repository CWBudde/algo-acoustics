package crossroom

import (
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/raytrace"
	"github.com/cwbudde/algo-acoustics/scene"
)

// Engine renders a source and receiver separated by one or more
// portals. Its method set is identical to the root package's
// BinauralLateBufferEngine, which it is declared separately from so that this
// package stays free of a dependency on the renderer that consumes it; Go
// satisfies both structurally.
type Engine interface {
	RenderMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error)
	RenderBinaural(sc *scene.Scene, receiver scene.Receiver, cfg ir.RenderConfig) (left, right *ir.Buffer, err error)
}

// EarlyEngine is the optional ability to emit the sparse early
// events of a cross-room render, which the CLI and pipeline use for event
// dumps. Both cross-room engines implement it.
type EarlyEngine interface {
	SolveEarly(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error)
}

// LateEngine exposes the late field on its own, without the early
// field folded in. Engine.RenderMono and RenderBinaural return the
// complete hybrid response, so callers that assemble the crossover themselves
// need these instead. Both cross-room engines implement it.
type LateEngine interface {
	RenderLateMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error)
	RenderLateBinaural(sc *scene.Scene, receiver scene.Receiver, cfg ir.RenderConfig) (left, right *ir.Buffer, err error)
}

// EngineConfig gathers the settings shared by both cross-room engines.
type EngineConfig struct {
	ISM      ism.ISMConfig
	Raytrace raytrace.EngineConfig
	Hybrid   hybrid.HybridConfig
	// OnTruncation reports a filter-network render that is not exhaustive. The
	// Phase 21 one-hop renderer never truncates, so it ignores this.
	OnTruncation func(Truncation)
}

// NewEngine picks the cross-room engine that suits a scene.
//
// The Phase 21 OneHop is chosen for exactly the shape it was
// built for — one source and one receiver in two directly adjacent shoebox
// rooms, joined by portals between that same pair — so its output stays
// bit-identical wherever it already applied. Everything else, portal chains
// above all, goes to the filter network.
func NewEngine(sc *scene.Scene, cfg EngineConfig) Engine {
	if matchesOneHop(sc) {
		return NewOneHop(OneHopConfig{
			ISM:      cfg.ISM,
			Raytrace: cfg.Raytrace,
			Hybrid:   cfg.Hybrid,
		})
	}

	return NewNetwork(NetworkConfig{
		ISM:          cfg.ISM,
		Raytrace:     cfg.Raytrace,
		Hybrid:       cfg.Hybrid,
		OnTruncation: cfg.OnTruncation,
	})
}

// matchesOneHop reports whether the Phase 21 renderer can
// handle a scene: one source and one receiver, in two different shoebox rooms
// that a portal joins directly.
//
// The room count must be exactly two. OneHop collects only the
// portals joining the source and receiver rooms, so a third room would have its
// flanking paths dropped without a trace even when a direct portal also exists.
// Anything above two rooms therefore belongs to the filter network.
func matchesOneHop(sc *scene.Scene) bool {
	sourceRoom, receiverRoom, ok := oneHopRoomPair(sc)
	if !ok {
		return false
	}

	connected := false

	for _, portal := range sc.Portals {
		// Any open portal sends the scene to the filter network. The Phase 21
		// renderer models "open" as a fully transmissive partition, tau = 1,
		// with both rooms still geometrically separate; only the scene graph
		// merges the two volumes into the single cavity that an open door
		// actually is.
		if portal.State == scene.PortalOpen {
			return false
		}

		if portal.RoomIndices == [2]int{sourceRoom, receiverRoom} ||
			portal.RoomIndices == [2]int{receiverRoom, sourceRoom} {
			connected = true
		}
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
