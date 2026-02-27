package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// GetActionLogsQuery retrieves action logs for an instance.
type GetActionLogsQuery struct {
	cqrs.QueryBase
	InstanceID string
}

// GetActionLogsHandler handles the GetActionLogsQuery.
type GetActionLogsHandler struct {
	db orm.DB
}

// NewGetActionLogsHandler creates a new GetActionLogsHandler.
func NewGetActionLogsHandler(db orm.DB) *GetActionLogsHandler {
	return &GetActionLogsHandler{db: db}
}

func (h *GetActionLogsHandler) Handle(ctx context.Context, q GetActionLogsQuery) ([]approval.ActionLog, error) {
	var logs []approval.ActionLog
	if err := h.db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", q.InstanceID)
	}).OrderBy("created_at").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	return logs, nil
}
