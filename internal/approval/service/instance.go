package service

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"text/template"
	"time"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/timex"
)

// StartInstanceCmd contains the parameters for starting a new instance.
type StartInstanceCmd struct {
	TenantID         string
	FlowCode         string
	ApplicantID      string
	ApplicantDeptID  *string
	BusinessRecordID *string
	FormData         map[string]any
}

// buildTitleTemplateData builds a camelCase map for rendering instance title templates.
// Templates access fields via {{.flowName}}, {{.flowCode}}, {{.instanceNo}},
// and form values via {{.formData.fieldName}}.
func buildTitleTemplateData(flowName, flowCode, instanceNo string, formData map[string]any) map[string]any {
	return map[string]any{
		"flowName":   flowName,
		"flowCode":   flowCode,
		"instanceNo": instanceNo,
		"formData":   formData,
	}
}

// ProcessTaskCmd contains the parameters for processing a task.
type ProcessTaskCmd struct {
	InstanceID   string
	TaskID       string
	Action       string // "approve", "reject", "transfer", "rollback", "handle"
	OperatorID   string
	Opinion      string
	FormData     map[string]any
	TransferToID string
	TargetNodeID string
}

// UrgeTaskCmd contains the parameters for urging a task.
type UrgeTaskCmd struct {
	InstanceID string
	TaskID     string
	UrgerID    string
	Message    string
}

// AddAssigneeCmd contains the parameters for dynamically adding assignees.
type AddAssigneeCmd struct {
	InstanceID string
	TaskID     string
	UserIDs    []string
	AddType    string // "before", "after", "parallel"
	OperatorID string
}

// InstanceService manages instance lifecycle.
type InstanceService struct {
	db              orm.DB
	engine          *engine.FlowEngine
	instanceNoGen   InstanceNoGenerator
	publisher       *dispatcher.EventPublisher
	assigneeService AssigneeService
}

// NewInstanceService creates a new InstanceService.
func NewInstanceService(db orm.DB, eng *engine.FlowEngine, instanceNoGen InstanceNoGenerator, pub *dispatcher.EventPublisher, assigneeSvc AssigneeService) *InstanceService {
	return &InstanceService{
		db:              db,
		engine:          eng,
		instanceNoGen:   instanceNoGen,
		publisher:       pub,
		assigneeService: assigneeSvc,
	}
}

// StartInstance creates a new flow instance and starts the engine.
func (s *InstanceService) StartInstance(ctx context.Context, cmd StartInstanceCmd) (*approval.Instance, error) {
	var result *approval.Instance

	if err := s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var flow approval.Flow

		if err := tx.NewSelect().
			Model(&flow).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", cmd.TenantID)
				cb.Equals("code", cmd.FlowCode)
			}).
			Scan(ctx); err != nil {
			return ErrFlowNotFound
		}

		if !flow.IsActive {
			return ErrFlowNotActive
		}

		if !flow.IsAllInitiateAllowed {
			allowed, err := s.checkInitiationPermission(ctx, tx, flow.ID, cmd.ApplicantID, cmd.ApplicantDeptID)
			if err != nil {
				return fmt.Errorf("check initiation permission: %w", err)
			}

			if !allowed {
				return ErrNotAllowedInitiate
			}
		}

		var version approval.FlowVersion

		if err := tx.NewSelect().
			Model(&version).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_id", flow.ID)
				cb.Equals("status", string(approval.VersionPublished))
			}).
			Scan(ctx); err != nil {
			return ErrNoPublishedVersion
		}

		instanceNo, err := s.instanceNoGen.Generate(ctx, cmd.FlowCode)
		if err != nil {
			return fmt.Errorf("generate instance number: %w", err)
		}

		title, err := renderTitle(
			flow.InstanceTitleTemplate,
			buildTitleTemplateData(flow.Name, flow.Code, instanceNo, cmd.FormData),
		)
		if err != nil {
			return fmt.Errorf("render instance title: %w", err)
		}

		instance := &approval.Instance{
			TenantID:         flow.TenantID,
			FlowID:           flow.ID,
			FlowVersionID:    version.ID,
			Title:            title,
			InstanceNo:       instanceNo,
			ApplicantID:      cmd.ApplicantID,
			ApplicantDeptID:  cmd.ApplicantDeptID,
			Status:           approval.InstanceRunning,
			BusinessRecordID: cmd.BusinessRecordID,
			FormData:         cmd.FormData,
		}

		if _, err := tx.NewInsert().Model(instance).Exec(ctx); err != nil {
			return fmt.Errorf("insert instance: %w", err)
		}

		submitLog := &approval.ActionLog{
			InstanceID: instance.ID,
			Action:     approval.ActionSubmit,
			OperatorID: cmd.ApplicantID,
		}
		if _, err := tx.NewInsert().Model(submitLog).Exec(ctx); err != nil {
			return fmt.Errorf("insert submit log: %w", err)
		}

		if err := s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewInstanceCreatedEvent(instance.ID, flow.ID, title, cmd.ApplicantID),
		}); err != nil {
			return fmt.Errorf("publish instance created event: %w", err)
		}

		if err := s.engine.StartProcess(ctx, tx, instance); err != nil {
			return fmt.Errorf("start process: %w", err)
		}

		if _, err := tx.NewUpdate().Model(instance).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update instance after start: %w", err)
		}

		result = instance

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// ProcessTask handles a task action (approve/reject/transfer/rollback/handle).
func (s *InstanceService) ProcessTask(ctx context.Context, cmd ProcessTaskCmd) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var instance approval.Instance

		if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.InstanceID)
		}).Scan(ctx); err != nil {
			return ErrInstanceNotFound
		}

		if instance.Status != approval.InstanceRunning {
			return ErrInstanceCompleted
		}

		var task approval.Task

		if err := tx.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.TaskID)
			c.Equals("instance_id", cmd.InstanceID)
		}).Scan(ctx); err != nil {
			return ErrTaskNotFound
		}

		if task.AssigneeID != cmd.OperatorID {
			return ErrNotAssignee
		}

		if task.Status != approval.TaskPending {
			return ErrTaskNotPending
		}

		var node approval.FlowNode

		if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", task.NodeID)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("load node: %w", err)
		}

		if len(cmd.FormData) > 0 {
			editableData := filterEditableFormData(cmd.FormData, node.FieldPermissions)
			if len(editableData) > 0 {
				if instance.FormData == nil {
					instance.FormData = make(map[string]any, len(editableData))
				}

				maps.Copy(instance.FormData, editableData)
			}
		}

		var (
			events []approval.DomainEvent
			err    error
		)

		switch approval.ActionType(cmd.Action) {
		case approval.ActionApprove, approval.ActionExecute:
			events, err = s.handleApprove(ctx, tx, &instance, &task, &node, cmd)
		case approval.ActionReject:
			events, err = s.handleReject(ctx, tx, &instance, &task, &node, cmd)
		case approval.ActionTransfer:
			events, err = s.handleTransfer(ctx, tx, &instance, &task, &node, cmd)
		case approval.ActionRollback:
			events, err = s.handleRollback(ctx, tx, &instance, &task, &node, cmd)
		default:
			return fmt.Errorf("unsupported action: %s", cmd.Action)
		}

		if err != nil {
			return err
		}

		actionLog := &approval.ActionLog{
			InstanceID: instance.ID,
			NodeID:     new(task.NodeID),
			TaskID:     new(task.ID),
			Action:     approval.ActionType(cmd.Action),
			OperatorID: cmd.OperatorID,
		}
		if cmd.Opinion != "" {
			actionLog.Opinion = new(cmd.Opinion)
		}
		if cmd.Action == string(approval.ActionTransfer) {
			actionLog.TransferToID = new(cmd.TransferToID)
		}

		if cmd.Action == string(approval.ActionRollback) {
			actionLog.RollbackToNodeID = new(cmd.TargetNodeID)
		}

		if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
			return fmt.Errorf("insert action log: %w", err)
		}

		if _, err := tx.NewUpdate().Model(&instance).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update instance: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, events)
	})
}

func (s *InstanceService) handleApprove(
	ctx context.Context,
	tx orm.DB,
	instance *approval.Instance,
	task *approval.Task,
	node *approval.FlowNode,
	cmd ProcessTaskCmd,
) ([]approval.DomainEvent, error) {
	if err := validateOpinion(node, cmd.Opinion); err != nil {
		return nil, err
	}

	targetStatus := approval.TaskApproved
	if node.NodeKind == approval.NodeHandle {
		targetStatus = approval.TaskHandled
	}

	if err := finishTask(ctx, tx, task, targetStatus); err != nil {
		return nil, err
	}

	events := []approval.DomainEvent{
		approval.NewTaskApprovedEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.Opinion),
	}

	if node.ApprovalMethod == approval.ApprovalSequential {
		if err := s.activateNextSequentialTask(ctx, tx, instance, node); err != nil {
			return nil, err
		}
	}

	completionEvents, err := s.handleNodeCompletion(ctx, tx, instance, node)
	if err != nil {
		return nil, err
	}

	return append(events, completionEvents...), nil
}

func (s *InstanceService) handleReject(
	ctx context.Context,
	tx orm.DB,
	instance *approval.Instance,
	task *approval.Task,
	node *approval.FlowNode,
	cmd ProcessTaskCmd,
) ([]approval.DomainEvent, error) {
	if err := validateOpinion(node, cmd.Opinion); err != nil {
		return nil, err
	}

	if err := finishTask(ctx, tx, task, approval.TaskRejected); err != nil {
		return nil, err
	}

	events := []approval.DomainEvent{
		approval.NewTaskRejectedEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.Opinion),
	}

	completionEvents, err := s.handleNodeCompletion(ctx, tx, instance, node)
	if err != nil {
		return nil, err
	}

	return append(events, completionEvents...), nil
}

func (s *InstanceService) handleTransfer(
	ctx context.Context,
	tx orm.DB,
	instance *approval.Instance,
	task *approval.Task,
	node *approval.FlowNode,
	cmd ProcessTaskCmd,
) ([]approval.DomainEvent, error) {
	if !node.IsTransferAllowed {
		return nil, ErrTransferNotAllowed
	}

	if cmd.TransferToID == "" {
		return nil, fmt.Errorf("transfer target user ID required")
	}

	if err := finishTask(ctx, tx, task, approval.TaskTransferred); err != nil {
		return nil, err
	}

	newTask := &approval.Task{
		TenantID:   instance.TenantID,
		InstanceID: instance.ID,
		NodeID:     task.NodeID,
		AssigneeID: cmd.TransferToID,
		SortOrder:  task.SortOrder,
		Status:     approval.TaskPending,
		Deadline:   task.Deadline,
	}
	if _, err := tx.NewInsert().Model(newTask).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert transfer task: %w", err)
	}

	return []approval.DomainEvent{
		approval.NewTaskTransferredEvent(task.ID, instance.ID, node.ID, cmd.OperatorID, cmd.TransferToID, cmd.Opinion),
		approval.NewTaskCreatedEvent(newTask.ID, instance.ID, node.ID, cmd.TransferToID, nil),
	}, nil
}

func (s *InstanceService) handleRollback(
	ctx context.Context,
	tx orm.DB,
	instance *approval.Instance,
	task *approval.Task,
	node *approval.FlowNode,
	cmd ProcessTaskCmd,
) ([]approval.DomainEvent, error) {
	if !node.IsRollbackAllowed {
		return nil, ErrRollbackNotAllowed
	}

	if cmd.TargetNodeID == "" {
		return nil, fmt.Errorf("target node ID required for rollback")
	}

	if err := s.validateRollbackTarget(ctx, tx, instance, node, cmd.TargetNodeID); err != nil {
		return nil, err
	}

	if err := finishTask(ctx, tx, task, approval.TaskRollback); err != nil {
		return nil, err
	}

	if err := s.cancelRemainingTasks(ctx, tx, instance.ID, node.ID); err != nil {
		return nil, err
	}

	// Restore form snapshot if rollback data strategy is "keep"
	if node.RollbackDataStrategy == approval.RollbackDataKeep {
		var snapshot approval.FormSnapshot

		err := tx.NewSelect().Model(&snapshot).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instance.ID)
			c.Equals("node_id", cmd.TargetNodeID)
		}).Scan(ctx)

		switch {
		case err == nil && snapshot.FormData != nil:
			instance.FormData = snapshot.FormData
		case err != nil && !result.IsRecordNotFound(err):
			return nil, fmt.Errorf("load form snapshot: %w", err)
		}
	}

	instance.CurrentNodeID = new(cmd.TargetNodeID)

	var targetNode approval.FlowNode

	if err := tx.NewSelect().Model(&targetNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.TargetNodeID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("find target node: %w", err)
	}

	if err := s.engine.ProcessNode(ctx, tx, instance, &targetNode); err != nil {
		return nil, fmt.Errorf("process rollback target node: %w", err)
	}

	return []approval.DomainEvent{
		approval.NewInstanceRolledBackEvent(instance.ID, node.ID, cmd.TargetNodeID, cmd.OperatorID),
	}, nil
}

// validateOpinion checks if an opinion is required but missing.
func validateOpinion(node *approval.FlowNode, opinion string) error {
	if node.IsOpinionRequired && opinion == "" {
		return ErrOpinionRequired
	}

	return nil
}

// finishTask transitions a task to the given status and sets its FinishedAt timestamp.
func finishTask(ctx context.Context, tx orm.DB, task *approval.Task, status approval.TaskStatus) error {
	if !engine.TaskStateMachine.CanTransition(task.Status, status) {
		return ErrInvalidTaskTransition
	}

	task.Status = status
	task.FinishedAt = new(timex.Now())

	if _, err := tx.NewUpdate().Model(task).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	return nil
}

// activateNextSequentialTask activates the next waiting task in sequential approval.
func (s *InstanceService) activateNextSequentialTask(ctx context.Context, tx orm.DB, instance *approval.Instance, node *approval.FlowNode) error {
	var nextTask approval.Task

	err := tx.NewSelect().Model(&nextTask).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instance.ID)
		c.Equals("node_id", node.ID)
		c.Equals("status", approval.TaskWaiting)
	}).OrderBy("sort_order").Limit(1).Scan(ctx)

	if err != nil {
		if result.IsRecordNotFound(err) {
			return nil
		}

		return fmt.Errorf("find next sequential task: %w", err)
	}

	if !engine.TaskStateMachine.CanTransition(approval.TaskWaiting, approval.TaskPending) {
		return nil
	}

	nextTask.Status = approval.TaskPending

	_, err = tx.NewUpdate().Model(&nextTask).WherePK().Exec(ctx)

	return err
}

// cancelRemainingTasks cancels all pending/waiting tasks on the given node.
func (s *InstanceService) cancelRemainingTasks(ctx context.Context, tx orm.DB, instanceID, nodeID string) error {
	_, err := tx.NewUpdate().Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.Equals("node_id", nodeID)
			c.In("status", cancelableTaskStatuses)
		}).Exec(ctx)

	return err
}

// cancelInstanceTasks cancels all pending/waiting tasks for an entire instance.
func cancelInstanceTasks(ctx context.Context, tx orm.DB, instanceID string) error {
	_, err := tx.NewUpdate().Model((*approval.Task)(nil)).
		Set("status", approval.TaskCanceled).
		Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.In("status", cancelableTaskStatuses)
		}).Exec(ctx)

	return err
}

var cancelableTaskStatuses = []string{string(approval.TaskPending), string(approval.TaskWaiting)}

// validateRollbackTarget validates the rollback target node based on the node's RollbackType.
func (s *InstanceService) validateRollbackTarget(ctx context.Context, tx orm.DB, instance *approval.Instance, currentNode *approval.FlowNode, targetNodeID string) error {
	switch currentNode.RollbackType {
	case approval.RollbackNone:
		return ErrRollbackNotAllowed

	case approval.RollbackPrevious:
		// Target must be a direct predecessor node (source node of any edge targeting the current node)
		count, err := tx.NewSelect().Model((*approval.FlowEdge)(nil)).Where(func(c orm.ConditionBuilder) {
			c.Equals("source_node_id", targetNodeID)
			c.Equals("target_node_id", currentNode.ID)
			c.Equals("flow_version_id", instance.FlowVersionID)
		}).Count(ctx)
		if err != nil {
			return fmt.Errorf("find previous node: %w", err)
		}

		if count == 0 {
			return ErrInvalidRollbackTarget
		}

	case approval.RollbackStart:
		// Target must be the start node of the flow
		var startNode approval.FlowNode

		if err := tx.NewSelect().Model(&startNode).Where(func(c orm.ConditionBuilder) {
			c.Equals("flow_version_id", instance.FlowVersionID)
			c.Equals("node_kind", string(approval.NodeStart))
		}).Scan(ctx); err != nil {
			return fmt.Errorf("find start node: %w", err)
		}

		if startNode.ID != targetNodeID {
			return ErrInvalidRollbackTarget
		}

	case approval.RollbackAny:
		// Any node in the same flow version is a valid target
		count, err := tx.NewSelect().Model((*approval.FlowNode)(nil)).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", targetNodeID)
			c.Equals("flow_version_id", instance.FlowVersionID)
		}).Count(ctx)
		if err != nil {
			return fmt.Errorf("find rollback target node: %w", err)
		}

		if count == 0 {
			return ErrInvalidRollbackTarget
		}
	}

	return nil
}

// handleNodeCompletion evaluates node completion and handles the result.
// On PassRulePassed: advances to the next node and cancels remaining tasks.
// On PassRuleRejected: marks instance as rejected, cancels remaining tasks, and resumes parent flow if applicable.
func (s *InstanceService) handleNodeCompletion(
	ctx context.Context,
	tx orm.DB,
	instance *approval.Instance,
	node *approval.FlowNode,
) ([]approval.DomainEvent, error) {
	completionResult, err := s.engine.EvaluateNodeCompletion(ctx, tx, instance, node)
	if err != nil {
		return nil, fmt.Errorf("evaluate node completion: %w", err)
	}

	switch completionResult {
	case approval.PassRulePassed:
		if err := s.triggerNodeCC(ctx, tx, instance, node, approval.PassRulePassed); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		if err := s.engine.AdvanceToNextNode(ctx, tx, instance, node, ""); err != nil {
			return nil, fmt.Errorf("advance to next node: %w", err)
		}

		if err := s.cancelRemainingTasks(ctx, tx, instance.ID, node.ID); err != nil {
			return nil, err
		}

		return nil, nil

	case approval.PassRuleRejected:
		if err := s.triggerNodeCC(ctx, tx, instance, node, approval.PassRuleRejected); err != nil {
			return nil, fmt.Errorf("trigger node cc: %w", err)
		}

		instance.Status = approval.InstanceRejected
		instance.FinishedAt = new(timex.Now())

		if err := s.cancelRemainingTasks(ctx, tx, instance.ID, node.ID); err != nil {
			return nil, err
		}

		if err := s.engine.ResumeParentFlow(ctx, tx, instance, approval.InstanceRejected); err != nil {
			return nil, fmt.Errorf("resume parent flow: %w", err)
		}

		return []approval.DomainEvent{
			approval.NewInstanceCompletedEvent(instance.ID, approval.InstanceRejected),
		}, nil

	default:
		return nil, nil
	}
}

// triggerNodeCC creates CC records when a node completes, based on CCTiming configuration.
// completionResult determines which CC timings are triggered:
//   - PassRulePassed: triggers CCTimingAlways + CCTimingOnApprove
//   - PassRuleRejected: triggers CCTimingAlways + CCTimingOnReject
func (s *InstanceService) triggerNodeCC(ctx context.Context, tx orm.DB, instance *approval.Instance, node *approval.FlowNode, completionResult approval.PassRuleResult) error {
	var ccConfigs []approval.FlowNodeCC

	if err := tx.NewSelect().Model(&ccConfigs).Where(func(c orm.ConditionBuilder) {
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

	if _, err := tx.NewInsert().Model(&records).Exec(ctx); err != nil {
		return fmt.Errorf("insert cc records: %w", err)
	}

	return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
		approval.NewCcNotifiedEvent(instance.ID, node.ID, ccUserIDs, false),
	})
}

// Withdraw withdraws an instance.
func (s *InstanceService) Withdraw(ctx context.Context, instanceID, operatorID, reason string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var instance approval.Instance

		if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", instanceID)
		}).Scan(ctx); err != nil {
			return ErrInstanceNotFound
		}

		if instance.ApplicantID != operatorID {
			return ErrNotApplicant
		}

		if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceWithdrawn) {
			return ErrWithdrawNotAllowed
		}

		now := timex.Now()
		instance.Status = approval.InstanceWithdrawn
		instance.FinishedAt = &now

		if _, err := tx.NewUpdate().Model(&instance).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update instance: %w", err)
		}

		if err := cancelInstanceTasks(ctx, tx, instanceID); err != nil {
			return fmt.Errorf("cancel tasks on withdraw: %w", err)
		}

		actionLog := &approval.ActionLog{
			InstanceID: instanceID,
			Action:     approval.ActionWithdraw,
			OperatorID: operatorID,
		}
		if reason != "" {
			actionLog.Opinion = new(reason)
		}
		if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
			return fmt.Errorf("insert action log: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewInstanceWithdrawnEvent(instanceID, operatorID),
		})
	})
}

// AddCC adds CC records for an instance.
func (s *InstanceService) AddCC(ctx context.Context, instanceID string, ccUserIDs []string, operatorID string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var instance approval.Instance

		if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", instanceID)
		}).Scan(ctx); err != nil {
			return ErrInstanceNotFound
		}

		// Validate manual CC is allowed on current node
		if instance.CurrentNodeID != nil {
			var node approval.FlowNode

			if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
				c.Equals("id", *instance.CurrentNodeID)
			}).Scan(ctx); err == nil && !node.IsManualCCAllowed {
				return ErrManualCcNotAllowed
			}
		}

		// Filter out already-existing CC records to avoid duplicates
		var existingCCs []approval.CCRecord

		if err := tx.NewSelect().Model(&existingCCs).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.In("cc_user_id", ccUserIDs)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("query existing cc records: %w", err)
		}

		existingSet := make(map[string]struct{}, len(existingCCs))
		for _, cc := range existingCCs {
			existingSet[cc.CCUserID] = struct{}{}
		}

		records := make([]approval.CCRecord, 0, len(ccUserIDs))

		for _, userID := range ccUserIDs {
			if _, exists := existingSet[userID]; exists {
				continue
			}

			record := approval.CCRecord{
				InstanceID: instanceID,
				CCUserID:   userID,
				IsManual:   true,
			}
			records = append(records, record)
		}

		if len(records) == 0 {
			return nil
		}

		if _, err := tx.NewInsert().Model(&records).Exec(ctx); err != nil {
			return fmt.Errorf("insert cc records: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewCcNotifiedEvent(instanceID, "", ccUserIDs, true),
		})
	})
}

// MarkCCRead marks all unread CC records as read for the given user in the given instance.
// If the CC records belong to a CC node with IsReadConfirmRequired=true and all records are now read,
// the node is advanced to continue the flow.
func (s *InstanceService) MarkCCRead(ctx context.Context, instanceID string, userID string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// 1. Query unread CC records for this user in this instance
		var records []approval.CCRecord
		if err := tx.NewSelect().Model(&records).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.Equals("cc_user_id", userID)
			c.IsNull("read_at")
		}).Scan(ctx); err != nil {
			return fmt.Errorf("query unread cc records: %w", err)
		}

		if len(records) == 0 {
			return nil
		}

		// 2. Batch update read_at
		now := time.Now()
		recordIDs := make([]string, 0, len(records))
		for _, r := range records {
			recordIDs = append(recordIDs, r.ID)
		}

		if _, err := tx.NewUpdate().Model((*approval.CCRecord)(nil)).
			Set("read_at = ?", now).
			Where(func(c orm.ConditionBuilder) {
				c.In("id", recordIDs)
			}).Exec(ctx); err != nil {
			return fmt.Errorf("update cc records read_at: %w", err)
		}

		// 3. Check if any CC node should advance
		return s.checkCCNodeCompletion(ctx, tx, instanceID, records)
	})
}

// checkCCNodeCompletion checks if all CC records for CC nodes are read and advances the flow.
func (s *InstanceService) checkCCNodeCompletion(ctx context.Context, tx orm.DB, instanceID string, records []approval.CCRecord) error {
	// Collect unique CC node IDs
	nodeIDs := make(map[string]struct{})
	for _, r := range records {
		if r.NodeID != nil {
			nodeIDs[*r.NodeID] = struct{}{}
		}
	}

	for nodeID := range nodeIDs {
		var node approval.FlowNode
		if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", nodeID)
		}).Scan(ctx); err != nil {
			continue
		}

		// Only process CC nodes with read confirmation required
		if node.NodeKind != approval.NodeCC || !node.IsReadConfirmRequired {
			continue
		}

		// Count remaining unread records for this node in this instance
		unreadCount, err := tx.NewSelect().Model((*approval.CCRecord)(nil)).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instanceID)
			c.Equals("node_id", nodeID)
			c.IsNull("read_at")
		}).Count(ctx)
		if err != nil {
			return fmt.Errorf("count unread cc records: %w", err)
		}

		if unreadCount == 0 {
			// All read — advance the flow
			var instance approval.Instance
			if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
				c.Equals("id", instanceID)
			}).Scan(ctx); err != nil {
				return fmt.Errorf("find instance for cc advance: %w", err)
			}

			if err := s.engine.AdvanceToNextNode(ctx, tx, &instance, &node, ""); err != nil {
				return fmt.Errorf("advance cc node: %w", err)
			}
		}
	}

	return nil
}

// AddAssignee dynamically adds assignees to a task.
func (s *InstanceService) AddAssignee(ctx context.Context, cmd AddAssigneeCmd) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var instance approval.Instance

		if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.InstanceID)
		}).Scan(ctx); err != nil {
			return ErrInstanceNotFound
		}

		if instance.Status != approval.InstanceRunning {
			return ErrInstanceCompleted
		}

		var task approval.Task

		if err := tx.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.TaskID)
			c.Equals("instance_id", cmd.InstanceID)
		}).Scan(ctx); err != nil {
			return ErrTaskNotFound
		}

		var node approval.FlowNode

		if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", task.NodeID)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("load node: %w", err)
		}

		if !node.IsAddAssigneeAllowed {
			return ErrAddAssigneeNotAllowed
		}

		if task.AssigneeID != cmd.OperatorID {
			return ErrNotAssignee
		}

		addType := approval.AddAssigneeType(cmd.AddType)

		if !addType.IsValid() {
			return ErrInvalidAddAssigneeType
		}

		if len(node.AddAssigneeTypes) > 0 && !slices.Contains(node.AddAssigneeTypes, cmd.AddType) {
			return ErrInvalidAddAssigneeType
		}

		// Find current max sort_order for this node to avoid collisions
		var lastTask approval.Task

		baseSortOrder := task.SortOrder
		if err := tx.NewSelect().Model(&lastTask).Where(func(c orm.ConditionBuilder) {
			c.Equals("instance_id", instance.ID)
			c.Equals("node_id", task.NodeID)
		}).OrderByDesc("sort_order").Limit(1).Scan(ctx); err == nil {
			baseSortOrder = lastTask.SortOrder
		}

		for i, userID := range cmd.UserIDs {
			newTask := &approval.Task{
				TenantID:        instance.TenantID,
				InstanceID:      instance.ID,
				NodeID:          task.NodeID,
				AssigneeID:      userID,
				SortOrder:       baseSortOrder + i + 1,
				ParentTaskID:    new(task.ID),
				AddAssigneeType: new(string(addType)),
			}
			switch addType {
			case approval.AddAssigneeBefore:
				newTask.Status = approval.TaskPending
				if engine.TaskStateMachine.CanTransition(task.Status, approval.TaskWaiting) {
					task.Status = approval.TaskWaiting

					if _, err := tx.NewUpdate().Model(&task).WherePK().Exec(ctx); err != nil {
						return fmt.Errorf("update original task: %w", err)
					}
				}

			case approval.AddAssigneeAfter:
				newTask.Status = approval.TaskWaiting

			case approval.AddAssigneeParallel:
				newTask.Status = approval.TaskPending
			}

			if _, err := tx.NewInsert().Model(newTask).Exec(ctx); err != nil {
				return fmt.Errorf("insert assignee task: %w", err)
			}
		}

		// Action log
		actionLog := &approval.ActionLog{
			InstanceID:       instance.ID,
			NodeID:           new(task.NodeID),
			TaskID:           new(task.ID),
			Action:           approval.ActionAddAssignee,
			OperatorID:       cmd.OperatorID,
			AddAssigneeType:  new(cmd.AddType),
			AddAssigneeToIDs: cmd.UserIDs,
		}
		if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
			return fmt.Errorf("insert action log: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewAssigneesAddedEvent(instance.ID, task.NodeID, task.ID, addType, cmd.UserIDs),
		})
	})
}

// RemoveAssignee removes an assignee by canceling their task.
func (s *InstanceService) RemoveAssignee(ctx context.Context, taskID, operatorID string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var task approval.Task

		if err := tx.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", taskID)
		}).Scan(ctx); err != nil {
			return ErrTaskNotFound
		}

		var node approval.FlowNode

		if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", task.NodeID)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("load node: %w", err)
		}

		if !node.IsRemoveAssigneeAllowed {
			return ErrRemoveAssigneeNotAllowed
		}

		// Validate operator: must be a peer assignee on the same node or a flow admin
		if !s.isAuthorizedForNodeOperation(ctx, tx, task, operatorID) {
			return ErrNotAssignee
		}

		canRemoveWithoutDeadlock, err := s.canRemoveAssigneeTask(ctx, tx, &node, task)
		if err != nil {
			return err
		}

		if !canRemoveWithoutDeadlock {
			return ErrLastAssigneeRemoval
		}

		originalStatus := task.Status
		if err := finishTask(ctx, tx, &task, approval.TaskRemoved); err != nil {
			return err
		}

		var instance approval.Instance
		if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", task.InstanceID)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("load instance: %w", err)
		}

		if node.ApprovalMethod == approval.ApprovalSequential && originalStatus == approval.TaskPending {
			if err := s.activateNextSequentialTask(ctx, tx, &instance, &node); err != nil {
				return err
			}
		}

		// Action log
		actionLog := &approval.ActionLog{
			InstanceID:        task.InstanceID,
			NodeID:            new(task.NodeID),
			TaskID:            new(task.ID),
			Action:            approval.ActionRemoveAssignee,
			OperatorID:        operatorID,
			RemoveAssigneeIDs: []string{task.AssigneeID},
		}
		if _, err := tx.NewInsert().Model(actionLog).Exec(ctx); err != nil {
			return fmt.Errorf("insert action log: %w", err)
		}

		events := []approval.DomainEvent{
			approval.NewAssigneesRemovedEvent(task.InstanceID, task.NodeID, task.ID, []string{task.AssigneeID}),
		}

		completionEvents, err := s.handleNodeCompletion(ctx, tx, &instance, &node)
		if err != nil {
			return err
		}

		events = append(events, completionEvents...)

		if _, err := tx.NewUpdate().Model(&instance).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update instance: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, events)
	})
}

// canRemoveAssigneeTask determines whether removing a task can still drive the
// node to progress (either through remaining actionable tasks or immediate
// completion under pass-rule evaluation).
func (s *InstanceService) canRemoveAssigneeTask(ctx context.Context, tx orm.DB, node *approval.FlowNode, task approval.Task) (bool, error) {
	var tasks []approval.Task
	if err := tx.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", task.InstanceID)
		c.Equals("node_id", task.NodeID)
	}).Scan(ctx); err != nil {
		return false, fmt.Errorf("query node tasks: %w", err)
	}

	hasOtherActionable := false
	simulatedTasks := make([]approval.Task, 0, len(tasks))
	for _, current := range tasks {
		if current.ID == task.ID {
			current.Status = approval.TaskRemoved
		} else if current.Status == approval.TaskPending || current.Status == approval.TaskWaiting {
			hasOtherActionable = true
		}

		simulatedTasks = append(simulatedTasks, current)
	}

	if hasOtherActionable {
		return true, nil
	}

	result, err := s.engine.EvaluatePassRuleWithTasks(node, simulatedTasks)
	if err != nil {
		return false, err
	}

	return result != approval.PassRulePending, nil
}

// isAuthorizedForNodeOperation checks if the operator is authorized to perform
// node-level operations (e.g., remove assignee). Returns true if the operator
// is a peer assignee on the same node or a flow admin.
func (s *InstanceService) isAuthorizedForNodeOperation(ctx context.Context, tx orm.DB, task approval.Task, operatorID string) bool {
	// Check if operator is a peer assignee on the same node
	peerCount, err := tx.NewSelect().Model((*approval.Task)(nil)).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", task.InstanceID)
		c.Equals("node_id", task.NodeID)
		c.Equals("assignee_id", operatorID)
		c.In("status", []string{string(approval.TaskPending), string(approval.TaskWaiting)})
	}).Count(ctx)
	if err == nil && peerCount > 0 {
		return true
	}

	// Check if operator is a flow admin
	var instance approval.Instance

	if err := tx.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", task.InstanceID)
	}).Scan(ctx); err != nil {
		return false
	}

	var flow approval.Flow

	if err := tx.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instance.FlowID)
	}).Scan(ctx); err != nil {
		return false
	}

	return slices.Contains(flow.AdminUserIDs, operatorID)
}

// checkInitiationPermission checks if the applicant is allowed to initiate the flow.
func (s *InstanceService) checkInitiationPermission(ctx context.Context, tx orm.DB, flowID, applicantID string, applicantDeptID *string) (bool, error) {
	var initiators []approval.FlowInitiator

	if err := tx.NewSelect().Model(&initiators).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flowID)
	}).Scan(ctx); err != nil {
		return false, fmt.Errorf("query flow initiators: %w", err)
	}

	// No initiator configs means no one can initiate (IsAllInitiateAllowed is already false)
	if len(initiators) == 0 {
		return false, nil
	}

	for _, ini := range initiators {
		switch ini.Kind {
		case approval.InitiatorUser:
			if slices.Contains(ini.IDs, applicantID) {
				return true, nil
			}

		case approval.InitiatorDept:
			if applicantDeptID == nil {
				continue
			}

			if slices.Contains(ini.IDs, *applicantDeptID) {
				return true, nil
			}

		case approval.InitiatorRole:
			if s.assigneeService == nil {
				continue
			}

			for _, roleID := range ini.IDs {
				users, err := s.assigneeService.GetRoleUsers(ctx, roleID)
				if err != nil {
					return false, fmt.Errorf("get users by role %s: %w", roleID, err)
				}

				if slices.Contains(users, applicantID) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// filterEditableFormData filters form data to only include fields that are editable or required
// based on the node's field permissions configuration.
// Fields without explicit permission are rejected (deny-by-default).
func filterEditableFormData(formData map[string]any, permissions map[string]approval.Permission) map[string]any {
	if len(permissions) == 0 {
		return formData
	}

	filtered := make(map[string]any, len(formData))

	for k, v := range formData {
		perm, hasPerm := permissions[k]
		if !hasPerm {
			continue
		}

		if perm == approval.PermissionEditable || perm == approval.PermissionRequired {
			filtered[k] = v
		}
	}

	return filtered
}

// UrgeTask sends an urge notification for a pending task with cooldown enforcement.
func (s *InstanceService) UrgeTask(ctx context.Context, cmd UrgeTaskCmd) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Load the task
		var task approval.Task
		if err := tx.NewSelect().Model(&task).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.TaskID)
			c.Equals("instance_id", cmd.InstanceID)
		}).Scan(ctx); err != nil {
			return ErrTaskNotFound
		}

		// Only pending or waiting tasks can be urged
		if task.Status != approval.TaskPending && task.Status != approval.TaskWaiting {
			return ErrTaskNotPending
		}

		// Load node to check cooldown settings
		var node approval.FlowNode
		if err := tx.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", task.NodeID)
		}).Scan(ctx); err != nil {
			return fmt.Errorf("load node: %w", err)
		}

		// Enforce cooldown: check if a recent urge exists within the cooldown window
		cooldownMinutes := node.UrgeCooldownMinutes
		if cooldownMinutes <= 0 {
			cooldownMinutes = 30 // default 30 minutes
		}

		cooldownSince := timex.DateTime(time.Now().Add(-time.Duration(cooldownMinutes) * time.Minute))

		existingCount, err := tx.NewSelect().Model((*approval.UrgeRecord)(nil)).Where(func(c orm.ConditionBuilder) {
			c.Equals("task_id", cmd.TaskID)
			c.Equals("urger_id", cmd.UrgerID)
			c.GreaterThan("created_at", cooldownSince)
		}).Count(ctx)
		if err != nil {
			return fmt.Errorf("check urge cooldown: %w", err)
		}

		if existingCount > 0 {
			return result.Err(
				fmt.Sprintf("催办操作过于频繁，请 %d 分钟后再试", cooldownMinutes),
				result.WithCode(ErrCodeUrgeCooldown),
			)
		}

		// Insert urge record
		record := &approval.UrgeRecord{
			InstanceID:   cmd.InstanceID,
			NodeID:       task.NodeID,
			TaskID:       new(cmd.TaskID),
			UrgerID:      cmd.UrgerID,
			TargetUserID: task.AssigneeID,
			Message:      cmd.Message,
		}

		if _, err := tx.NewInsert().Model(record).Exec(ctx); err != nil {
			return fmt.Errorf("insert urge record: %w", err)
		}

		// Publish urge event
		event := approval.NewTaskUrgedEvent(
			cmd.InstanceID, task.NodeID, cmd.TaskID,
			cmd.UrgerID, task.AssigneeID, cmd.Message,
		)

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{event})
	})
}

// renderTitle renders an instance title from a Go text/template string.
// Templates use camelCase keys: {{.flowName}}, {{.flowCode}}, {{.instanceNo}},
// {{.applicantId}}, and form values via {{index .formData "fieldName"}}.
func renderTitle(titleTemplate string, data map[string]any) (string, error) {
	if titleTemplate == "" {
		return data["flowName"].(string) + "-" + data["instanceNo"].(string), nil
	}

	tmpl, err := template.New("title").Parse(titleTemplate)
	if err != nil {
		return "", fmt.Errorf("parse title template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute title template: %w", err)
	}

	return buf.String(), nil
}
