package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// GetFlowGraphQuery retrieves the flow graph for a published flow.
type GetFlowGraphQuery struct {
	cqrs.QueryBase
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

func (h *GetFlowGraphHandler) Handle(ctx context.Context, q GetFlowGraphQuery) (*service.FlowGraph, error) {
	var flow approval.Flow
	if err := h.db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", q.FlowID)
	}).Scan(ctx); err != nil {
		return nil, service.ErrFlowNotFound
	}

	var version approval.FlowVersion
	if err := h.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", q.FlowID)
		c.Equals("status", string(approval.VersionPublished))
	}).Scan(ctx); err != nil {
		return nil, service.ErrNoPublishedVersion
	}

	var nodes []approval.FlowNode
	if err := h.db.NewSelect().Model(&nodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}

	var edges []approval.FlowEdge
	if err := h.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}

	return &service.FlowGraph{
		Flow:    &flow,
		Version: &version,
		Nodes:   nodes,
		Edges:   edges,
	}, nil
}
