package algoacoustics

import (
	"errors"
	"fmt"
	"sort"

	"github.com/cwbudde/algo-acoustics/geometry"
	"github.com/cwbudde/algo-acoustics/hybrid"
	"github.com/cwbudde/algo-acoustics/ir"
	"github.com/cwbudde/algo-acoustics/ism"
	"github.com/cwbudde/algo-acoustics/scene"
)

const (
	defaultMaxPathHops = 4
	defaultMaxPaths    = 32
)

// NetworkRendererConfig configures the multi-room filter network.
type NetworkRendererConfig struct {
	ISM      ism.ISMConfig
	Raytrace RaytraceEngineConfig
	Hybrid   hybrid.HybridConfig
	// MaxPathHops bounds the portal traversals on a path; zero selects 4.
	MaxPathHops int
	// MaxPaths bounds how many paths are rendered, strongest first; zero
	// selects 32. Convolution assembly is the dominant cost, so this is the
	// main guard against a topology with many weak flanking paths.
	MaxPaths int
	// BandFloorDB is the level below which a band is skipped; zero selects
	// hybrid.DefaultBandFloorDB.
	BandFloorDB float64
	// DynamicRays overrides the ray count for interactive re-simulation; zero
	// keeps the Raytrace launch count.
	DynamicRays int
	// Seed makes the stochastic synthesis reproducible; zero selects 1.
	Seed int64
}

// NetworkRenderer renders multi-room propagation as the filter network of
// docs/raven.md section 5.2, composing each path as a product of separately
// simulated room-group transfer functions.
//
// It supersedes TransmissionRenderer for anything beyond the Phase 21 shape of
// two adjacent shoebox rooms joined by one portal pair. That renderer remains
// the fast path for exactly that shape and is left untouched, so its output is
// unchanged.
//
// Composition is by per-band convolution rather than by re-emitting events.
// Re-emission costs a complete solve per event per hop, which is exponential in
// the number of hops; convolution costs one simulation per hop regardless of
// how many events arrived.
type NetworkRenderer struct {
	Config NetworkRendererConfig
}

// NewNetworkRenderer constructs the multi-room filter-network renderer.
func NewNetworkRenderer(cfg NetworkRendererConfig) *NetworkRenderer {
	return &NetworkRenderer{Config: cfg}
}

// networkPlan holds the resolved graph, paths, and endpoints of one render.
type networkPlan struct {
	graph    *scene.AcousticSceneGraph
	tree     *scene.PathSearchTree
	paths    []networkPath
	source   scene.Source
	receiver scene.Receiver

	// factorCache memoises the per-hop factors of each path, so a render that
	// needs both the early and the late field simulates each hop once.
	factorCache []cachedFactors
}

type cachedFactors struct {
	factors []*GroupFactor
	hasLate bool
}

func (p *networkPlan) cachedFactors(pathIndex int, needLate bool) []*GroupFactor {
	if pathIndex >= len(p.factorCache) {
		return nil
	}

	entry := p.factorCache[pathIndex]
	if entry.factors == nil || (needLate && !entry.hasLate) {
		return nil
	}

	return entry.factors
}

func (p *networkPlan) storeFactors(pathIndex int, factors []*GroupFactor, hasLate bool) {
	for len(p.factorCache) <= pathIndex {
		p.factorCache = append(p.factorCache, cachedFactors{})
	}

	p.factorCache[pathIndex] = cachedFactors{factors: factors, hasLate: hasLate}
}

// networkPath is one propagation path reduced to the hops the renderer walks.
type networkPath struct {
	// groups lists the room groups in order, source group first.
	groups []scene.GroupID
	// portals lists the traversed portals, one fewer than groups.
	portals []scene.GroupPortalView
	// transmission is the accumulated per-band coefficient, used for ranking.
	transmission []float64
	activeBands  []bool
}

// SolveEarly returns the early events arriving at the receiver, summed across
// paths. It exists so the renderer can stand in for the Phase 21 engine where
// callers consume events directly.
func (r *NetworkRenderer) SolveEarly(sc *scene.Scene, cfg ir.RenderConfig) ([]ir.Event, error) {
	plan, err := r.prepare(sc)
	if err != nil {
		return nil, err
	}

	var events []ir.Event

	bandCount := sc.BandSpec.BandCount()

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, false)
		if err != nil {
			return nil, err
		}

		events = append(events, composePathEvents(plan.graph.Scene(), path, factors, bandCount)...)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}

// RenderMono renders the summed multi-room hybrid response.
func (r *NetworkRenderer) RenderMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	plan, err := r.prepare(sc)
	if err != nil {
		return nil, err
	}

	early, late, err := r.renderPaths(plan, cfg)
	if err != nil {
		return nil, err
	}

	// One global alignment on the summed fields, never per path: per-path
	// early-to-late ratios are physically meaningful and aligning each path
	// separately would flatten what makes flanking audible.
	late = hybrid.AlignLateTailBuffer(late, early, r.Config.Hybrid)

	combined := hybrid.CombineBuffers(early, late, r.Config.Hybrid)
	if combined == nil {
		return nil, errors.New("combine multi-room mono field")
	}

	return combined, nil
}

// RenderLateMono renders only the summed late field.
func (r *NetworkRenderer) RenderLateMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	plan, err := r.prepare(sc)
	if err != nil {
		return nil, err
	}

	_, late, err := r.renderPaths(plan, cfg)

	return late, err
}

// RenderBinaural renders the summed multi-room hybrid BRIR.
func (r *NetworkRenderer) RenderBinaural(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
	if receiver.HRTF == nil {
		return nil, nil, errors.New("binaural receiver is missing an HRTF")
	}

	plan, err := r.prepare(sc)
	if err != nil {
		return nil, nil, err
	}

	plan.receiver = receiver

	events, err := r.SolveEarly(sc, cfg)
	if err != nil {
		return nil, nil, err
	}

	headEvents := append([]ir.Event(nil), events...)
	for index := range headEvents {
		headEvents[index].Direction = receiver.WorldToHeadDir(headEvents[index].Direction)
	}

	earlyLeft, earlyRight, err := ir.RenderBinaural(headEvents, receiver.HRTF, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("render multi-room binaural early field: %w", err)
	}

	lateLeft, lateRight, err := r.RenderLateBinaural(sc, receiver, cfg)
	if err != nil {
		return nil, nil, err
	}

	lateLeft = hybrid.AlignLateTail(lateLeft, events, r.Config.Hybrid)
	lateRight = hybrid.AlignLateTail(lateRight, events, r.Config.Hybrid)
	left = hybrid.CombineBuffers(earlyLeft, lateLeft, r.Config.Hybrid)
	right = hybrid.CombineBuffers(earlyRight, lateRight, r.Config.Hybrid)

	if left == nil || right == nil {
		return nil, nil, errors.New("combine multi-room binaural field")
	}

	return left, right, nil
}

// RenderLateBinaural renders the summed directional late field.
func (r *NetworkRenderer) RenderLateBinaural(
	sc *scene.Scene,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
	if receiver.HRTF == nil {
		return nil, nil, errors.New("binaural receiver is missing an HRTF")
	}

	plan, err := r.prepare(sc)
	if err != nil {
		return nil, nil, err
	}

	plan.receiver = receiver

	group, ok := plan.graph.GroupOfPosition(receiver.Position)
	if !ok {
		return nil, nil, errors.New("receiver must belong to exactly one room group")
	}

	volume, err := groupVolume(plan.graph, group)
	if err != nil {
		return nil, nil, err
	}

	summed, directions, probabilities, err := r.sumLateHistograms(plan, cfg)
	if err != nil {
		return nil, nil, err
	}

	bins := make([]ir.EnergyBin, len(summed.Bins))
	for index, bin := range summed.Bins {
		bins[index] = ir.EnergyBin{TimeSeconds: bin.TimeSeconds, BandEnergy: append([]float64(nil), bin.BandEnergy...)}
	}

	left, right, err = ir.RenderBinauralPoisson(ir.BinauralPoissonConfig{
		Bins:            bins,
		BinDuration:     summed.BinDuration,
		Volume:          volume,
		BandSpec:        sc.BandSpec,
		SampleRate:      cfg.SampleRate,
		HRTF:            receiver.HRTF,
		DGDirections:    directions,
		DGProbabilities: probabilities,
	}, r.networkRandom())
	if err != nil {
		return nil, nil, fmt.Errorf("render multi-room binaural late field: %w", err)
	}

	return left, right, nil
}

// prepare builds the scene graph, searches the paths, and ranks them.
func (r *NetworkRenderer) prepare(sc *scene.Scene) (*networkPlan, error) {
	if r == nil {
		return nil, errors.New("network renderer is nil")
	}

	if len(sc.Sources) != 1 {
		return nil, fmt.Errorf("multi-room rendering requires exactly one source, got %d", len(sc.Sources))
	}

	// raytrace.TraceSecondary accepts exactly one receiver, so several
	// receivers would need one full render each.
	if len(sc.Receivers) != 1 {
		return nil, fmt.Errorf(
			"multi-room rendering requires exactly one receiver, got %d: the ray tracer detects one receiver per trace",
			len(sc.Receivers),
		)
	}

	graph, err := scene.NewAcousticSceneGraph(sc)
	if err != nil {
		return nil, fmt.Errorf("build scene graph: %w", err)
	}

	sourceGroup, ok := graph.GroupOfPosition(sc.Sources[0].Position)
	if !ok {
		return nil, errors.New("source must belong to exactly one room group")
	}

	receiverGroup, ok := graph.GroupOfPosition(sc.Receivers[0].Position)
	if !ok {
		return nil, errors.New("receiver must belong to exactly one room group")
	}

	tree, err := graph.SearchPaths(sourceGroup, scene.PathSearchConfig{
		MaxDepth:     r.maxPathHops(),
		PruneFloorDB: r.bandFloorDB(),
		BandCount:    sc.BandSpec.BandCount(),
	})
	if err != nil {
		return nil, fmt.Errorf("search propagation paths: %w", err)
	}

	plan := &networkPlan{
		graph:    graph,
		tree:     tree,
		source:   sc.Sources[0],
		receiver: sc.Receivers[0],
	}

	plan.paths = r.rankPaths(tree, receiverGroup)
	if len(plan.paths) == 0 {
		return nil, fmt.Errorf(
			"no propagation path from the source reaches the receiver above the %.0f dB floor; "+
				"lower NetworkRendererConfig.BandFloorDB or raise MaxPathHops (currently %d) to include quieter paths",
			r.bandFloorDB(), r.maxPathHops(),
		)
	}

	return plan, nil
}

// rankPaths converts the search leaves into renderable paths, strongest first,
// and truncates to MaxPaths.
func (r *NetworkRenderer) rankPaths(tree *scene.PathSearchTree, receiverGroup scene.GroupID) []networkPath {
	var paths []networkPath

	for _, leaf := range tree.Leaves {
		if tree.Nodes[leaf].Group != receiverGroup {
			continue
		}

		nodes := tree.PathTo(leaf)
		path := networkPath{
			transmission: tree.Nodes[leaf].Transmission,
			activeBands:  tree.Nodes[leaf].ActiveBands,
		}

		for _, index := range nodes {
			path.groups = append(path.groups, tree.Nodes[index].Group)
			if step := tree.Nodes[index].Step; step != nil {
				path.portals = append(path.portals, step.Portal)
			}
		}

		paths = append(paths, path)
	}

	sort.SliceStable(paths, func(i, j int) bool {
		return pathEnergy(paths[i]) > pathEnergy(paths[j])
	})

	if limit := r.maxPaths(); len(paths) > limit {
		paths = paths[:limit]
	}

	return paths
}

func (r *NetworkRenderer) maxPathHops() int {
	if r.Config.MaxPathHops > 0 {
		return r.Config.MaxPathHops
	}

	return defaultMaxPathHops
}

func (r *NetworkRenderer) maxPaths() int {
	if r.Config.MaxPaths > 0 {
		return r.Config.MaxPaths
	}

	return defaultMaxPaths
}

func (r *NetworkRenderer) bandFloorDB() float64 {
	if r.Config.BandFloorDB != 0 {
		return r.Config.BandFloorDB
	}

	return hybrid.DefaultBandFloorDB
}

func pathEnergy(path networkPath) float64 {
	total := 0.0
	for _, value := range path.transmission {
		total += value
	}

	return total
}

func groupVolume(graph *scene.AcousticSceneGraph, id scene.GroupID) (float64, error) {
	group, err := graph.GroupGeometry(id)
	if err != nil {
		return 0, fmt.Errorf("build group geometry: %w", err)
	}

	if group.Volume <= 0 {
		return 0, fmt.Errorf("group %d has no derivable volume", id)
	}

	return group.Volume, nil
}

// portalPort builds the port on one side of a portal traversal, nudged just
// inside the named group.
func portalPort(sc *scene.Scene, view scene.GroupPortalView, entering bool) groupPort {
	portal := sc.Portals[view.PortalIndex]
	normal := view.Normal(sc)
	centre := portal.Center()

	offset := networkPortalOffsetMeters
	if !entering {
		offset = -networkPortalOffsetMeters
	}

	polygon := append([]geometry.Vec3(nil), portal.Polygon...)
	if !view.Reversed {
		// The surface detector must face the arriving energy, so the polygon
		// is wound against the direction of travel.
		reverseVec3s(polygon)
	}

	return groupPort{
		Kind:     portKindPortal,
		Index:    view.PortalIndex,
		Position: centre.Add(normal.Scale(offset)),
		Polygon:  polygon,
	}
}
