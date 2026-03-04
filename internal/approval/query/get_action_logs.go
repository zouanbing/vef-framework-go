package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// GetActionLogsQuery retrieves action logs for an instance.
type GetActionLogsQuery struct {
	cqrs.BaseQuery

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

func (h *GetActionLogsHandler) Handle(ctx context.Context, query GetActionLogsQuery) ([]approval.ActionLog, error) {
	db := contextx.DB(ctx, h.db)

	var logs []approval.ActionLog

	if err := db.NewSelect().
		Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	return logs, nil
}
