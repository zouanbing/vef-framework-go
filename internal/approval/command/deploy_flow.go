package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// DeployFlowCmd deploys a flow definition to an existing flow.
type DeployFlowCmd struct {
	cqrs.BaseCommand

	FlowID         string
	FlowDefinition approval.FlowDefinition
	FormDefinition *approval.FormDefinition
}

// AssigneeProvider is the interface for accessing assignees from typed node data.
type AssigneeProvider interface {
	GetAssignees() []approval.AssigneeDefinition
}

// DeployFlowHandler handles the DeployFlowCmd command.
type DeployFlowHandler struct {
	db      orm.DB
	flowSvc *service.FlowService
}

// NewDeployFlowHandler creates a new DeployFlowHandler.
func NewDeployFlowHandler(db orm.DB, flowSvc *service.FlowService) *DeployFlowHandler {
	return &DeployFlowHandler{db: db, flowSvc: flowSvc}
}

func (h *DeployFlowHandler) Handle(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	if err := h.flowSvc.ValidateFlowDefinition(&cmd.FlowDefinition); err != nil {
		return nil, fmt.Errorf("%w: %v", shared.ErrInvalidFlowDesign, err)
	}

	db := contextx.DB(ctx, h.db)

	// Load existing flow
	var flow approval.Flow
	if err := db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.FlowID)
	}).Scan(ctx); err != nil {
		return nil, shared.ErrFlowNotFound
	}

	// Bump version
	flow.CurrentVersion++
	if _, err := db.NewUpdate().Model(&flow).WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("update flow version: %w", err)
	}

	// Create new version
	version := approval.FlowVersion{
		FlowID:     flow.ID,
		Version:    flow.CurrentVersion,
		Status:     approval.VersionDraft,
		FlowSchema: &cmd.FlowDefinition,
		FormSchema: cmd.FormDefinition,
	}
	if _, err := db.NewInsert().Model(&version).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Create nodes and build nodeKey -> nodeID mapping
	nodeKeyToID := make(map[string]string, len(cmd.FlowDefinition.Nodes))

	for _, nd := range cmd.FlowDefinition.Nodes {
		nodeData, err := nd.ParseData()
		if err != nil {
			return nil, fmt.Errorf("parse node %q data: %w", nd.ID, err)
		}

		node := approval.FlowNode{
			FlowVersionID: version.ID,
			NodeKey:       nd.ID,
			NodeKind:      nd.Type,
		}
		nodeData.ApplyTo(&node)

		if _, err := db.NewInsert().Model(&node).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert node: %w", err)
		}

		nodeKeyToID[nd.ID] = node.ID

		if ap, ok := nodeData.(AssigneeProvider); ok {
			for _, assigneeDef := range ap.GetAssignees() {
				assignee := approval.FlowNodeAssignee{
					NodeID:    node.ID,
					Kind:      approval.AssigneeKind(assigneeDef.Kind),
					IDs:       assigneeDef.IDs,
					SortOrder: assigneeDef.SortOrder,
				}
				if assigneeDef.FormField != "" {
					assignee.FormField = new(assigneeDef.FormField)
				}

				if _, err := db.NewInsert().Model(&assignee).Exec(ctx); err != nil {
					return nil, fmt.Errorf("insert node assignee: %w", err)
				}
			}
		}
	}

	// Create edges using real node IDs
	for _, edgeDef := range cmd.FlowDefinition.Edges {
		sourceID, ok := nodeKeyToID[edgeDef.Source]
		if !ok {
			return nil, fmt.Errorf("%w: unknown source node key %q", shared.ErrInvalidFlowDesign, edgeDef.Source)
		}

		targetID, ok := nodeKeyToID[edgeDef.Target]
		if !ok {
			return nil, fmt.Errorf("%w: unknown target node key %q", shared.ErrInvalidFlowDesign, edgeDef.Target)
		}

		edge := approval.FlowEdge{
			FlowVersionID: version.ID,
			SourceNodeID:  sourceID,
			TargetNodeID:  targetID,
		}
		if edgeDef.SourceHandle != "" {
			edge.SourceHandle = new(edgeDef.SourceHandle)
		}

		if _, err := db.NewInsert().Model(&edge).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert edge: %w", err)
		}
	}

	return &version, nil
}
