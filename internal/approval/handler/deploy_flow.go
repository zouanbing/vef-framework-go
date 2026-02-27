package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// DeployFlowCmd deploys a flow definition to an existing flow.
type DeployFlowCmd struct {
	cqrs.CommandBase
	FlowID     string
	Definition string
}

// DeployFlowHandler handles the DeployFlowCmd command.
type DeployFlowHandler struct {
	db orm.DB
}

// NewDeployFlowHandler creates a new DeployFlowHandler.
func NewDeployFlowHandler(db orm.DB) *DeployFlowHandler {
	return &DeployFlowHandler{db: db}
}

func (h *DeployFlowHandler) Handle(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	var def approval.FlowDefinition
	if err := json.Unmarshal([]byte(cmd.Definition), &def); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrInvalidFlowDesign, err)
	}

	if err := service.ValidateFlowDefinition(&def); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrInvalidFlowDesign, err)
	}

	db := dbFromCtx(ctx, h.db)

	// Load existing flow
	var flow approval.Flow
	if err := db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.FlowID)
	}).Scan(ctx); err != nil {
		return nil, service.ErrFlowNotFound
	}

	// Bump version
	flow.CurrentVersion++
	if _, err := db.NewUpdate().Model(&flow).WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("update flow version: %w", err)
	}

	// Create new version
	version := approval.FlowVersion{
		FlowID:  flow.ID,
		Version: flow.CurrentVersion,
		Status:  approval.VersionDraft,
	}
	if _, err := db.NewInsert().Model(&version).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Create nodes and build nodeKey -> nodeID mapping
	nodeKeyToID := make(map[string]string, len(def.Nodes))

	for _, nd := range def.Nodes {
		var name string
		if nd.Data != nil {
			if v, ok := nd.Data["label"].(string); ok {
				name = v
			}
		}

		node := approval.FlowNode{
			FlowVersionID: version.ID,
			NodeKey:       nd.ID,
			NodeKind:      nd.Type,
			Name:          name,
		}
		service.ApplyNodeData(&node, nd.Data)

		if _, err := db.NewInsert().Model(&node).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert node: %w", err)
		}

		nodeKeyToID[nd.ID] = node.ID

		assignees := service.ExtractFromData[approval.AssigneeDefinition](nd.Data, "assignees")
		for _, assigneeDef := range assignees {
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

	// Create edges using real node IDs
	for _, edgeDef := range def.Edges {
		sourceID, ok := nodeKeyToID[edgeDef.Source]
		if !ok {
			return nil, fmt.Errorf("%w: unknown source node key %q", service.ErrInvalidFlowDesign, edgeDef.Source)
		}

		targetID, ok := nodeKeyToID[edgeDef.Target]
		if !ok {
			return nil, fmt.Errorf("%w: unknown target node key %q", service.ErrInvalidFlowDesign, edgeDef.Target)
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
