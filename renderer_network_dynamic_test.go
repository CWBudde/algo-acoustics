package algoacoustics

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-acoustics/hrtf"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
)

func dynamicTestConfig() NetworkRendererConfig {
	cfg := networkTestConfig()
	cfg.ISM = ism.ISMConfig{MaxOrder: 1}

	return cfg
}

// dynamicRenderer builds a renderer with a working ray-tracing configuration,
// which the dense render needs even when only the early field is compared.
func dynamicRenderer(floorDB float64) *NetworkRenderer {
	cfg := dynamicTestConfig()
	cfg.BandFloorDB = floorDB

	return NewNetworkRenderer(cfg)
}

// TestNetworkPlanApplyInvalidatesOnlyTheMergedGroup pins the property the whole
// dynamic path rests on: opening one door must re-simulate only the group that
// changed, leaving every other group in the building warm.
func TestNetworkPlanApplyInvalidatesOnlyTheMergedGroup(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 4, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	renderer := dynamicRenderer(-120)

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	set, err := plan.Apply(PortalStateChange{PortalIndex: 0, State: scene.PortalOpen, Aperture: 1})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Rooms 0 and 1 merge into one new group; rooms 2 and 3 are untouched.
	if len(set.InvalidatedSignatures) != 1 {
		t.Fatalf("invalidated %d groups, want exactly the merged one", len(set.InvalidatedSignatures))
	}

	if set.RecomputedFactors != 1 {
		t.Fatalf("recomputed %d groups, want 1", set.RecomputedFactors)
	}

	if set.ReusedFactors < 2 {
		t.Fatalf("reused %d groups, want the two untouched rooms to stay warm", set.ReusedFactors)
	}
}

// TestNetworkPlanApplyKeepsDistantGroupsWarm is the sharper version: a portal
// at one end of the building must not disturb the group at the other end.
func TestNetworkPlanApplyKeepsDistantGroupsWarm(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 4, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	renderer := dynamicRenderer(-120)

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	before := plan.signatures

	var farthest uint64

	for id, signature := range before {
		rooms, _ := plan.plan.graph.GroupRooms(id)
		if len(rooms) == 1 && rooms[0] == 3 {
			farthest = signature
		}
	}

	if farthest == 0 {
		t.Fatal("could not identify the farthest room's group")
	}

	_, err = plan.Apply(PortalStateChange{PortalIndex: 0, State: scene.PortalOpen, Aperture: 1})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	found := false

	for _, signature := range plan.signatures {
		if signature == farthest {
			found = true
		}
	}

	if !found {
		t.Fatal("the farthest room's signature changed even though nothing about it did")
	}
}

// TestNetworkPlanDynamicMatchesColdRender is the correctness guard for the
// whole incremental path: after a sequence of toggles, the plan's output must
// equal a render built from scratch in the same portal configuration.
func TestNetworkPlanDynamicMatchesColdRender(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 3, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	renderer := dynamicRenderer(-120)

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	toggles := []PortalStateChange{
		{PortalIndex: 0, State: scene.PortalOpen, Aperture: 1},
		{PortalIndex: 0, State: scene.PortalClosed},
		{PortalIndex: 1, State: scene.PortalOpen, Aperture: 1},
	}

	for index, change := range toggles {
		_, err = plan.Apply(change)
		if err != nil {
			t.Fatalf("Apply %d: %v", index, err)
		}
	}

	warm, err := plan.RenderMono()
	if err != nil {
		t.Fatalf("RenderMono (warm): %v", err)
	}

	// Rebuild the same configuration from scratch.
	cold := chainRoomScene(t, 3, 0.25)
	cold.Portals[1].State = scene.PortalOpen

	coldBuffer, err := renderer.RenderMono(cold, cfg)
	if err != nil {
		t.Fatalf("RenderMono (cold): %v", err)
	}

	if warm.Len() != coldBuffer.Len() {
		t.Fatalf("warm length %d, cold %d", warm.Len(), coldBuffer.Len())
	}

	for index := range coldBuffer.Samples {
		if math.Abs(warm.Samples[index]-coldBuffer.Samples[index]) > 1e-9 {
			t.Fatalf("sample %d: warm %v, cold %v", index, warm.Samples[index], coldBuffer.Samples[index])
		}
	}
}

func TestNetworkPlanApplyRejectsAnUnknownPortal(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 2, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	renderer := dynamicRenderer(0)

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	_, err = plan.Apply(PortalStateChange{PortalIndex: 42, State: scene.PortalOpen})
	if err == nil {
		t.Fatal("Apply accepted a portal index that does not exist")
	}
}

// TestPrepareDoesNotMutateTheCallersScene guards against a plan quietly
// rewriting portal state the caller still owns.
func TestPrepareDoesNotMutateTheCallersScene(t *testing.T) {
	t.Parallel()

	sc := chainRoomScene(t, 2, 0.25)
	cfg := ir.RenderConfig{SampleRate: sc.SampleRate, DurationSeconds: 0.2, BandSpec: sc.BandSpec}

	renderer := dynamicRenderer(0)

	plan, err := renderer.Prepare(sc, cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	_, err = plan.Apply(PortalStateChange{PortalIndex: 0, State: scene.PortalOpen, Aperture: 1})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if sc.Portals[0].State != scene.PortalClosed {
		t.Fatal("Apply mutated the caller's scene")
	}
}

// TestPortalCacheRendersAllThreeStates pins the Phase 25.4 headline: the open
// endpoint is a real merged-geometry render, not the tau = 1 all-pass stand-in.
func TestPortalCacheRendersAllThreeStates(t *testing.T) {
	t.Parallel()

	sc := transmissionTestScene(0.25)
	sc.Receivers[0].HRTF = hrtf.NoopDataset{SampleRateHz: sc.SampleRate}
	cfg := transmissionTestRenderConfig(sc)

	renderer := NewNetworkRenderer(dynamicTestConfig())

	cache, err := renderer.PortalCache(sc, sc.Receivers[0], cfg, 0)
	if err != nil {
		t.Fatalf("PortalCache: %v", err)
	}

	closed, err := cache.AtApertureMerged(0, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(0): %v", err)
	}

	merged, err := cache.AtApertureMerged(1, 2)
	if err != nil {
		t.Fatalf("AtApertureMerged(1): %v", err)
	}

	if bufferEnergy(closed.Left) <= 0 || bufferEnergy(merged.Left) <= 0 {
		t.Fatal("a cached portal endpoint carries no energy")
	}

	// The open endpoint must be louder than the closed one: the partition is
	// gone rather than merely transmissive.
	if bufferEnergy(merged.Left) <= bufferEnergy(closed.Left) {
		t.Fatalf("merged energy %v is not above closed %v",
			bufferEnergy(merged.Left), bufferEnergy(closed.Left))
	}

	// The all-pass endpoint reached through AtAperture must differ from the
	// merged one; if they were identical the third state would be pointless.
	allPass, err := cache.AtAperture(1, 2)
	if err != nil {
		t.Fatalf("AtAperture(1): %v", err)
	}

	if buffersMatch(allPass.Left, merged.Left) {
		t.Fatal("the all-pass and merged responses are identical; the merged render is not being used")
	}
}

func bufferEnergy(buffer *ir.Buffer) float64 {
	total := 0.0
	for _, sample := range buffer.Samples {
		total += sample * sample
	}

	return total
}

func buffersMatch(first, second *ir.Buffer) bool {
	if first.Len() != second.Len() {
		return false
	}

	for index := range first.Samples {
		if math.Abs(first.Samples[index]-second.Samples[index]) > 1e-12 {
			return false
		}
	}

	return true
}

// TestCrossRoomEngineSelectionFollowsPortalState pins the rule that gives the
// browser demo a genuinely merged open endpoint without any demo-side change.
//
// The Phase 21 renderer models an open portal as a fully transmissive
// partition, tau = 1, with the two rooms still geometrically separate. Only the
// scene graph merges the volumes into one cavity, so an open portal must always
// route to the filter network.
func TestCrossRoomEngineSelectionFollowsPortalState(t *testing.T) {
	t.Parallel()

	closedEngine := NewCrossRoomEngine(transmissionTestScene(0.25), CrossRoomEngineConfig{})
	if _, ok := closedEngine.(*TransmissionRenderer); !ok {
		t.Fatalf("a closed portal selected %T, want the Phase 21 fast path", closedEngine)
	}

	open := transmissionTestScene(0.25)
	open.Portals[0].State = scene.PortalOpen

	openEngine := NewCrossRoomEngine(open, CrossRoomEngineConfig{})
	if _, ok := openEngine.(*NetworkRenderer); !ok {
		t.Fatalf("an open portal selected %T, want the filter network", openEngine)
	}

	// A portal chain is beyond the Phase 21 shape whatever its state.
	chained := NewCrossRoomEngine(chainRoomScene(t, 3, 0.25), CrossRoomEngineConfig{})
	if _, ok := chained.(*NetworkRenderer); !ok {
		t.Fatalf("a portal chain selected %T, want the filter network", chained)
	}
}
