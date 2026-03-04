package query

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// GetFlowGraphQuery retrieves the flow graph for a published flow.
type GetFlowGraphQuery struct {
	cqrs.BaseQuery

	FlowID string
}

// GetFlowGraphHandler handles the GetFlowGraphQuery.
type GetFlowGraphHandler struct {
	db orm.DB
}

// NewGetFlowGraphHandler creates a new GetFlowGraphHandler.
func NewGetFlowGraphHandler(db orm.DB) *GetFlowGraphHandler {
	return &GetFlowGraphHandler{db: db}
}

func (h *GetFlowGraphHandler) Handle(ctx context.Context, query GetFlowGraphQuery) (*shared.FlowGraph, error) {
	db := contextx.DB(ctx, h.db)

	var flow approval.Flow
	flow.ID = query.FlowID

	if err := db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(ctx); err != nil {
		return nil, shared.ErrFlowNotFound
	}

	var version approval.FlowVersion

	if err := db.NewSelect().
		Model(&version).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", query.FlowID).
				Equals("status", string(approval.VersionPublished))
		}).
		Scan(ctx); err != nil {
		return nil, shared.ErrNoPublishedVersion
	}

	var nodes []approval.FlowNode

	if err := db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}

	var edges []approval.FlowEdge

	if err := db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}

	return &shared.FlowGraph{
		Flow:    &flow,
		Version: &version,
		Nodes:   nodes,
		Edges:   edges,
	}, nil
}
