package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// GetInstanceDetailQuery retrieves the full detail of an instance.
type GetInstanceDetailQuery struct {
	cqrs.BaseQuery

	InstanceID string
}

// GetInstanceDetailHandler handles the GetInstanceDetailQuery.
type GetInstanceDetailHandler struct {
	db orm.DB
}

// NewGetInstanceDetailHandler creates a new GetInstanceDetailHandler.
func NewGetInstanceDetailHandler(db orm.DB) *GetInstanceDetailHandler {
	return &GetInstanceDetailHandler{db: db}
}

func (h *GetInstanceDetailHandler) Handle(ctx context.Context, query GetInstanceDetailQuery) (*shared.InstanceDetail, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance
	instance.ID = query.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		WherePK().
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query instance: %w", err)
	}

	var tasks []approval.Task

	if err := db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	var actionLogs []approval.ActionLog

	if err := db.NewSelect().
		Model(&actionLogs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	var flowNodes []approval.FlowNode

	if err := db.NewSelect().
		Model(&flowNodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", instance.FlowVersionID) }).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	return &shared.InstanceDetail{
		Instance:   instance,
		Tasks:      tasks,
		ActionLogs: actionLogs,
		FlowNodes:  flowNodes,
	}, nil
}
