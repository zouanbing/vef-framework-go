package shared

import "github.com/ilxqx/vef-framework-go/approval"

// PagedResult is a generic paginated result.
type PagedResult[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
}

// CreateFlowInitiatorCmd contains the parameters for creating a flow initiator.
type CreateFlowInitiatorCmd struct {
	Kind approval.InitiatorKind
	IDs  []string
}

// FlowGraph contains the complete flow graph for a version.
type FlowGraph struct {
	Flow    *approval.Flow        `json:"flow"`
	Version *approval.FlowVersion `json:"version"`
	Nodes   []approval.FlowNode   `json:"nodes"`
	Edges   []approval.FlowEdge   `json:"edges"`
}

// InstanceDetail contains the full details of an instance.
type InstanceDetail struct {
	Instance   approval.Instance    `json:"instance"`
	Tasks      []approval.Task      `json:"tasks"`
	ActionLogs []approval.ActionLog `json:"actionLogs"`
	FlowNodes  []approval.FlowNode  `json:"flowNodes"`
}
