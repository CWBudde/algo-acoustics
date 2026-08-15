package algoacoustics

import (
	"errors"
	"fmt"
	"sort"
	"strings"

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
	// MaxComposedEventsPerPath bounds the sparse event expansion of one path,
	// which is combinatorial in the number of hops; zero selects
	// defaultMaxComposedEventsPerPath and a negative value removes the cap.
	// Removing it is safe only for shallow topologies: the expansion is the
	// cartesian product of the hops' events.
	MaxComposedEventsPerPath int
	// BandFloorDB is the level below which a band is skipped; zero selects
	// hybrid.DefaultBandFloorDB.
	BandFloorDB float64
	// DynamicRays overrides the ray count for interactive re-simulation; zero
	// keeps the Raytrace launch count.
	DynamicRays int
	// Seed makes the stochastic synthesis reproducible; zero selects 1.
	Seed int64
	// OnTruncation reports every way in which a render fell short of an
	// exhaustive one. A truncated render still produces plausible output, so a
	// caller that does not observe this has no other symptom to go by.
	OnTruncation func(NetworkTruncation)
}

// NetworkTruncation records how far a render fell short of exhaustive.
//
// A truncated path search or a capped event expansion leaves the output looking
// entirely reasonable while quietly omitting flanking paths or reflections,
// which is why the renderer reports it rather than swallowing it.
type NetworkTruncation struct {
	// PathSearch reports that the group-graph search hit its depth, node, or
	// prune-floor limit, so paths beyond it were never enumerated.
	PathSearch bool
	// PathsFound is how many paths reached the receiver group.
	PathsFound int
	// PathsRendered is how many of those survived the MaxPaths cap.
	PathsRendered int
	// EventsDropped counts sparse events discarded by the per-path expansion
	// cap. The relative level floor is not counted: it discards only what is
	// inaudible, whereas the cap discards whatever is left over.
	EventsDropped int
}

// Truncated reports whether anything was lost.
func (t NetworkTruncation) Truncated() bool {
	return t.PathSearch || t.PathsRendered < t.PathsFound || t.EventsDropped > 0
}

// String renders the truncation as a single human-readable warning line.
func (t NetworkTruncation) String() string {
	var reasons []string

	if t.PathSearch {
		reasons = append(reasons, "the propagation path search hit its depth or node limit")
	}

	if t.PathsRendered < t.PathsFound {
		reasons = append(reasons, fmt.Sprintf("only %d of %d paths were rendered", t.PathsRendered, t.PathsFound))
	}

	if t.EventsDropped > 0 {
		reasons = append(reasons, fmt.Sprintf("%d composed early events were dropped", t.EventsDropped))
	}

	if len(reasons) == 0 {
		return "the multi-room render is exhaustive"
	}

	return "the multi-room render is not exhaustive: " + strings.Join(reasons, "; ")
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

	truncation NetworkTruncation

	// factors memoises hops by their endpoint identity rather than by path, so
	// a hop that several paths share is simulated once. Paths through a
	// building routinely reuse the same room group between the same two
	// portals, and each such hop costs a full ISM solve and ray trace.
	factors map[hopKey]cachedFactor
}

// hopKey identifies one hop by its endpoints: the room group it crosses and the
// portals it enters and leaves by. Entry is portalNone for the hop starting at
// the primary source, exit is portalNone for the hop ending at the receiver.
//
// A group plus an entry portal fixes the direction of travel, because a portal
// joins exactly two groups, so these three values determine the simulation
// completely.
type hopKey struct {
	group scene.GroupID
	entry int
	exit  int
}

// portalNone marks a hop endpoint that is the real source or receiver rather
// than a portal.
const portalNone = -1

type cachedFactor struct {
	factor *GroupFactor
	needs  factorNeeds
}

// factorNeeds selects which fields of a GroupFactor a caller actually uses.
// Tracing the late field costs a full ray trace, so a caller that only wants
// the early events must be able to say so.
type factorNeeds struct {
	early bool
	late  bool
}

func (n factorNeeds) union(other factorNeeds) factorNeeds {
	return factorNeeds{early: n.early || other.early, late: n.late || other.late}
}

// missing returns what a hop still owes to satisfy want.
func (n factorNeeds) missing(want factorNeeds) factorNeeds {
	return factorNeeds{early: want.early && !n.early, late: want.late && !n.late}
}

func (n factorNeeds) empty() bool {
	return !n.early && !n.late
}

// hopKeyAt derives the cache key of one hop of a path.
func hopKeyAt(path networkPath, index int) hopKey {
	key := hopKey{group: path.groups[index], entry: portalNone, exit: portalNone}
	if index > 0 {
		key.entry = path.portals[index-1].PortalIndex
	}

	if index < len(path.groups)-1 {
		key.exit = path.portals[index].PortalIndex
	}

	return key
}

func (p *networkPlan) storeFactor(key hopKey, factor *GroupFactor, needs factorNeeds) {
	if p.factors == nil {
		p.factors = make(map[hopKey]cachedFactor)
	}

	p.factors[key] = cachedFactor{factor: factor, needs: needs}
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

	events, err := r.solveEarlyEvents(plan, cfg)
	if err != nil {
		return nil, err
	}

	r.reportTruncation(plan)

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

	r.reportTruncation(plan)

	return combined, nil
}

// RenderLateMono renders only the summed late field.
//
// It traces the late field alone. Solving the early field here would be pure
// waste, and doubly so for the CLI hybrid path, which asks for the early field
// separately.
func (r *NetworkRenderer) RenderLateMono(sc *scene.Scene, cfg ir.RenderConfig) (*ir.Buffer, error) {
	plan, err := r.prepare(sc)
	if err != nil {
		return nil, err
	}

	late, err := r.renderLatePaths(plan, cfg)
	if err != nil {
		return nil, err
	}

	r.reportTruncation(plan)

	return hybrid.HistogramToBuffer(late, cfg.SampleRate), nil
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

	// One plan serves both fields: preparing a second one would search the
	// paths again and re-simulate every hop the early field already solved.
	events, err := r.solveEarlyEvents(plan, cfg)
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

	lateLeft, lateRight, err := r.renderLateBinauralFromPlan(plan, receiver, cfg)
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

	r.reportTruncation(plan)

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

	left, right, err = r.renderLateBinauralFromPlan(plan, receiver, cfg)
	if err != nil {
		return nil, nil, err
	}

	r.reportTruncation(plan)

	return left, right, nil
}

func (r *NetworkRenderer) renderLateBinauralFromPlan(
	plan *networkPlan,
	receiver scene.Receiver,
	cfg ir.RenderConfig,
) (left, right *ir.Buffer, err error) {
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
		BandSpec:        plan.graph.Scene().BandSpec,
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

// solveEarlyEvents composes the sparse early events of an already-prepared plan,
// so a binaural render reuses its own plan instead of preparing a second one and
// re-simulating every hop.
func (r *NetworkRenderer) solveEarlyEvents(plan *networkPlan, cfg ir.RenderConfig) ([]ir.Event, error) {
	var events []ir.Event

	bandCount := plan.graph.Scene().BandSpec.BandCount()

	for pathIndex, path := range plan.paths {
		factors, err := r.renderPathFactors(plan, pathIndex, cfg, factorNeeds{early: true})
		if err != nil {
			return nil, err
		}

		composed, dropped := composePathEvents(plan.graph.Scene(), path, factors, bandCount, r.maxComposedEventsPerPath())
		plan.truncation.EventsDropped += dropped

		events = append(events, composed...)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeSeconds != events[j].TimeSeconds {
			return events[i].TimeSeconds < events[j].TimeSeconds
		}

		return events[i].DistanceMeters < events[j].DistanceMeters
	})

	return events, nil
}

// reportTruncation hands the accumulated truncation to the configured observer.
func (r *NetworkRenderer) reportTruncation(plan *networkPlan) {
	if r.Config.OnTruncation == nil || !plan.truncation.Truncated() {
		return
	}

	r.Config.OnTruncation(plan.truncation)
}

// prepare builds the scene graph, searches the paths, and ranks them.
func (r *NetworkRenderer) prepare(sc *scene.Scene) (*networkPlan, error) {
	if r == nil {
		return nil, errors.New("network renderer is nil")
	}

	if sc == nil {
		return nil, errors.New("scene is nil")
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

	var found int

	plan.paths, found = r.rankPaths(tree, receiverGroup)
	if len(plan.paths) == 0 {
		return nil, fmt.Errorf(
			"no propagation path from the source reaches the receiver above the %.0f dB floor; "+
				"lower NetworkRendererConfig.BandFloorDB or raise MaxPathHops (currently %d) to include quieter paths",
			r.bandFloorDB(), r.maxPathHops(),
		)
	}

	// The search may return usable leaves while having abandoned other branches
	// at the depth, node, or prune-floor limit. Rendering that tree as if it
	// were exhaustive under-renders a large topology with no other symptom, so
	// the state is carried through to OnTruncation.
	plan.truncation = NetworkTruncation{
		PathSearch:    tree.Truncated,
		PathsFound:    found,
		PathsRendered: len(plan.paths),
	}

	return plan, nil
}

// rankPaths converts the search leaves into renderable paths, strongest first,
// and truncates to MaxPaths. It also returns how many paths reached the
// receiver before that cap applied.
func (r *NetworkRenderer) rankPaths(tree *scene.PathSearchTree, receiverGroup scene.GroupID) ([]networkPath, int) {
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

	found := len(paths)
	if limit := r.maxPaths(); len(paths) > limit {
		paths = paths[:limit]
	}

	return paths, found
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

func (r *NetworkRenderer) maxComposedEventsPerPath() int {
	if r.Config.MaxComposedEventsPerPath != 0 {
		return r.Config.MaxComposedEventsPerPath
	}

	return defaultMaxComposedEventsPerPath
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
