package service

import (
	"fmt"

	"github.com/coldsmirk/go-collections"
	streams "github.com/coldsmirk/go-streams"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// validNodeKinds defines the set of valid node kinds for flow validation.
var validNodeKinds = collections.NewHashSetFrom(
	approval.NodeStart,
	approval.NodeEnd,
	approval.NodeApproval,
	approval.NodeHandle,
	approval.NodeCondition,
	approval.NodeCC,
)

// FlowDefinitionService provides flow-level domain operations.
type FlowDefinitionService struct{}

// NewFlowDefinitionService creates a new FlowDefinitionService.
func NewFlowDefinitionService() *FlowDefinitionService {
	return &FlowDefinitionService{}
}

// ValidateFlowDefinition validates the structural integrity of a flow definition.
func (s *FlowDefinitionService) ValidateFlowDefinition(def *approval.FlowDefinition) error {
	if len(def.Nodes) == 0 {
		return fmt.Errorf("flow must have at least one node")
	}

	// --- Phase 1: Node validation ---
	var (
		nodeIDs      = collections.NewHashSet[string]()
		condBranches = make(map[string][]approval.ConditionBranch)

		startCount, endCount int
		startID              string
		endIDs               []string
	)

	for i := range def.Nodes {
		node := &def.Nodes[i]

		if node.ID == "" {
			return fmt.Errorf("node ID must not be empty")
		}

		if !nodeIDs.Add(node.ID) {
			return fmt.Errorf("duplicate node ID %q", node.ID)
		}

		if !validNodeKinds.Contains(node.Kind) {
			return fmt.Errorf("invalid node kind %q for node %q", node.Kind, node.ID)
		}

		switch node.Kind {
		case approval.NodeStart:
			startCount++
			startID = node.ID
		case approval.NodeEnd:
			endCount++
			endIDs = append(endIDs, node.ID)
		case approval.NodeCondition:
			data, err := node.ParseData()
			if err != nil {
				return fmt.Errorf("parse node %q data: %w", node.ID, err)
			}

			condBranches[node.ID] = data.(*approval.ConditionNodeData).Branches
		}
	}

	if startCount != 1 {
		return fmt.Errorf("flow must have exactly 1 start node, found %d", startCount)
	}

	if endCount < 1 {
		return fmt.Errorf("flow must have at least 1 end node, found %d", endCount)
	}

	// --- Phase 2: Edge validation & adjacency ---
	var (
		edgeIDs     = collections.NewHashSet[string]()
		outEdges    = make(map[string][]approval.EdgeDefinition, len(def.Nodes))
		inDegree    = make(map[string]int, len(def.Nodes))
		adjacency   = make(map[string][]string, len(def.Nodes))
		reversedAdj = make(map[string][]string, len(def.Nodes))
	)

	for _, edge := range def.Edges {
		if edge.ID == "" {
			return fmt.Errorf("edge ID must not be empty")
		}

		if !edgeIDs.Add(edge.ID) {
			return fmt.Errorf("duplicate edge ID %q", edge.ID)
		}

		if !nodeIDs.Contains(edge.Source) {
			return fmt.Errorf("edge %q references unknown source node %q", edge.ID, edge.Source)
		}

		if !nodeIDs.Contains(edge.Target) {
			return fmt.Errorf("edge %q references unknown target node %q", edge.ID, edge.Target)
		}

		outEdges[edge.Source] = append(outEdges[edge.Source], edge)
		inDegree[edge.Target]++
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		reversedAdj[edge.Target] = append(reversedAdj[edge.Target], edge.Source)
	}

	// --- Phase 3: Degree constraints ---
	if inDegree[startID] > 0 {
		return fmt.Errorf("start node must not have incoming edges")
	}

	if len(outEdges[startID]) != 1 {
		return fmt.Errorf("start node must have exactly 1 outgoing edge, found %d", len(outEdges[startID]))
	}

	for _, endID := range endIDs {
		if len(outEdges[endID]) > 0 {
			return fmt.Errorf("end node %q must not have outgoing edges", endID)
		}

		if inDegree[endID] == 0 {
			return fmt.Errorf("end node %q must have at least 1 incoming edge", endID)
		}
	}

	for _, node := range def.Nodes {
		if node.Kind == approval.NodeStart || node.Kind == approval.NodeEnd {
			continue
		}

		outs := outEdges[node.ID]

		switch node.Kind {
		case approval.NodeCondition:
			if err := validateConditionEdges(node.ID, condBranches[node.ID], outs); err != nil {
				return err
			}
		default:
			if len(outs) != 1 {
				return fmt.Errorf("node %q must have exactly 1 outgoing edge, found %d", node.ID, len(outs))
			}

			if outs[0].SourceHandle != nil {
				return fmt.Errorf("non-condition node %q must not have sourceHandle on outgoing edge", node.ID)
			}
		}
	}

	// --- Phase 4: Topology ---
	nodeIDSlice := streams.MapTo(streams.FromSlice(def.Nodes), func(n approval.NodeDefinition) string {
		return n.ID
	}).Collect()

	if detectCycle(nodeIDSlice, adjacency) {
		return fmt.Errorf("flow graph contains a cycle")
	}

	reachable := collectReachable(adjacency, startID)
	if reachable.Size() != nodeIDs.Size() {
		for _, node := range def.Nodes {
			if !reachable.Contains(node.ID) {
				return fmt.Errorf("node %q is not reachable from start node", node.ID)
			}
		}
	}

	canReachEnd := collectReachable(reversedAdj, endIDs...)
	if canReachEnd.Size() != nodeIDs.Size() {
		for _, node := range def.Nodes {
			if !canReachEnd.Contains(node.ID) {
				return fmt.Errorf("node %q cannot reach end node", node.ID)
			}
		}
	}

	return nil
}

// validateConditionEdges validates that a condition node's outgoing edges match its branches exactly.
func validateConditionEdges(nodeID string, branches []approval.ConditionBranch, outs []approval.EdgeDefinition) error {
	if len(branches) < 2 {
		return fmt.Errorf("condition node %q must have at least 2 branches, found %d", nodeID, len(branches))
	}

	branchIDs := collections.NewHashSet[string]()
	var defaultCount int

	for _, branch := range branches {
		if branch.ID == "" {
			return fmt.Errorf("condition node %q has a branch with empty ID", nodeID)
		}

		if !branchIDs.Add(branch.ID) {
			return fmt.Errorf("condition node %q has duplicate branch ID %q", nodeID, branch.ID)
		}

		if branch.IsDefault {
			defaultCount++
		}
	}

	if defaultCount != 1 {
		return fmt.Errorf("condition node %q must have exactly 1 default branch, found %d", nodeID, defaultCount)
	}

	edgeHandles := collections.NewHashSet[string]()

	for _, edge := range outs {
		if edge.SourceHandle == nil {
			return fmt.Errorf("condition node %q: edge %q must have a sourceHandle", nodeID, edge.ID)
		}

		handle := *edge.SourceHandle
		if !branchIDs.Contains(handle) {
			return fmt.Errorf("condition node %q: edge %q has unknown sourceHandle %q", nodeID, edge.ID, handle)
		}

		if !edgeHandles.Add(handle) {
			return fmt.Errorf("condition node %q: duplicate outgoing edge for handle %q", nodeID, handle)
		}
	}

	if edgeHandles.Size() != branchIDs.Size() {
		for _, branch := range branches {
			if !edgeHandles.Contains(branch.ID) {
				return fmt.Errorf("condition node %q: branch %q has no outgoing edge", nodeID, branch.ID)
			}
		}
	}

	return nil
}

// detectCycle returns true if the directed graph contains a cycle (DFS coloring).
func detectCycle(nodes []string, adjacency map[string][]string) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	var (
		color = make(map[string]int, len(nodes))
		visit func(string) bool
	)

	visit = func(node string) bool {
		color[node] = gray

		for _, next := range adjacency[node] {
			if color[next] == gray || (color[next] == white && visit(next)) {
				return true
			}
		}

		color[node] = black

		return false
	}

	for _, node := range nodes {
		if color[node] == white && visit(node) {
			return true
		}
	}

	return false
}

// collectReachable returns the set of nodes reachable from any of the start nodes via BFS.
func collectReachable(adjacency map[string][]string, starts ...string) collections.Set[string] {
	visited := collections.NewHashSet[string]()
	queue := make([]string, len(starts))

	for i, start := range starts {
		visited.Add(start)
		queue[i] = start
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adjacency[current] {
			if visited.Add(next) {
				queue = append(queue, next)
			}
		}
	}

	return visited
}
