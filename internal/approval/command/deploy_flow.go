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
	Description    *string
	FlowDefinition approval.FlowDefinition
	FormDefinition *approval.FormDefinition
}

// AssigneeProvider is the interface for accessing assignees from typed node data.
type AssigneeProvider interface {
	// GetAssignees returns the assignee definitions configured on this node.
	GetAssignees() []approval.AssigneeDefinition
}

// CCProvider is the interface for accessing CC list from typed node data.
type CCProvider interface {
	// GetCCs returns the CC recipient definitions configured on this node.
	GetCCs() []approval.CCDefinition
}

// DeployFlowHandler handles the DeployFlowCmd command.
type DeployFlowHandler struct {
	db         orm.DB
	flowDefSvc *service.FlowDefinitionService
}

// NewDeployFlowHandler creates a new DeployFlowHandler.
func NewDeployFlowHandler(db orm.DB, flowDefSvc *service.FlowDefinitionService) *DeployFlowHandler {
	return &DeployFlowHandler{db: db, flowDefSvc: flowDefSvc}
}

func (h *DeployFlowHandler) Handle(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	if err := h.flowDefSvc.ValidateFlowDefinition(&cmd.FlowDefinition); err != nil {
		return nil, fmt.Errorf("%w: %v", shared.ErrInvalidFlowDesign, err)
	}

	db := contextx.DB(ctx, h.db)

	var flow approval.Flow
	flow.ID = cmd.FlowID
	if err := db.NewSelect().
		Model(&flow).
		Select("current_version").
		WherePK().
		Scan(ctx); err != nil {
		return nil, shared.ErrFlowNotFound
	}

	version := approval.FlowVersion{
		FlowID:      flow.ID,
		Version:     flow.CurrentVersion + 1,
		Status:      approval.VersionDraft,
		Description: cmd.Description,
		FlowSchema:  &cmd.FlowDefinition,
		FormSchema:  cmd.FormDefinition,
	}
	if _, err := db.NewInsert().
		Model(&version).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Create nodes and build nodeKey -> nodeID mapping
	nodeKeyToID := make(map[string]string, len(cmd.FlowDefinition.Nodes))

	for _, nodeDef := range cmd.FlowDefinition.Nodes {
		nodeData, err := nodeDef.ParseData()
		if err != nil {
			return nil, fmt.Errorf("parse node %q data: %w", nodeDef.ID, err)
		}

		node := approval.FlowNode{
			FlowVersionID: version.ID,
			Key:           nodeDef.ID,
			Kind:          nodeDef.Kind,
		}
		nodeData.ApplyTo(&node)

		if _, err := db.NewInsert().
			Model(&node).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert node: %w", err)
		}

		nodeKeyToID[nodeDef.ID] = node.ID

		if ap, ok := nodeData.(AssigneeProvider); ok {
			for _, assigneeDef := range ap.GetAssignees() {
				assignee := approval.FlowNodeAssignee{
					NodeID:    node.ID,
					Kind:      assigneeDef.Kind,
					IDs:       assigneeDef.IDs,
					FormField: assigneeDef.FormField,
					SortOrder: assigneeDef.SortOrder,
				}

				if _, err := db.NewInsert().
					Model(&assignee).
					Exec(ctx); err != nil {
					return nil, fmt.Errorf("insert node assignee: %w", err)
				}
			}
		}

		if cp, ok := nodeData.(CCProvider); ok {
			for _, ccDef := range cp.GetCCs() {
				cc := approval.FlowNodeCC{
					NodeID:    node.ID,
					Kind:      ccDef.Kind,
					IDs:       ccDef.IDs,
					FormField: ccDef.FormField,
					Timing:    ccDef.Timing,
				}

				if _, err := db.NewInsert().
					Model(&cc).
					Exec(ctx); err != nil {
					return nil, fmt.Errorf("insert node cc: %w", err)
				}
			}
		}
	}

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
			Key:           edgeDef.ID,
			SourceNodeID:  sourceID,
			SourceNodeKey: edgeDef.Source,
			TargetNodeID:  targetID,
			TargetNodeKey: edgeDef.Target,
			SourceHandle:  edgeDef.SourceHandle,
		}

		if _, err := db.NewInsert().
			Model(&edge).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert edge: %w", err)
		}
	}

	return &version, nil
}
