package approval

import (
	"context"

	"github.com/ilxqx/vef-framework-go/timex"
)

// DomainEvent is the base interface for all approval domain events.
type DomainEvent interface {
	// EventName returns the unique event identifier (e.g., "approval.instance.created").
	EventName() string
	// OccurredAt returns the timestamp when the event occurred.
	OccurredAt() timex.DateTime
}

// EventDispatcher dispatches outbox events to external systems.
// Default implementation forwards to event.Bus.
type EventDispatcher interface {
	// Dispatch sends an outbox event record to the external event system.
	Dispatch(ctx context.Context, record EventOutbox) error
}

// ==================== Instance Events ====================

// InstanceCreatedEvent fired when a new instance is created.
type InstanceCreatedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FlowID       string         `json:"flowId"`
	Title        string         `json:"title"`
	ApplicantID  string         `json:"applicantId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceCreatedEvent(instanceID, flowID, title, applicantID string) *InstanceCreatedEvent {
	return &InstanceCreatedEvent{
		InstanceID:   instanceID,
		FlowID:       flowID,
		Title:        title,
		ApplicantID:  applicantID,
		OccurredTime: timex.Now(),
	}
}

func (e *InstanceCreatedEvent) EventName() string          { return "approval.instance.created" }
func (e *InstanceCreatedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceCompletedEvent fired when instance reaches a final status.
type InstanceCompletedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FinalStatus  InstanceStatus `json:"finalStatus"`
	FinishedAt   timex.DateTime `json:"finishedAt"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceCompletedEvent(instanceID string, finalStatus InstanceStatus) *InstanceCompletedEvent {
	now := timex.Now()

	return &InstanceCompletedEvent{
		InstanceID:   instanceID,
		FinalStatus:  finalStatus,
		FinishedAt:   now,
		OccurredTime: now,
	}
}

func (e *InstanceCompletedEvent) EventName() string          { return "approval.instance.completed" }
func (e *InstanceCompletedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceWithdrawnEvent fired when applicant withdraws the instance.
type InstanceWithdrawnEvent struct {
	InstanceID   string         `json:"instanceId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceWithdrawnEvent(instanceID, operatorID string) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceID:   instanceID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (e *InstanceWithdrawnEvent) EventName() string          { return "approval.instance.withdrawn" }
func (e *InstanceWithdrawnEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceRolledBackEvent fired when instance is rolled back.
type InstanceRolledBackEvent struct {
	InstanceID   string         `json:"instanceId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceRolledBackEvent(instanceID, fromNodeID, toNodeID, operatorID string) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceID:   instanceID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (e *InstanceRolledBackEvent) EventName() string          { return "approval.instance.rollback" }
func (e *InstanceRolledBackEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceReturnedEvent fired when instance is returned to the initiator.
type InstanceReturnedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FromNodeID   string         `json:"fromNodeId"`
	ToNodeID     string         `json:"toNodeId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceReturnedEvent(instanceID, fromNodeID, toNodeID, operatorID string) *InstanceReturnedEvent {
	return &InstanceReturnedEvent{
		InstanceID:   instanceID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (e *InstanceReturnedEvent) EventName() string          { return "approval.instance.returned" }
func (e *InstanceReturnedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// InstanceResubmittedEvent fired when the initiator resubmits a returned instance.
type InstanceResubmittedEvent struct {
	InstanceID   string         `json:"instanceId"`
	OperatorID   string         `json:"operatorId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewInstanceResubmittedEvent(instanceID, operatorID string) *InstanceResubmittedEvent {
	return &InstanceResubmittedEvent{
		InstanceID:   instanceID,
		OperatorID:   operatorID,
		OccurredTime: timex.Now(),
	}
}

func (e *InstanceResubmittedEvent) EventName() string          { return "approval.instance.resubmitted" }
func (e *InstanceResubmittedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Node Events ====================

// NodeEnteredEvent fired when instance enters a new node.
type NodeEnteredEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	NodeName     string         `json:"nodeName"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeEnteredEvent(instanceID, nodeID, nodeName string) *NodeEnteredEvent {
	return &NodeEnteredEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		NodeName:     nodeName,
		OccurredTime: timex.Now(),
	}
}

func (e *NodeEnteredEvent) EventName() string          { return "approval.node.entered" }
func (e *NodeEnteredEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// NodeAutoPassedEvent fired when a node is auto-passed.
type NodeAutoPassedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	Reason       string         `json:"reason"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewNodeAutoPassedEvent(instanceID, nodeID, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		Reason:       reason,
		OccurredTime: timex.Now(),
	}
}

func (e *NodeAutoPassedEvent) EventName() string          { return "approval.node.auto_passed" }
func (e *NodeAutoPassedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ParallelJoinedEvent fired when parallel branches are joined.
type ParallelJoinedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewParallelJoinedEvent(instanceID, nodeID string) *ParallelJoinedEvent {
	return &ParallelJoinedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OccurredTime: timex.Now(),
	}
}

func (e *ParallelJoinedEvent) EventName() string          { return "approval.node.parallel_joined" }
func (e *ParallelJoinedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Task Events ====================

// TaskCreatedEvent fired when a new task is created.
type TaskCreatedEvent struct {
	TaskID       string          `json:"taskId"`
	InstanceID   string          `json:"instanceId"`
	NodeID       string          `json:"nodeId"`
	AssigneeID   string          `json:"assigneeId"`
	Deadline     *timex.DateTime `json:"deadline,omitempty"`
	OccurredTime timex.DateTime  `json:"occurredTime"`
}

func NewTaskCreatedEvent(taskID, instanceID, nodeID, assigneeID string, deadline *timex.DateTime) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (e *TaskCreatedEvent) EventName() string          { return "approval.task.created" }
func (e *TaskCreatedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskApprovedEvent fired when a task is approved.
type TaskApprovedEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskApprovedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskApprovedEvent {
	return &TaskApprovedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      new(opinion),
		OccurredTime: timex.Now(),
	}
}

func (e *TaskApprovedEvent) EventName() string          { return "approval.task.approved" }
func (e *TaskApprovedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskRejectedEvent fired when a task is rejected.
type TaskRejectedEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	OperatorID   string         `json:"operatorId"`
	Opinion      *string        `json:"opinion,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskRejectedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskRejectedEvent {
	return &TaskRejectedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      new(opinion),
		OccurredTime: timex.Now(),
	}
}

func (e *TaskRejectedEvent) EventName() string          { return "approval.task.rejected" }
func (e *TaskRejectedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskTransferredEvent fired when a task is transferred.
type TaskTransferredEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	FromUserID   string         `json:"fromUserId"`
	ToUserID     string         `json:"toUserId"`
	Reason       *string        `json:"reason,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskTransferredEvent(taskID, instanceID, nodeID, fromUserID, toUserID, reason string) *TaskTransferredEvent {
	return &TaskTransferredEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		ToUserID:     toUserID,
		Reason:       new(reason),
		OccurredTime: timex.Now(),
	}
}

func (e *TaskTransferredEvent) EventName() string          { return "approval.task.transferred" }
func (e *TaskTransferredEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskTimeoutEvent fired when a task times out.
type TaskTimeoutEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	Deadline     timex.DateTime `json:"deadline"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskTimeoutEvent(taskID, instanceID, nodeID, assigneeID string, deadline timex.DateTime) *TaskTimeoutEvent {
	return &TaskTimeoutEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		OccurredTime: timex.Now(),
	}
}

func (e *TaskTimeoutEvent) EventName() string          { return "approval.task.timeout" }
func (e *TaskTimeoutEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// AssigneesAddedEvent fired when assignees are dynamically added.
type AssigneesAddedEvent struct {
	InstanceID   string          `json:"instanceId"`
	NodeID       string          `json:"nodeId"`
	TaskID       string          `json:"taskId"`
	AddType      AddAssigneeType `json:"addType"`
	AssigneeIDs  []string        `json:"assigneeIds"`
	OccurredTime timex.DateTime  `json:"occurredTime"`
}

func NewAssigneesAddedEvent(instanceID, nodeID, taskID string, addType AddAssigneeType, assigneeIDs []string) *AssigneesAddedEvent {
	return &AssigneesAddedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		AddType:      addType,
		AssigneeIDs:  assigneeIDs,
		OccurredTime: timex.Now(),
	}
}

func (e *AssigneesAddedEvent) EventName() string          { return "approval.task.assignee_added" }
func (e *AssigneesAddedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// AssigneesRemovedEvent fired when assignees are dynamically removed.
type AssigneesRemovedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	TaskID       string         `json:"taskId"`
	AssigneeIDs  []string       `json:"assigneeIds"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewAssigneesRemovedEvent(instanceID, nodeID, taskID string, assigneeIDs []string) *AssigneesRemovedEvent {
	return &AssigneesRemovedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		AssigneeIDs:  assigneeIDs,
		OccurredTime: timex.Now(),
	}
}

func (e *AssigneesRemovedEvent) EventName() string          { return "approval.task.assignee_removed" }
func (e *AssigneesRemovedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== CC Events ====================

// CCNotifiedEvent fired when users are carbon-copied.
type CCNotifiedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	CcUserIDs    []string       `json:"ccUserIds"`
	IsManual     bool           `json:"isManual"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewCCNotifiedEvent(instanceID, nodeID string, ccUserIDs []string, isManual bool) *CCNotifiedEvent {
	return &CCNotifiedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		CcUserIDs:    ccUserIDs,
		IsManual:     isManual,
		OccurredTime: timex.Now(),
	}
}

func (e *CCNotifiedEvent) EventName() string          { return "approval.cc.notified" }
func (e *CCNotifiedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Flow Events ====================

// FlowPublishedEvent fired when a flow version is published.
type FlowPublishedEvent struct {
	FlowID       string         `json:"flowId"`
	VersionID    string         `json:"versionId"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewFlowPublishedEvent(flowID, versionID string) *FlowPublishedEvent {
	return &FlowPublishedEvent{
		FlowID:       flowID,
		VersionID:    versionID,
		OccurredTime: timex.Now(),
	}
}

func (e *FlowPublishedEvent) EventName() string          { return "approval.flow.published" }
func (e *FlowPublishedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// ==================== Timeout & Urge Events ====================

// TaskDeadlineWarningEvent fired when a task is approaching its deadline.
type TaskDeadlineWarningEvent struct {
	TaskID       string         `json:"taskId"`
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	AssigneeID   string         `json:"assigneeId"`
	Deadline     timex.DateTime `json:"deadline"`
	HoursLeft    int            `json:"hoursLeft"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskDeadlineWarningEvent(taskID, instanceID, nodeID, assigneeID string, deadline timex.DateTime, hoursLeft int) *TaskDeadlineWarningEvent {
	return &TaskDeadlineWarningEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		HoursLeft:    hoursLeft,
		OccurredTime: timex.Now(),
	}
}

func (e *TaskDeadlineWarningEvent) EventName() string          { return "approval.task.deadline_warning" }
func (e *TaskDeadlineWarningEvent) OccurredAt() timex.DateTime { return e.OccurredTime }

// TaskUrgedEvent fired when a task assignee is urged/reminded.
type TaskUrgedEvent struct {
	InstanceID   string         `json:"instanceId"`
	NodeID       string         `json:"nodeId"`
	TaskID       string         `json:"taskId"`
	UrgerID      string         `json:"urgerId"`
	TargetUserID string         `json:"targetUserId"`
	Message      *string        `json:"message,omitempty"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

func NewTaskUrgedEvent(instanceID, nodeID, taskID, urgerID, targetUserID, message string) *TaskUrgedEvent {
	return &TaskUrgedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		UrgerID:      urgerID,
		TargetUserID: targetUserID,
		Message:      new(message),
		OccurredTime: timex.Now(),
	}
}

func (e *TaskUrgedEvent) EventName() string          { return "approval.task.urged" }
func (e *TaskUrgedEvent) OccurredAt() timex.DateTime { return e.OccurredTime }
