package scene

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
)

const (
	// DefaultPathPruneFloorDB is the accumulated transmission level below which
	// a propagation path is considered inaudible. It matches the -60 dB culling
	// convention the diffraction path search already uses.
	DefaultPathPruneFloorDB = -60.0
	// DefaultMaxPathDepth bounds the number of portal traversals on a path.
	DefaultMaxPathDepth = 6
	// DefaultMaxPathNodes bounds the search tree, guarding against topologies
	// whose simple-path count grows explosively.
	DefaultMaxPathNodes = 100_000
)

// PathSearchConfig tunes the depth-first search across the group graph.
type PathSearchConfig struct {
	// MaxDepth bounds portal traversals; zero selects DefaultMaxPathDepth.
	MaxDepth int
	// MaxNodes bounds the tree size; zero selects DefaultMaxPathNodes.
	MaxNodes int
	// PruneFloorDB is the accumulated transmission floor in dB, a negative
	// number; zero selects DefaultPathPruneFloorDB.
	PruneFloorDB float64
	// BandCount is the number of frequency bands; zero takes the scene's.
	BandCount int
	// ReductionWeights weights the bands when estimating a single-number
	// reduction index; nil selects DefaultReductionWeights.
	ReductionWeights []float64
}

// PathStep is one portal traversal along a propagation path.
type PathStep struct {
	Portal GroupPortalView
	// Transmission holds this portal's coefficient per band.
	Transmission []float64
}

// PathNode is one group residency in the path search tree.
type PathNode struct {
	Group    GroupID
	Depth    int
	Parent   int
	Children []int
	// Step is the portal traversal that reached this node; nil at the root.
	Step *PathStep
	// Transmission is the accumulated per-band coefficient from the root.
	Transmission []float64
	// ActiveBands marks the bands still above the prune floor.
	ActiveBands []bool
	// ReductionDB is the accumulated single-number reduction index estimate.
	ReductionDB float64
	// IsLeaf marks a group holding at least one receiver.
	IsLeaf bool
}

// PathSearchTree is the result of the depth-first search across the group
// graph, the Path Search Tree of docs/raven.md section 10.
type PathSearchTree struct {
	Nodes  []PathNode
	Root   int
	Leaves []int
	// Truncated reports that at least one branch was cut short by the depth,
	// node, or prune-floor limits, so the tree is not exhaustive. Callers must
	// surface this rather than swallow it: a silently truncated search
	// under-renders a large building without any other symptom.
	Truncated bool
}

// PathTo returns the node indices from the root down to node.
func (t *PathSearchTree) PathTo(node int) []int {
	if t == nil || node < 0 || node >= len(t.Nodes) {
		return nil
	}

	var reversed []int

	for current := node; current >= 0; current = t.Nodes[current].Parent {
		reversed = append(reversed, current)
	}

	slices.Reverse(reversed)

	return reversed
}

// PortalSequence returns the portal indices traversed from the root to node.
func (t *PathSearchTree) PortalSequence(node int) []int {
	nodes := t.PathTo(node)

	portals := make([]int, 0, len(nodes))

	for _, index := range nodes {
		if step := t.Nodes[index].Step; step != nil {
			portals = append(portals, step.Portal.PortalIndex)
		}
	}

	return portals
}

// SearchPaths enumerates the propagation paths leading from a source group to
// every group holding a receiver, as a depth-first search across the group
// graph (docs/raven.md section 5.1).
//
// The search runs over groups rather than rooms, so every edge is a closed
// portal: open portals have already been merged into their group. It uses an
// explicit LIFO stack and marks the groups on the current path, which is the
// cycle detection — the search therefore enumerates simple paths only.
func (g *AcousticSceneGraph) SearchPaths(sourceGroup GroupID, cfg PathSearchConfig) (*PathSearchTree, error) {
	if g == nil || g.scene == nil {
		return nil, errors.New("scene graph is nil")
	}

	if _, ok := g.GroupRooms(sourceGroup); !ok {
		return nil, fmt.Errorf("source group %d does not exist", sourceGroup)
	}

	cfg, err := cfg.withDefaults(g.scene.BandSpec.BandCount())
	if err != nil {
		return nil, err
	}

	if cfg.BandCount <= 0 {
		return nil, errors.New("path search requires a positive band count")
	}

	receiverGroups := g.groupsHoldingReceivers()

	unity := make([]float64, cfg.BandCount)
	for index := range unity {
		unity[index] = 1
	}

	tree := &PathSearchTree{}
	tree.Nodes = append(tree.Nodes, PathNode{
		Group:        sourceGroup,
		Parent:       -1,
		Transmission: unity,
		ActiveBands:  activeBands(unity, cfg.PruneFloorDB),
		IsLeaf:       receiverGroups[sourceGroup],
	})

	if tree.Nodes[0].IsLeaf {
		tree.Leaves = append(tree.Leaves, 0)
	}

	g.expandPaths(tree, cfg, receiverGroups)

	return tree, nil
}

// pathFrame is one entry of the explicit depth-first stack. childCursor records
// how many of the frame's outgoing portals have been considered so far.
type pathFrame struct {
	node        int
	edges       []GroupPortalView
	childCursor int
}

func (g *AcousticSceneGraph) expandPaths(tree *PathSearchTree, cfg PathSearchConfig, receiverGroups map[GroupID]bool) {
	onPath := make([]bool, g.GroupCount())
	onPath[tree.Nodes[0].Group] = true

	stack := []pathFrame{{node: 0, edges: g.sortedGroupEdges(tree.Nodes[0].Group)}}

	for len(stack) > 0 {
		frame := &stack[len(stack)-1]

		if frame.childCursor >= len(frame.edges) {
			onPath[tree.Nodes[frame.node].Group] = false
			stack = stack[:len(stack)-1]

			continue
		}

		edge := frame.edges[frame.childCursor]
		frame.childCursor++

		child, ok := g.growPath(tree, cfg, frame.node, edge, onPath, receiverGroups)
		if !ok {
			continue
		}

		onPath[tree.Nodes[child].Group] = true
		stack = append(stack, pathFrame{node: child, edges: g.sortedGroupEdges(tree.Nodes[child].Group)})
	}
}

// growPath appends a child node for one portal traversal, or reports that the
// branch was rejected. Every rejection other than a cycle marks the tree as
// truncated.
func (g *AcousticSceneGraph) growPath(
	tree *PathSearchTree,
	cfg PathSearchConfig,
	parent int,
	edge GroupPortalView,
	onPath []bool,
	receiverGroups map[GroupID]bool,
) (int, bool) {
	if onPath[edge.ToGroup] {
		// Revisiting a group on the current path would be a cycle. Simple
		// paths only, so this is not a truncation.
		return 0, false
	}

	if tree.Nodes[parent].Depth+1 > cfg.MaxDepth || len(tree.Nodes) >= cfg.MaxNodes {
		tree.Truncated = true

		return 0, false
	}

	portal := g.scene.Portals[edge.PortalIndex]
	transmission := make([]float64, cfg.BandCount)
	accumulated := make([]float64, cfg.BandCount)

	for band := range transmission {
		transmission[band] = portal.TransmissionAt(g.scene.Materials, band)
		accumulated[band] = tree.Nodes[parent].Transmission[band] * transmission[band]
	}

	active := activeBands(accumulated, cfg.PruneFloorDB)
	if !anyActive(active) {
		tree.Truncated = true

		return 0, false
	}

	// The per-band mask above is the only pruning criterion. The weighted
	// reduction index is recorded but deliberately not used to reject a branch:
	// for a spectrally selective portal chain the normalised weighting can drag
	// the aggregate below the floor while a band is still audible, and dropping
	// the branch would discard that surviving contribution.
	reduction := WeightedReductionIndexDB(accumulated, cfg.ReductionWeights)

	child := len(tree.Nodes)
	tree.Nodes = append(tree.Nodes, PathNode{
		Group:        edge.ToGroup,
		Depth:        tree.Nodes[parent].Depth + 1,
		Parent:       parent,
		Step:         &PathStep{Portal: edge, Transmission: transmission},
		Transmission: accumulated,
		ActiveBands:  active,
		ReductionDB:  reduction,
		IsLeaf:       receiverGroups[edge.ToGroup],
	})
	tree.Nodes[parent].Children = append(tree.Nodes[parent].Children, child)

	if tree.Nodes[child].IsLeaf {
		tree.Leaves = append(tree.Leaves, child)
	}

	return child, true
}

// sortedGroupEdges returns a group's outgoing closed portals in a stable order,
// so the search visits branches identically across runs.
func (g *AcousticSceneGraph) sortedGroupEdges(from GroupID) []GroupPortalView {
	edges := g.GroupPortalViews(from)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].PortalIndex != edges[j].PortalIndex {
			return edges[i].PortalIndex < edges[j].PortalIndex
		}

		return edges[i].FromRoom < edges[j].FromRoom
	})

	return edges
}

func (g *AcousticSceneGraph) groupsHoldingReceivers() map[GroupID]bool {
	groups := map[GroupID]bool{}

	for _, receiver := range g.scene.Receivers {
		if id, ok := g.GroupOfPosition(receiver.Position); ok {
			groups[id] = true
		}
	}

	return groups
}

func (c PathSearchConfig) withDefaults(sceneBandCount int) (PathSearchConfig, error) {
	if c.MaxDepth <= 0 {
		c.MaxDepth = DefaultMaxPathDepth
	}

	if c.MaxNodes <= 0 {
		c.MaxNodes = DefaultMaxPathNodes
	}

	if c.PruneFloorDB == 0 {
		c.PruneFloorDB = DefaultPathPruneFloorDB
	}

	if c.BandCount <= 0 {
		c.BandCount = sceneBandCount
	}

	if c.BandCount <= 0 {
		return c, nil
	}

	if len(c.ReductionWeights) == 0 {
		c.ReductionWeights = DefaultReductionWeights(c.BandCount)

		return c, nil
	}

	if len(c.ReductionWeights) != c.BandCount {
		return c, fmt.Errorf("path search reduction weights have length %d, want %d",
			len(c.ReductionWeights), c.BandCount)
	}

	// The reduction formula reads the weights as an energy weighting, so they
	// must be finite, non-negative, and normalisable. An all-zero vector would
	// make every reduction +Inf and a vector that does not sum to one would
	// shift the index away from the level it is compared against.
	normalised := normalizeReductionWeights(c.ReductionWeights)
	if normalised == nil {
		return c, errors.New("path search reduction weights must be finite, non-negative, and sum to a positive value")
	}

	c.ReductionWeights = normalised

	return c, nil
}

// normalizeReductionWeights returns a copy of weights scaled to sum to one, or
// nil if they are not a usable energy weighting.
func normalizeReductionWeights(weights []float64) []float64 {
	total := 0.0

	for _, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil
		}

		total += weight
	}

	if total <= 0 || math.IsInf(total, 0) {
		return nil
	}

	normalised := make([]float64, len(weights))
	for index, weight := range weights {
		normalised[index] = weight / total
	}

	return normalised
}

// activeBands marks the bands whose accumulated transmission is still above the
// floor. This is the source elimination of docs/raven.md section 10: a band the
// portals have already killed need not be simulated at all.
func activeBands(transmission []float64, floorDB float64) []bool {
	active := make([]bool, len(transmission))
	for index, value := range transmission {
		active[index] = value > 0 && 10*math.Log10(value) > floorDB
	}

	return active
}

func anyActive(active []bool) bool {
	for _, value := range active {
		if value {
			return true
		}
	}

	return false
}

// WeightedReductionIndexDB estimates a single-number sound reduction index from
// per-band transmission coefficients as R = -10*log10(sum(w_b * tau_b)).
//
// This is a pragmatic weighted energy average, NOT ISO 717-1. That standard
// needs third-octave data from 100 Hz to 3150 Hz and a shifting reference
// curve, neither of which octave-band scene data can supply. The estimate is
// used only to prune the path search; the filter network always works from the
// per-band coefficients.
//
// Along a chain the per-band coefficients multiply, which is exact for cascaded
// intensity transmission ratios, and the single-number index is recomputed from
// that product. Summing per-portal indices instead would be wrong whenever the
// portals differ in spectral shape.
//
// Portal area, room absorption, and propagation distance are deliberately
// excluded. All three attenuate further, so pruning on transmission alone is
// conservative and never discards a path that would have been audible.
func WeightedReductionIndexDB(transmission, weights []float64) float64 {
	if len(transmission) == 0 {
		return math.Inf(1)
	}

	// Weights that are missing, mis-sized, or not a usable energy weighting fall
	// back to the defaults; usable ones are normalised so a caller-supplied
	// vector cannot shift the index by its overall scale.
	if len(weights) != len(transmission) {
		weights = DefaultReductionWeights(len(transmission))
	} else if normalised := normalizeReductionWeights(weights); normalised != nil {
		weights = normalised
	} else {
		weights = DefaultReductionWeights(len(transmission))
	}

	sum := 0.0
	for index, value := range transmission {
		sum += weights[index] * math.Max(value, 0)
	}

	if sum <= 0 {
		return math.Inf(1)
	}

	return -10 * math.Log10(sum)
}

// DefaultReductionWeights returns normalised band weights that emphasise the
// mid frequencies, approximating where the ISO 717-1 reference curve carries
// most of its weight. The result always sums to one.
func DefaultReductionWeights(bandCount int) []float64 {
	if bandCount <= 0 {
		return nil
	}

	weights := make([]float64, bandCount)
	total := 0.0

	// A raised-cosine bump over the band index keeps the lowest and highest
	// bands contributing without letting them dominate.
	for index := range weights {
		position := (float64(index) + 0.5) / float64(bandCount)
		weights[index] = 0.5 - 0.5*math.Cos(2*math.Pi*position)

		if weights[index] < 1e-3 {
			weights[index] = 1e-3
		}

		total += weights[index]
	}

	for index := range weights {
		weights[index] /= total
	}

	return weights
}
