package hybrid

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-acoustics/scene"
)

// PPGNodeKind classifies a node of the propagation path graph.
type PPGNodeKind int

const (
	// PPGSourceNode is the single node standing for the primary source.
	PPGSourceNode PPGNodeKind = iota
	// PPGPortalNode is a portal filter, traversed in one direction.
	PPGPortalNode
	// PPGReceiverNode is the single node all propagation paths end at.
	PPGReceiverNode
)

// PPGNode is a node of the propagation path graph: the source, a portal filter,
// or the receiver.
type PPGNode struct {
	Kind PPGNodeKind
	// PortalIndex identifies the portal for PPGPortalNode; -1 otherwise.
	PortalIndex int
	// Reversed records the traversal direction, so a portal crossed one way is
	// a different filter node than the same portal crossed the other way.
	Reversed bool
	In, Out  []int
}

// PPGEdge is a room-group transfer function joining two filter nodes.
type PPGEdge struct {
	From, To int
	Group    scene.GroupID
	// ActiveBands is the union of the contributing paths' band masks.
	ActiveBands []bool
	// Transmission is the per-band maximum of the contributing paths'
	// accumulated coefficients at the edge entry.
	Transmission []float64
}

// PPG is the propagation path graph of docs/raven.md section 10: a directed
// acyclic graph that serves as the construction plan for the filter network.
//
// The mapping from the path search tree is exactly the one the reference gives:
// the tree root becomes the source node, tree edges (portal traversals) become
// portal filter nodes, tree nodes (group residencies) become transfer-function
// edges, and every leaf merges into a single receiver node.
type PPG struct {
	Nodes            []PPGNode
	Edges            []PPGEdge
	Source, Receiver int
}

// ppgNodeKey deduplicates portal filter nodes. Two distinct paths that reach
// the same portal in the same direction converge on one node, which is what
// turns the tree into a genuine DAG and lets a group's transfer function be
// rendered once per entry/exit pair instead of once per path.
type ppgNodeKey struct {
	portalIndex int
	reversed    bool
}

type ppgEdgeKey struct {
	from, to int
	group    scene.GroupID
}

// BuildPPG converts a path search tree into a propagation path graph.
//
// Because portal nodes are shared between paths, a merged node has no single
// per-path band mask. ActiveBands is therefore the union across contributing
// paths and Transmission their per-band maximum. Both are conservative: the
// filter network may simulate a band it could have skipped, but never skips one
// it needed. Exact per-path products are recovered by walking the graph and
// multiplying the portal filters along the way.
func BuildPPG(tree *scene.PathSearchTree) (*PPG, error) {
	if tree == nil {
		return nil, errors.New("path search tree is nil")
	}

	if len(tree.Nodes) == 0 {
		return nil, errors.New("path search tree has no nodes")
	}

	graph := &PPG{}
	graph.Source = graph.addNode(PPGNode{Kind: PPGSourceNode, PortalIndex: -1})
	graph.Receiver = graph.addNode(PPGNode{Kind: PPGReceiverNode, PortalIndex: -1})

	portalNodes := map[ppgNodeKey]int{}
	edgeIndex := map[ppgEdgeKey]int{}

	// nodeOf maps a tree node to the graph node standing for the filter that
	// leads into it: the source node for the root, a portal node otherwise.
	nodeOf := make([]int, len(tree.Nodes))
	nodeOf[tree.Root] = graph.Source

	for index := range tree.Nodes {
		treeNode := &tree.Nodes[index]
		if treeNode.Step == nil {
			continue
		}

		key := ppgNodeKey{portalIndex: treeNode.Step.Portal.PortalIndex, reversed: treeNode.Step.Portal.Reversed}

		node, ok := portalNodes[key]
		if !ok {
			node = graph.addNode(PPGNode{Kind: PPGPortalNode, PortalIndex: key.portalIndex, Reversed: key.reversed})
			portalNodes[key] = node
		}

		nodeOf[index] = node
	}

	for index := range tree.Nodes {
		treeNode := &tree.Nodes[index]

		// A group residency becomes an edge from the filter that entered it to
		// each filter that leaves it.
		for _, child := range treeNode.Children {
			graph.mergeEdge(edgeIndex, nodeOf[index], nodeOf[child], treeNode.Group, treeNode)
		}

		// A leaf's residency also becomes an edge to the receiver.
		if treeNode.IsLeaf {
			graph.mergeEdge(edgeIndex, nodeOf[index], graph.Receiver, treeNode.Group, treeNode)
		}
	}

	return graph, nil
}

// EdgesForGroup returns the edge indices whose transfer function belongs to a
// room group, so a caller can render that group once and reuse the result.
func (p *PPG) EdgesForGroup(id scene.GroupID) []int {
	var indices []int

	for index := range p.Edges {
		if p.Edges[index].Group == id {
			indices = append(indices, index)
		}
	}

	return indices
}

// ActiveBandsForGroup returns the union of the band masks of every edge in a
// group, which is the set of bands that group must be simulated in.
func (p *PPG) ActiveBandsForGroup(id scene.GroupID) []bool {
	var union []bool

	for index := range p.Edges {
		edge := &p.Edges[index]
		if edge.Group != id {
			continue
		}

		if union == nil {
			union = make([]bool, len(edge.ActiveBands))
		}

		for band, active := range edge.ActiveBands {
			if band < len(union) && active {
				union[band] = true
			}
		}
	}

	return union
}

// TopologicalOrder returns the node indices in an order where every edge leads
// from an earlier node to a later one, which is the order the filter network is
// evaluated in. It fails if the graph contains a cycle.
func (p *PPG) TopologicalOrder() ([]int, error) {
	indegree := make([]int, len(p.Nodes))
	for index := range p.Edges {
		indegree[p.Edges[index].To]++
	}

	queue := make([]int, 0, len(p.Nodes))

	for node, degree := range indegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	order := make([]int, 0, len(p.Nodes))

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		order = append(order, node)

		for _, edgeIndex := range p.Nodes[node].Out {
			target := p.Edges[edgeIndex].To

			indegree[target]--
			if indegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}

	if len(order) != len(p.Nodes) {
		return nil, fmt.Errorf("propagation path graph is cyclic: ordered %d of %d nodes", len(order), len(p.Nodes))
	}

	return order, nil
}

func (p *PPG) addNode(node PPGNode) int {
	p.Nodes = append(p.Nodes, node)

	return len(p.Nodes) - 1
}

func (p *PPG) mergeEdge(index map[ppgEdgeKey]int, from, to int, group scene.GroupID, treeNode *scene.PathNode) {
	key := ppgEdgeKey{from: from, to: to, group: group}

	existing, ok := index[key]
	if !ok {
		p.Edges = append(p.Edges, PPGEdge{
			From:         from,
			To:           to,
			Group:        group,
			ActiveBands:  append([]bool(nil), treeNode.ActiveBands...),
			Transmission: append([]float64(nil), treeNode.Transmission...),
		})
		index[key] = len(p.Edges) - 1
		p.Nodes[from].Out = append(p.Nodes[from].Out, len(p.Edges)-1)
		p.Nodes[to].In = append(p.Nodes[to].In, len(p.Edges)-1)

		return
	}

	edge := &p.Edges[existing]
	for band := range edge.ActiveBands {
		if band < len(treeNode.ActiveBands) && treeNode.ActiveBands[band] {
			edge.ActiveBands[band] = true
		}
	}

	for band := range edge.Transmission {
		if band < len(treeNode.Transmission) && treeNode.Transmission[band] > edge.Transmission[band] {
			edge.Transmission[band] = treeNode.Transmission[band]
		}
	}
}
