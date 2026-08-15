package hybrid

import (
	"testing"

	"github.com/cwbudde/algo-acoustics/scene"
)

// treeBuilder assembles a path search tree by hand, so the PPG conversion can
// be tested without standing up scene geometry.
type treeBuilder struct {
	tree *scene.PathSearchTree
}

func newTreeBuilder(bandCount int) *treeBuilder {
	active := make([]bool, bandCount)
	for index := range active {
		active[index] = true
	}

	transmission := make([]float64, bandCount)
	for index := range transmission {
		transmission[index] = 1
	}

	return &treeBuilder{tree: &scene.PathSearchTree{
		Nodes: []scene.PathNode{{
			Group:        0,
			Parent:       -1,
			Transmission: transmission,
			ActiveBands:  active,
		}},
	}}
}

// add appends a child reached by traversing a portal, and returns its index.
func (b *treeBuilder) add(parent int, portalIndex int, reversed bool, group scene.GroupID, factor float64) int {
	parentNode := &b.tree.Nodes[parent]

	transmission := make([]float64, len(parentNode.Transmission))
	active := make([]bool, len(parentNode.ActiveBands))

	for index := range transmission {
		transmission[index] = parentNode.Transmission[index] * factor
		active[index] = parentNode.ActiveBands[index] && transmission[index] > 1e-9
	}

	child := len(b.tree.Nodes)
	b.tree.Nodes = append(b.tree.Nodes, scene.PathNode{
		Group:  group,
		Depth:  parentNode.Depth + 1,
		Parent: parent,
		Step: &scene.PathStep{
			Portal: scene.GroupPortalView{
				PortalView: scene.PortalView{PortalIndex: portalIndex, Reversed: reversed},
				FromGroup:  parentNode.Group,
				ToGroup:    group,
			},
		},
		Transmission: transmission,
		ActiveBands:  active,
	})
	b.tree.Nodes[parent].Children = append(b.tree.Nodes[parent].Children, child)

	return child
}

func (b *treeBuilder) markLeaf(node int) {
	b.tree.Nodes[node].IsLeaf = true
	b.tree.Leaves = append(b.tree.Leaves, node)
}

func TestBuildPPGMergesLeavesIntoSingleReceiver(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(6)
	first := builder.add(0, 0, false, 1, 0.01)
	second := builder.add(0, 1, false, 2, 0.01)
	builder.markLeaf(first)
	builder.markLeaf(second)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	receivers := 0

	for index := range graph.Nodes {
		if graph.Nodes[index].Kind == PPGReceiverNode {
			receivers++
		}
	}

	if receivers != 1 {
		t.Fatalf("got %d receiver nodes, want exactly one", receivers)
	}

	if got := len(graph.Nodes[graph.Receiver].In); got != 2 {
		t.Fatalf("receiver has %d incoming edges, want one per leaf", got)
	}
}

func TestBuildPPGMapsTreeEdgesToPortalNodesAndTreeNodesToEdges(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(6)
	middle := builder.add(0, 0, false, 1, 0.01)
	leaf := builder.add(middle, 1, false, 2, 0.01)
	builder.markLeaf(leaf)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	// Source, receiver, and one portal filter node per traversal.
	portals := 0

	for index := range graph.Nodes {
		if graph.Nodes[index].Kind == PPGPortalNode {
			portals++
		}
	}

	if portals != 2 {
		t.Fatalf("got %d portal nodes, want one per traversal", portals)
	}

	// Group residencies become edges: source group, middle group, leaf group.
	if got := len(graph.Edges); got != 3 {
		t.Fatalf("got %d edges, want one per group residency", got)
	}

	if graph.Edges[0].From != graph.Source {
		t.Fatalf("the first edge starts at node %d, want the source", graph.Edges[0].From)
	}

	last := graph.Edges[len(graph.Edges)-1]
	if last.To != graph.Receiver {
		t.Fatalf("the last edge ends at node %d, want the receiver", last.To)
	}
}

func TestBuildPPGDedupesSharedPortalTraversals(t *testing.T) {
	t.Parallel()

	// A diamond: two routes out of the source group converge on group 3 through
	// the same portal 9. Sharing that node is what turns the tree into a DAG
	// and lets group 3 be rendered once instead of twice.
	builder := newTreeBuilder(6)
	left := builder.add(0, 0, false, 1, 0.1)
	right := builder.add(0, 1, false, 2, 0.1)
	leftLeaf := builder.add(left, 9, false, 3, 0.1)
	rightLeaf := builder.add(right, 9, false, 3, 0.1)
	builder.markLeaf(leftLeaf)
	builder.markLeaf(rightLeaf)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	if len(graph.Nodes) >= len(builder.tree.Nodes)+2 {
		t.Fatalf("got %d graph nodes for %d tree nodes: no dedup happened",
			len(graph.Nodes), len(builder.tree.Nodes))
	}

	shared := -1

	for index := range graph.Nodes {
		if graph.Nodes[index].Kind == PPGPortalNode && graph.Nodes[index].PortalIndex == 9 {
			if shared >= 0 {
				t.Fatal("portal 9 produced more than one node despite identical traversal direction")
			}

			shared = index
		}
	}

	if shared < 0 {
		t.Fatal("portal 9 has no node")
	}

	if got := len(graph.Nodes[shared].In); got != 2 {
		t.Fatalf("the shared portal node has %d incoming edges, want 2", got)
	}

	// Both leaves are the same group residency reached through the same node,
	// so they merge into one edge to the receiver.
	if got := len(graph.Nodes[graph.Receiver].In); got != 1 {
		t.Fatalf("receiver has %d incoming edges, want the merged one", got)
	}
}

func TestBuildPPGSeparatesOppositeTraversalDirections(t *testing.T) {
	t.Parallel()

	// The same portal crossed the other way is a different filter, so it must
	// not share a node.
	builder := newTreeBuilder(6)
	forward := builder.add(0, 4, false, 1, 0.1)
	backward := builder.add(0, 4, true, 2, 0.1)
	builder.markLeaf(forward)
	builder.markLeaf(backward)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	portals := 0

	for index := range graph.Nodes {
		if graph.Nodes[index].Kind == PPGPortalNode {
			portals++
		}
	}

	if portals != 2 {
		t.Fatalf("got %d portal nodes, want one per traversal direction", portals)
	}
}

func TestPPGActiveBandsForGroupIsUnionOfPaths(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(3)

	// One route kills the upper bands, the other the lower ones. The merged
	// edge must keep the union, so the group is still simulated where either
	// path needs it — conservative by design.
	left := builder.add(0, 0, false, 1, 0.5)
	builder.tree.Nodes[left].ActiveBands = []bool{true, true, false}
	right := builder.add(0, 1, false, 1, 0.5)
	builder.tree.Nodes[right].ActiveBands = []bool{false, true, true}
	builder.markLeaf(left)
	builder.markLeaf(right)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	union := graph.ActiveBandsForGroup(1)
	for band, active := range union {
		if !active {
			t.Fatalf("band %d is inactive; the union must keep every band either path needs", band)
		}
	}
}

func TestPPGEdgesForGroup(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(6)
	middle := builder.add(0, 0, false, 1, 0.1)
	leaf := builder.add(middle, 1, false, 2, 0.1)
	builder.markLeaf(leaf)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	for _, group := range []scene.GroupID{0, 1, 2} {
		if got := len(graph.EdgesForGroup(group)); got != 1 {
			t.Fatalf("group %d has %d edges, want 1", group, got)
		}
	}

	if got := len(graph.EdgesForGroup(7)); got != 0 {
		t.Fatalf("an absent group reported %d edges", got)
	}
}

func TestPPGTopologicalOrderIsAcyclic(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(6)
	left := builder.add(0, 0, false, 1, 0.1)
	right := builder.add(0, 1, false, 2, 0.1)
	leftLeaf := builder.add(left, 9, false, 3, 0.1)
	rightLeaf := builder.add(right, 9, false, 3, 0.1)
	builder.markLeaf(leftLeaf)
	builder.markLeaf(rightLeaf)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	order, err := graph.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	if len(order) != len(graph.Nodes) {
		t.Fatalf("order covers %d of %d nodes", len(order), len(graph.Nodes))
	}

	position := make(map[int]int, len(order))
	for index, node := range order {
		position[node] = index
	}

	for index := range graph.Edges {
		edge := &graph.Edges[index]
		if position[edge.From] >= position[edge.To] {
			t.Fatalf("edge %d runs backwards in the topological order", index)
		}
	}
}

func TestBuildPPGStaysAcyclicWhenPathsCrossPortalsInOppositeOrder(t *testing.T) {
	t.Parallel()

	// A ring of groups: one simple path leaves through portal 1 and then portal
	// 2, another through portal 2 and then portal 1. Keying portal nodes on
	// (portal, direction) alone would merge both occurrences of each portal and
	// produce the edge pair 1->2 and 2->1, which is a cycle even though both
	// searched paths are simple.
	builder := newTreeBuilder(6)
	first := builder.add(0, 1, false, 1, 0.1)
	firstLeaf := builder.add(first, 2, false, 2, 0.1)
	second := builder.add(0, 2, false, 2, 0.1)
	secondLeaf := builder.add(second, 1, false, 1, 0.1)
	builder.markLeaf(firstLeaf)
	builder.markLeaf(secondLeaf)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	order, err := graph.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	if len(order) != len(graph.Nodes) {
		t.Fatalf("order covers %d of %d nodes", len(order), len(graph.Nodes))
	}
}

func TestBuildPPGRejectsEmptyInput(t *testing.T) {
	t.Parallel()

	_, err := BuildPPG(nil)
	if err == nil {
		t.Fatal("BuildPPG accepted a nil tree")
	}

	_, err = BuildPPG(&scene.PathSearchTree{})
	if err == nil {
		t.Fatal("BuildPPG accepted a tree with no nodes")
	}
}

func TestBuildPPGHandlesASourceGroupThatIsAlreadyTheReceiverGroup(t *testing.T) {
	t.Parallel()

	builder := newTreeBuilder(6)
	builder.markLeaf(0)

	graph, err := BuildPPG(builder.tree)
	if err != nil {
		t.Fatalf("BuildPPG: %v", err)
	}

	if len(graph.Edges) != 1 {
		t.Fatalf("got %d edges, want the single source-to-receiver residency", len(graph.Edges))
	}

	if graph.Edges[0].From != graph.Source || graph.Edges[0].To != graph.Receiver {
		t.Fatalf("edge = %+v, want source to receiver", graph.Edges[0])
	}
}
