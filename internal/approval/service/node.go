package service

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// NodeService provides node-level domain operations.
type NodeService struct {
	engine    *engine.FlowEngine
	publisher *dispatcher.EventPublisher
	taskSvc   *TaskService
}

// NewNodeService creates a new NodeService.
func NewNodeService(eng *engine.FlowEngine, pub *dispatcher.EventPublisher, taskSvc *TaskService) *NodeService {
	return &NodeService{engine: eng, publisher: pub, taskSvc: taskSvc}
}

// HandleNodeCompletion evaluates node completion and handles the result.
// On PassRulePassed: advances to the next node and cancels remaining tasks.
// On PassRuleRejected: marks instance as rejected, cancels remaining tasks, and resumes parent flow.
func (s *NodeService) HandleNodeCompletion(
	ctx context.Context,
	db orm.DB,
	instance *approval.Instance,
	node *approval.FlowNode,
) ([]approval.DomainEvent, error) {
	completionResult, err := s.engine.EvaluateNodeCompletion(ctx, db, instance, node)
	if err != nil {
		return nil, fmt.Errorf("evaluate node completion: %w", err)
	}

	switch completionResult {
	case approval.PassRulePassed:
		if err := s.TriggerNodeCC(ctx, db, instance, node, approval.PassRulePassed); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		if err := s.engine.AdvanceToNextNode(ctx, db, instance, node, ""); err != nil {
			return nil, fmt.Errorf("advance to next node: %w", err)
		}

		if err := s.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
			return nil, err
		}

		return nil, nil

	case approval.PassRuleRejected:
		if err := s.TriggerNodeCC(ctx, db, instance, node, approval.PassRuleRejected); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		instance.Status = approval.InstanceRejected
		instance.FinishedAt = new(timex.Now())

		if err := s.taskSvc.CancelRemainingTasks(ctx, db, instance.ID, node.ID); err != nil {
			return nil, err
		}

		return []approval.DomainEvent{
			approval.NewInstanceCompletedEvent(instance.ID, approval.InstanceRejected),
		}, nil

	default:
		return nil, nil
	}
}

// TriggerNodeCC creates CC records when a node completes, based on CCTiming configuration.
func (s *NodeService) TriggerNodeCC(ctx context.Context, db orm.DB, instance *approval.Instance, node *approval.FlowNode, completionResult approval.PassRuleResult) error {
	var ccConfigs []approval.FlowNodeCC

	if err := db.NewSelect().Model(&ccConfigs).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", node.ID)
	}).Scan(ctx); err != nil {
		return fmt.Errorf("load cc configs for node %s: %w", node.ID, err)
	}

	if len(ccConfigs) == 0 {
		return nil
	}

	var ccUserIDs []string
	for _, cfg := range ccConfigs {
		switch cfg.Timing {
		case approval.CCTimingAlways:
			ccUserIDs = append(ccUserIDs, cfg.IDs...)
		case approval.CCTimingOnApprove:
			if completionResult == approval.PassRulePassed {
				ccUserIDs = append(ccUserIDs, cfg.IDs...)
			}
		case approval.CCTimingOnReject:
			if completionResult == approval.PassRuleRejected {
				ccUserIDs = append(ccUserIDs, cfg.IDs...)
			}
		default:
			ccUserIDs = append(ccUserIDs, cfg.IDs...)
		}
	}

	if len(ccUserIDs) == 0 {
		return nil
	}

	records := make([]approval.CCRecord, 0, len(ccUserIDs))
	for _, userID := range ccUserIDs {
		record := approval.CCRecord{
			InstanceID: instance.ID,
			NodeID:     new(node.ID),
			CCUserID:   userID,
			IsManual:   false,
		}
		records = append(records, record)
	}

	if _, err := db.NewInsert().Model(&records).Exec(ctx); err != nil {
		return fmt.Errorf("insert cc records: %w", err)
	}

	return s.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewCcNotifiedEvent(instance.ID, node.ID, ccUserIDs, false),
	})
}

// CheckCCNodeCompletion checks if all CC records for CC nodes are read and advances the flow.
func (s *NodeService) CheckCCNodeCompletion(ctx context.Context, db orm.DB, instanceID string, records []approval.CCRecord) error {
	nodeIDs := make(map[string]struct{})
	for _, r := range records {
		if r.NodeID != nil {
			nodeIDs[*r.NodeID] = struct{}{}
		}
	}

	for nodeID := range nodeIDs {
		var node approval.FlowNode
		if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", nodeID)
		}).Scan(ctx); err != nil {
			continue
		}

		if node.NodeKind != approval.NodeCC || !node.IsReadConfirmRequired {
			continue
		}

		unreadCount, err := db.NewSelect().Model((*approval.CCRecord)(nil)).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.Equals("node_id", nodeID)
			c.IsNull("read_at")
		}).Count(ctx)
		if err != nil {
			return fmt.Errorf("count unread cc records: %w", err)
		}

		if unreadCount == 0 {
			var instance approval.Instance
			if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
				c.Equals("id", instanceID)
			}).Scan(ctx); err != nil {
				return fmt.Errorf("find instance for cc advance: %w", err)
			}

			if err := s.engine.AdvanceToNextNode(ctx, db, &instance, &node, ""); err != nil {
				return fmt.Errorf("advance cc node: %w", err)
			}
		}
	}

	return nil
}
