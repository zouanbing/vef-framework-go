package service

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/constants"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/page"
)

// InstanceQuery contains parameters for querying instances.
type InstanceQuery struct {
	ApplicantID string
	Status      string
	FlowID      string
	Keyword     string
	page.Pageable
}

// TaskQuery contains parameters for querying tasks.
type TaskQuery struct {
	AssigneeID string
	InstanceID string
	Status     string
	page.Pageable
}

// InstanceDetail contains the full details of an instance.
type InstanceDetail struct {
	Instance   approval.Instance    `json:"instance"`
	Tasks      []approval.Task      `json:"tasks"`
	ActionLogs []approval.ActionLog `json:"actionLogs"`
	FlowNodes  []approval.FlowNode  `json:"flowNodes"`
}

// QueryService provides read-only queries for approval data.
type QueryService struct {
	db orm.DB
}

// NewQueryService creates a new QueryService.
func NewQueryService(db orm.DB) *QueryService {
	return &QueryService{db: db}
}

// FindInstances queries instances with filtering and pagination.
func (s *QueryService) FindInstances(ctx context.Context, q InstanceQuery) ([]approval.Instance, int, error) {
	var instances []approval.Instance

	sq := s.db.NewSelect().Model(&instances)

	if q.ApplicantID != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("applicant_id", q.ApplicantID)
		})
	}

	if q.Status != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("status", q.Status)
		})
	}

	if q.FlowID != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("flow_id", q.FlowID)
		})
	}

	if q.Keyword != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Contains("title", q.Keyword)
		})
	}

	sq = sq.OrderByDesc("created_at")

	q.Normalize(20)
	sq = sq.Limit(q.Size).Offset(q.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query instances: %w", err)
	}

	return instances, int(count), nil
}

// FindTasks queries tasks with filtering and pagination.
func (s *QueryService) FindTasks(ctx context.Context, q TaskQuery) ([]approval.Task, int, error) {
	var tasks []approval.Task

	sq := s.db.NewSelect().Model(&tasks)

	if q.AssigneeID != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("assignee_id", q.AssigneeID)
		})
	}

	if q.InstanceID != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", q.InstanceID)
		})
	}

	if q.Status != constants.Empty {
		sq = sq.Where(func(c orm.ConditionBuilder) {
			c.Equals("status", q.Status)
		})
	}

	sq = sq.OrderByDesc("created_at")

	q.Normalize(20)
	sq = sq.Limit(q.Size).Offset(q.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}

	return tasks, int(count), nil
}

// GetInstanceDetail returns the full detail of an instance including tasks, action logs, and flow nodes.
func (s *QueryService) GetInstanceDetail(ctx context.Context, instanceID string) (*InstanceDetail, error) {
	var instance approval.Instance

	if err := s.db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instanceID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query instance: %w", err)
	}

	var tasks []approval.Task

	if err := s.db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
	}).OrderBy("sort_order").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	var actionLogs []approval.ActionLog

	if err := s.db.NewSelect().Model(&actionLogs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
	}).OrderBy("created_at").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	var flowNodes []approval.FlowNode

	if err := s.db.NewSelect().Model(&flowNodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", instance.FlowVersionID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	return &InstanceDetail{
		Instance:   instance,
		Tasks:      tasks,
		ActionLogs: actionLogs,
		FlowNodes:  flowNodes,
	}, nil
}

// GetActionLogs returns action logs for an instance.
func (s *QueryService) GetActionLogs(ctx context.Context, instanceID string) ([]approval.ActionLog, error) {
	var logs []approval.ActionLog

	if err := s.db.NewSelect().Model(&logs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
	}).OrderBy("created_at").Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	return logs, nil
}
