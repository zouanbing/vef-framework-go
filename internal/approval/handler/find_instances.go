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

// FindInstancesQuery queries instances with filtering and pagination.
type FindInstancesQuery struct {
	cqrs.QueryBase
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

func (h *FindInstancesHandler) Handle(ctx context.Context, q FindInstancesQuery) (*service.PagedResult[approval.Instance], error) {
	var instances []approval.Instance

	sq := h.db.NewSelect().Model(&instances)

	if q.TenantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("tenant_id", q.TenantID) })
	}
	if q.ApplicantID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("applicant_id", q.ApplicantID) })
	}
	if q.Status != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("status", q.Status) })
	}
	if q.FlowID != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Equals("flow_id", q.FlowID) })
	}
	if q.Keyword != "" {
		sq = sq.Where(func(c orm.ConditionBuilder) { c.Contains("title", q.Keyword) })
	}

	sq = sq.OrderByDesc("created_at")
	q.Normalize(20)
	sq = sq.Limit(q.Size).Offset(q.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query instances: %w", err)
	}

	return &service.PagedResult[approval.Instance]{List: instances, Total: int(count)}, nil
}
