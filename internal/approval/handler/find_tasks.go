package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/page"
)

// FindTasksQuery queries tasks with filtering and pagination.
type FindTasksQuery struct {
	cqrs.QueryBase
	TenantID   string
	AssigneeID string
	InstanceID string
	Status     string
	page.Pageable
}

// FindTasksHandler handles the FindTasksQuery.
type FindTasksHandler struct {
	db orm.DB
}

// NewFindTasksHandler creates a new FindTasksHandler.
func NewFindTasksHandler(db orm.DB) *FindTasksHandler {
	return &FindTasksHandler{db: db}
}

func (h *FindTasksHandler) Handle(ctx context.Context, q FindTasksQuery) (*service.PagedResult[approval.Task], error) {
	var tasks []approval.Task

	sq := h.db.NewSelect().Model(&tasks)

	if q.TenantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("tenant_id", q.TenantID) })
	}
	if q.AssigneeID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("assignee_id", q.AssigneeID) })
	}
	if q.InstanceID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("instance_id", q.InstanceID) })
	}
	if q.Status != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("status", q.Status) })
	}

	sq = sq.OrderByDesc("created_at")
	q.Normalize(20)
	sq = sq.Limit(q.Size).Offset(q.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	return &service.PagedResult[approval.Task]{List: tasks, Total: int(count)}, nil
}
