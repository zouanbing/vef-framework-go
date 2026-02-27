package query

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/page"
)

// FindTasksQuery queries tasks with filtering and pagination.
type FindTasksQuery struct {
	cqrs.BaseQuery
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

func (h *FindTasksHandler) Handle(ctx context.Context, query FindTasksQuery) (*shared.PagedResult[approval.Task], error) {
	db := contextx.DB(ctx, h.db)

	var tasks []approval.Task

	sq := db.NewSelect().Model(&tasks)

	if query.TenantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("tenant_id", query.TenantID) })
	}
	if query.AssigneeID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("assignee_id", query.AssigneeID) })
	}
	if query.InstanceID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("instance_id", query.InstanceID) })
	}
	if query.Status != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("status", query.Status) })
	}

	sq = sq.OrderByDesc("created_at")
	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	return &shared.PagedResult[approval.Task]{List: tasks, Total: int(count)}, nil
}
