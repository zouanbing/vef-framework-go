package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// GetInstanceDetailQuery retrieves the full detail of an instance.
type GetInstanceDetailQuery struct {
	cqrs.QueryBase
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

func (h *GetInstanceDetailHandler) Handle(ctx context.Context, q GetInstanceDetailQuery) (*service.InstanceDetail, error) {
	var instance approval.Instance
	if err := h.db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", q.InstanceID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query instance: %w", err)
	}

	var tasks []approval.Task
	if err := h.db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", q.InstanceID)
	}).OrderBy("sort_order").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	var actionLogs []approval.ActionLog
	if err := h.db.NewSelect().Model(&actionLogs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", q.InstanceID)
	}).OrderBy("created_at").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	var flowNodes []approval.FlowNode
	if err := h.db.NewSelect().Model(&flowNodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", instance.FlowVersionID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	return &service.InstanceDetail{
		Instance:   instance,
		Tasks:      tasks,
		ActionLogs: actionLogs,
		FlowNodes:  flowNodes,
	}, nil
}
