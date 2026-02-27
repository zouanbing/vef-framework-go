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

// FindInstancesQuery queries instances with filtering and pagination.
type FindInstancesQuery struct {
	cqrs.BaseQuery
	TenantID    string
	ApplicantID string
	Status      string
	FlowID      string
	Keyword     string
	page.Pageable
}

// FindInstancesHandler handles the FindInstancesQuery.
type FindInstancesHandler struct {
	db orm.DB
}

// NewFindInstancesHandler creates a new FindInstancesHandler.
func NewFindInstancesHandler(db orm.DB) *FindInstancesHandler {
	return &FindInstancesHandler{db: db}
}

func (h *FindInstancesHandler) Handle(ctx context.Context, query FindInstancesQuery) (*shared.PagedResult[approval.Instance], error) {
	db := contextx.DB(ctx, h.db)

	var instances []approval.Instance

	sq := db.NewSelect().Model(&instances)

	if query.TenantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("tenant_id", query.TenantID) })
	}
	if query.ApplicantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("applicant_id", query.ApplicantID) })
	}
	if query.Status != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("status", query.Status) })
	}
	if query.FlowID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("flow_id", query.FlowID) })
	}
	if query.Keyword != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Contains("title", query.Keyword) })
	}

	sq = sq.OrderByDesc("created_at")
	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query instances: %w", err)
	}

	return &shared.PagedResult[approval.Instance]{List: instances, Total: int(count)}, nil
}
