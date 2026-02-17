package approval

import "time"

// DomainEvent is the base interface for all approval domain events.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// ==================== Instance Events ====================

// InstanceCreatedEvent fired when a new instance is created.
type InstanceCreatedEvent struct {
	InstanceID   string    `json:"instanceId"`
	FlowID       string    `json:"flowId"`
	Title        string    `json:"title"`
	ApplicantID  string    `json:"applicantId"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewInstanceCreatedEvent(instanceID, flowID, title, applicantID string) *InstanceCreatedEvent {
	return &InstanceCreatedEvent{
		InstanceID:   instanceID,
		FlowID:       flowID,
		Title:        title,
		ApplicantID:  applicantID,
		OccurredTime: time.Now(),
	}
}

func (e *InstanceCreatedEvent) EventName() string     { return "approval.instance.created" }
func (e *InstanceCreatedEvent) OccurredAt() time.Time { return e.OccurredTime }

// InstanceCompletedEvent fired when instance reaches a final status.
type InstanceCompletedEvent struct {
	InstanceID   string         `json:"instanceId"`
	FinalStatus  InstanceStatus `json:"finalStatus"`
	FinishedAt   time.Time      `json:"finishedAt"`
	OccurredTime time.Time      `json:"occurredTime"`
}

func NewInstanceCompletedEvent(instanceID string, finalStatus InstanceStatus) *InstanceCompletedEvent {
	now := time.Now()

	return &InstanceCompletedEvent{
		InstanceID:   instanceID,
		FinalStatus:  finalStatus,
		FinishedAt:   now,
		OccurredTime: now,
	}
}

func (e *InstanceCompletedEvent) EventName() string     { return "approval.instance.completed" }
func (e *InstanceCompletedEvent) OccurredAt() time.Time { return e.OccurredTime }

// InstanceWithdrawnEvent fired when applicant withdraws the instance.
type InstanceWithdrawnEvent struct {
	InstanceID   string    `json:"instanceId"`
	OperatorID   string    `json:"operatorId"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewInstanceWithdrawnEvent(instanceID, operatorID string) *InstanceWithdrawnEvent {
	return &InstanceWithdrawnEvent{
		InstanceID:   instanceID,
		OperatorID:   operatorID,
		OccurredTime: time.Now(),
	}
}

func (e *InstanceWithdrawnEvent) EventName() string     { return "approval.instance.withdrawn" }
func (e *InstanceWithdrawnEvent) OccurredAt() time.Time { return e.OccurredTime }

// InstanceRolledBackEvent fired when instance is rolled back.
type InstanceRolledBackEvent struct {
	InstanceID   string    `json:"instanceId"`
	FromNodeID   string    `json:"fromNodeId"`
	ToNodeID     string    `json:"toNodeId"`
	OperatorID   string    `json:"operatorId"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewInstanceRolledBackEvent(instanceID, fromNodeID, toNodeID, operatorID string) *InstanceRolledBackEvent {
	return &InstanceRolledBackEvent{
		InstanceID:   instanceID,
		FromNodeID:   fromNodeID,
		ToNodeID:     toNodeID,
		OperatorID:   operatorID,
		OccurredTime: time.Now(),
	}
}

func (e *InstanceRolledBackEvent) EventName() string     { return "approval.instance.rollback" }
func (e *InstanceRolledBackEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== Node Events ====================

// NodeEnteredEvent fired when instance enters a new node.
type NodeEnteredEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	NodeName     string    `json:"nodeName"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewNodeEnteredEvent(instanceID, nodeID, nodeName string) *NodeEnteredEvent {
	return &NodeEnteredEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		NodeName:     nodeName,
		OccurredTime: time.Now(),
	}
}

func (e *NodeEnteredEvent) EventName() string     { return "approval.node.entered" }
func (e *NodeEnteredEvent) OccurredAt() time.Time { return e.OccurredTime }

// NodeAutoPassedEvent fired when a node is auto-passed.
type NodeAutoPassedEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	Reason       string    `json:"reason"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewNodeAutoPassedEvent(instanceID, nodeID, reason string) *NodeAutoPassedEvent {
	return &NodeAutoPassedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		Reason:       reason,
		OccurredTime: time.Now(),
	}
}

func (e *NodeAutoPassedEvent) EventName() string     { return "approval.node.auto_passed" }
func (e *NodeAutoPassedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ParallelJoinedEvent fired when parallel branches are joined.
type ParallelJoinedEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewParallelJoinedEvent(instanceID, nodeID string) *ParallelJoinedEvent {
	return &ParallelJoinedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OccurredTime: time.Now(),
	}
}

func (e *ParallelJoinedEvent) EventName() string     { return "approval.node.parallel_joined" }
func (e *ParallelJoinedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== Task Events ====================

// TaskCreatedEvent fired when a new task is created.
type TaskCreatedEvent struct {
	TaskID       string     `json:"taskId"`
	InstanceID   string     `json:"instanceId"`
	NodeID       string     `json:"nodeId"`
	AssigneeID   string     `json:"assigneeId"`
	Deadline     *time.Time `json:"deadline,omitempty"`
	OccurredTime time.Time  `json:"occurredTime"`
}

func NewTaskCreatedEvent(taskID, instanceID, nodeID, assigneeID string, deadline *time.Time) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		OccurredTime: time.Now(),
	}
}

func (e *TaskCreatedEvent) EventName() string     { return "approval.task.created" }
func (e *TaskCreatedEvent) OccurredAt() time.Time { return e.OccurredTime }

// TaskApprovedEvent fired when a task is approved.
type TaskApprovedEvent struct {
	TaskID       string    `json:"taskId"`
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	OperatorID   string    `json:"operatorId"`
	Opinion      string    `json:"opinion"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskApprovedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskApprovedEvent {
	return &TaskApprovedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      opinion,
		OccurredTime: time.Now(),
	}
}

func (e *TaskApprovedEvent) EventName() string     { return "approval.task.approved" }
func (e *TaskApprovedEvent) OccurredAt() time.Time { return e.OccurredTime }

// TaskRejectedEvent fired when a task is rejected.
type TaskRejectedEvent struct {
	TaskID       string    `json:"taskId"`
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	OperatorID   string    `json:"operatorId"`
	Opinion      string    `json:"opinion"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskRejectedEvent(taskID, instanceID, nodeID, operatorID, opinion string) *TaskRejectedEvent {
	return &TaskRejectedEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		OperatorID:   operatorID,
		Opinion:      opinion,
		OccurredTime: time.Now(),
	}
}

func (e *TaskRejectedEvent) EventName() string     { return "approval.task.rejected" }
func (e *TaskRejectedEvent) OccurredAt() time.Time { return e.OccurredTime }

// TaskTransferredEvent fired when a task is transferred.
type TaskTransferredEvent struct {
	TaskID       string    `json:"taskId"`
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	FromUserID   string    `json:"fromUserId"`
	ToUserID     string    `json:"toUserId"`
	Reason       string    `json:"reason"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskTransferredEvent(taskID, instanceID, nodeID, fromUserID, toUserID, reason string) *TaskTransferredEvent {
	return &TaskTransferredEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		FromUserID:   fromUserID,
		ToUserID:     toUserID,
		Reason:       reason,
		OccurredTime: time.Now(),
	}
}

func (e *TaskTransferredEvent) EventName() string     { return "approval.task.transferred" }
func (e *TaskTransferredEvent) OccurredAt() time.Time { return e.OccurredTime }

// TaskTimeoutEvent fired when a task times out.
type TaskTimeoutEvent struct {
	TaskID       string    `json:"taskId"`
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	AssigneeID   string    `json:"assigneeId"`
	Deadline     time.Time `json:"deadline"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskTimeoutEvent(taskID, instanceID, nodeID, assigneeID string, deadline time.Time) *TaskTimeoutEvent {
	return &TaskTimeoutEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		OccurredTime: time.Now(),
	}
}

func (e *TaskTimeoutEvent) EventName() string     { return "approval.task.timeout" }
func (e *TaskTimeoutEvent) OccurredAt() time.Time { return e.OccurredTime }

// AssigneesAddedEvent fired when assignees are dynamically added.
type AssigneesAddedEvent struct {
	InstanceID   string          `json:"instanceId"`
	NodeID       string          `json:"nodeId"`
	TaskID       string          `json:"taskId"`
	AddType      AddAssigneeType `json:"addType"`
	AssigneeIDs  []string        `json:"assigneeIds"`
	OccurredTime time.Time       `json:"occurredTime"`
}

func NewAssigneesAddedEvent(instanceID, nodeID, taskID string, addType AddAssigneeType, assigneeIDs []string) *AssigneesAddedEvent {
	return &AssigneesAddedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		AddType:      addType,
		AssigneeIDs:  assigneeIDs,
		OccurredTime: time.Now(),
	}
}

func (e *AssigneesAddedEvent) EventName() string     { return "approval.task.assignee_added" }
func (e *AssigneesAddedEvent) OccurredAt() time.Time { return e.OccurredTime }

// AssigneesRemovedEvent fired when assignees are dynamically removed.
type AssigneesRemovedEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	TaskID       string    `json:"taskId"`
	AssigneeIDs  []string  `json:"assigneeIds"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewAssigneesRemovedEvent(instanceID, nodeID, taskID string, assigneeIDs []string) *AssigneesRemovedEvent {
	return &AssigneesRemovedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		AssigneeIDs:  assigneeIDs,
		OccurredTime: time.Now(),
	}
}

func (e *AssigneesRemovedEvent) EventName() string     { return "approval.task.assignee_removed" }
func (e *AssigneesRemovedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== CC Events ====================

// CcNotifiedEvent fired when users are carbon-copied.
type CcNotifiedEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	CcUserIDs    []string  `json:"ccUserIds"`
	IsManual     bool      `json:"isManual"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewCcNotifiedEvent(instanceID, nodeID string, ccUserIDs []string, isManual bool) *CcNotifiedEvent {
	return &CcNotifiedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		CcUserIDs:    ccUserIDs,
		IsManual:     isManual,
		OccurredTime: time.Now(),
	}
}

func (e *CcNotifiedEvent) EventName() string     { return "approval.cc.notified" }
func (e *CcNotifiedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== SubFlow Events ====================

// SubFlowStartedEvent fired when a sub-flow is started.
type SubFlowStartedEvent struct {
	ParentInstanceID string    `json:"parentInstanceId"`
	SubInstanceID    string    `json:"subInstanceId"`
	ParentNodeID     string    `json:"parentNodeId"`
	OccurredTime     time.Time `json:"occurredTime"`
}

func NewSubFlowStartedEvent(parentInstanceID, subInstanceID, parentNodeID string) *SubFlowStartedEvent {
	return &SubFlowStartedEvent{
		ParentInstanceID: parentInstanceID,
		SubInstanceID:    subInstanceID,
		ParentNodeID:     parentNodeID,
		OccurredTime:     time.Now(),
	}
}

func (e *SubFlowStartedEvent) EventName() string     { return "approval.subflow.started" }
func (e *SubFlowStartedEvent) OccurredAt() time.Time { return e.OccurredTime }

// SubFlowCompletedEvent fired when a sub-flow completes.
type SubFlowCompletedEvent struct {
	ParentInstanceID string         `json:"parentInstanceId"`
	SubInstanceID    string         `json:"subInstanceId"`
	ParentNodeID     string         `json:"parentNodeId"`
	FinalStatus      InstanceStatus `json:"finalStatus"`
	OccurredTime     time.Time      `json:"occurredTime"`
}

func NewSubFlowCompletedEvent(parentInstanceID, subInstanceID, parentNodeID string, finalStatus InstanceStatus) *SubFlowCompletedEvent {
	return &SubFlowCompletedEvent{
		ParentInstanceID: parentInstanceID,
		SubInstanceID:    subInstanceID,
		ParentNodeID:     parentNodeID,
		FinalStatus:      finalStatus,
		OccurredTime:     time.Now(),
	}
}

func (e *SubFlowCompletedEvent) EventName() string     { return "approval.subflow.completed" }
func (e *SubFlowCompletedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== Flow Events ====================

// FlowPublishedEvent fired when a flow version is published.
type FlowPublishedEvent struct {
	FlowID       string    `json:"flowId"`
	VersionID    string    `json:"versionId"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewFlowPublishedEvent(flowID, versionID string) *FlowPublishedEvent {
	return &FlowPublishedEvent{
		FlowID:       flowID,
		VersionID:    versionID,
		OccurredTime: time.Now(),
	}
}

func (e *FlowPublishedEvent) EventName() string     { return "approval.flow.published" }
func (e *FlowPublishedEvent) OccurredAt() time.Time { return e.OccurredTime }

// ==================== Timeout & Urge Events ====================

// TaskDeadlineWarningEvent fired when a task is approaching its deadline.
type TaskDeadlineWarningEvent struct {
	TaskID       string    `json:"taskId"`
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	AssigneeID   string    `json:"assigneeId"`
	Deadline     time.Time `json:"deadline"`
	HoursLeft    int       `json:"hoursLeft"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskDeadlineWarningEvent(taskID, instanceID, nodeID, assigneeID string, deadline time.Time, hoursLeft int) *TaskDeadlineWarningEvent {
	return &TaskDeadlineWarningEvent{
		TaskID:       taskID,
		InstanceID:   instanceID,
		NodeID:       nodeID,
		AssigneeID:   assigneeID,
		Deadline:     deadline,
		HoursLeft:    hoursLeft,
		OccurredTime: time.Now(),
	}
}

func (e *TaskDeadlineWarningEvent) EventName() string     { return "approval.task.deadline_warning" }
func (e *TaskDeadlineWarningEvent) OccurredAt() time.Time { return e.OccurredTime }

// TaskUrgedEvent fired when a task assignee is urged/reminded.
type TaskUrgedEvent struct {
	InstanceID   string    `json:"instanceId"`
	NodeID       string    `json:"nodeId"`
	TaskID       string    `json:"taskId"`
	UrgerID      string    `json:"urgerId"`
	TargetUserID string    `json:"targetUserId"`
	Message      string    `json:"message"`
	OccurredTime time.Time `json:"occurredTime"`
}

func NewTaskUrgedEvent(instanceID, nodeID, taskID, urgerID, targetUserID, message string) *TaskUrgedEvent {
	return &TaskUrgedEvent{
		InstanceID:   instanceID,
		NodeID:       nodeID,
		TaskID:       taskID,
		UrgerID:      urgerID,
		TargetUserID: targetUserID,
		Message:      message,
		OccurredTime: time.Now(),
	}
}

func (e *TaskUrgedEvent) EventName() string     { return "approval.task.urged" }
func (e *TaskUrgedEvent) OccurredAt() time.Time { return e.OccurredTime }
