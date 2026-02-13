package approval

import (
	"github.com/ilxqx/vef-framework-go/decimal"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
)

// ── Flow Definition ─────────────────────────────────────────────────────────

// Flow represents a flow definition.
type Flow struct {
	orm.BaseModel `bun:"table:apv_flow,alias:af"`
	orm.Model

	// TenantID is the tenant identifier for multi-tenancy support.
	TenantID string `json:"tenantId" bun:"tenant_id"`
	// CategoryID is the ID of the category this flow belongs to.
	CategoryID string `json:"categoryId" bun:"category_id"`
	// Code is the unique identifier code for the flow.
	Code string `json:"code" bun:"code"`
	// Name is the display name of the flow.
	Name string `json:"name" bun:"name"`
	// Icon is the optional icon for the flow.
	Icon null.String `json:"icon" bun:"icon,nullzero"`
	// Description is an optional description of the flow.
	Description null.String `json:"description" bun:"description,nullzero"`
	// BindingMode specifies the data binding mode (standalone or bound).
	BindingMode BindingMode `json:"bindingMode" bun:"binding_mode"`
	// BusinessTable is the bound business table name when in bound mode.
	BusinessTable null.String `json:"businessTable" bun:"business_table,nullzero"`
	// BusinessPkField is the primary key field of the business table.
	BusinessPkField null.String `json:"businessPkField" bun:"business_pk_field,nullzero"`
	// BusinessTitleField is the title field mapping of the business table.
	BusinessTitleField null.String `json:"businessTitleField" bun:"business_title_field,nullzero"`
	// BusinessStatusField is the status field mapping of the business table.
	BusinessStatusField null.String `json:"businessStatusField" bun:"business_status_field,nullzero"`
	// AdminUserIDs is the list of flow administrator user IDs.
	AdminUserIDs []string `json:"adminUserIds" bun:"admin_user_ids,type:jsonb,nullzero"`
	// IsAllInitiateAllowed indicates whether all users can initiate this flow.
	IsAllInitiateAllowed bool `json:"isAllInitiateAllowed" bun:"is_all_initiate_allowed"`
	// InstanceTitleTemplate is the Go text/template for generating instance titles.
	InstanceTitleTemplate string `json:"instanceTitleTemplate" bun:"instance_title_template"`
	// IsActive indicates whether the flow is enabled.
	IsActive bool `json:"isActive" bun:"is_active"`
	// CurrentVersion is the current published version number.
	CurrentVersion int `json:"currentVersion" bun:"current_version"`
}

// FlowCategory represents a category for grouping flows.
type FlowCategory struct {
	orm.BaseModel `bun:"table:apv_flow_category,alias:afc"`
	orm.Model

	// Code is the unique identifier code for the category.
	Code string `json:"code" bun:"code"`
	// Name is the display name of the category.
	Name string `json:"name" bun:"name"`
	// Icon is the optional icon for the category.
	Icon null.String `json:"icon" bun:"icon,nullzero"`
	// ParentID is the ID of the parent category for hierarchical structure.
	ParentID null.String `json:"parentId" bun:"parent_id,nullzero"`
	// SortOrder determines the display order of categories.
	SortOrder int `json:"sortOrder" bun:"sort_order"`
	// IsActive indicates whether the category is enabled.
	IsActive bool `json:"isActive" bun:"is_active"`
	// Remark is an optional description or note for the category.
	Remark null.String `json:"remark" bun:"remark,nullzero"`
}

// FlowVersion represents a flow version.
type FlowVersion struct {
	orm.BaseModel `bun:"table:apv_flow_version,alias:afv"`
	orm.Model

	// FlowID is the ID of the flow this version belongs to.
	FlowID string `json:"flowId" bun:"flow_id"`
	// Version is the version number.
	Version int `json:"version" bun:"version"`
	// Status is the version status (draft, published, archived).
	Status VersionStatus `json:"status" bun:"status"`
	// StorageMode specifies the form data storage mode.
	StorageMode StorageMode `json:"storageMode" bun:"storage_mode"`
	// FlowSchema contains the flow structure definition (React Flow compatible).
	FlowSchema *FlowDefinition `json:"flowSchema" bun:"flow_schema,type:jsonb,nullzero"`
	// FormSchema contains the form structure definition.
	FormSchema *FormDefinition `json:"formSchema" bun:"form_schema,type:jsonb,nullzero"`
	// PublishedAt is the timestamp when this version was published.
	PublishedAt null.DateTime `json:"publishedAt" bun:"published_at,nullzero"`
	// PublishedBy is the user ID who published this version.
	PublishedBy null.String `json:"publishedBy" bun:"published_by,nullzero"`
}

// FlowNode represents a flow node.
type FlowNode struct {
	orm.BaseModel `bun:"table:apv_flow_node,alias:afn"`
	orm.Model

	FlowVersionID        string                 `json:"flowVersionId" bun:"flow_version_id"`
	NodeKey              string                 `json:"nodeKey" bun:"node_key"`
	NodeKind             NodeKind               `json:"nodeKind" bun:"node_kind"`
	Name                 string                 `json:"name" bun:"name"`
	Description          null.String            `json:"description" bun:"description,nullzero"`
	ExecutionType        ExecutionType          `json:"executionType" bun:"execution_type"`
	ApprovalMethod       ApprovalMethod         `json:"approvalMethod" bun:"approval_method"`
	PassRule             PassRule               `json:"passRule" bun:"pass_rule"`
	PassRatio            decimal.Decimal        `json:"passRatio" bun:"pass_ratio"`
	EmptyHandlerAction   EmptyHandlerAction     `json:"emptyHandlerAction" bun:"empty_handler_action"`
	FallbackUserIDs      []string               `json:"fallbackUserIds" bun:"fallback_user_ids,type:jsonb,nullzero"`
	AdminUserIDs         []string               `json:"adminUserIds" bun:"admin_user_ids,type:jsonb,nullzero"`
	SameApplicantAction  SameApplicantAction    `json:"sameApplicantAction" bun:"same_applicant_action"`
	IsRollbackAllowed    bool                   `json:"isRollbackAllowed" bun:"is_rollback_allowed"`
	RollbackType         RollbackType           `json:"rollbackType" bun:"rollback_type"`
	RollbackDataStrategy RollbackDataStrategy   `json:"rollbackDataStrategy" bun:"rollback_data_strategy"`
	IsAddAssigneeAllowed bool                   `json:"isAddAssigneeAllowed" bun:"is_add_assignee_allowed"`
	AddAssigneeTypes     []string               `json:"addAssigneeTypes" bun:"add_assignee_types,type:jsonb,nullzero"`
	IsRemoveAssigneeAllowed bool               `json:"isRemoveAssigneeAllowed" bun:"is_remove_assignee_allowed"`
	FieldPermissions     map[string]any         `json:"fieldPermissions" bun:"field_permissions,type:jsonb,nullzero"`
	IsManualCCAllowed    bool                   `json:"isManualCcAllowed" bun:"is_manual_cc_allowed"`
	IsTransferAllowed    bool                   `json:"isTransferAllowed" bun:"is_transfer_allowed"`
	IsOpinionRequired    bool                   `json:"isOpinionRequired" bun:"is_opinion_required"`
	TimeoutHours         int                    `json:"timeoutHours" bun:"timeout_hours"`
	DuplicateHandlerAction DuplicateHandlerAction `json:"duplicateHandlerAction" bun:"duplicate_handler_action"`
	SubFlowConfig        map[string]any         `json:"subFlowConfig" bun:"sub_flow_config,type:jsonb,nullzero"`
	Branches             []ConditionBranch      `json:"branches" bun:"branches,type:jsonb,nullzero"`
}

// FlowEdge represents a directed edge between two flow nodes.
type FlowEdge struct {
	orm.BaseModel `bun:"table:apv_flow_edge,alias:afe"`
	orm.IDModel

	FlowVersionID string      `json:"flowVersionId" bun:"flow_version_id"`
	SourceNodeID  string      `json:"sourceNodeId" bun:"source_node_id"`
	TargetNodeID  string      `json:"targetNodeId" bun:"target_node_id"`
	SourceHandle  null.String `json:"sourceHandle" bun:"source_handle,nullzero"`
}

// FlowNodeAssignee represents a node assignee configuration.
type FlowNodeAssignee struct {
	orm.BaseModel `bun:"table:apv_flow_node_assignee,alias:afna"`
	orm.IDModel

	NodeID       string       `json:"nodeId" bun:"node_id"`
	AssigneeKind AssigneeKind `json:"assigneeKind" bun:"assignee_kind"`
	AssigneeIDs  []string     `json:"assigneeIds" bun:"assignee_ids,type:jsonb,nullzero"`
	FormField    null.String  `json:"formField" bun:"form_field,nullzero"`
	SortOrder    int          `json:"sortOrder" bun:"sort_order"`
}

// FlowNodeCC represents a node CC configuration.
type FlowNodeCC struct {
	orm.BaseModel `bun:"table:apv_flow_node_cc,alias:afnc"`
	orm.IDModel

	NodeID    string      `json:"nodeId" bun:"node_id"`
	CCType    string      `json:"ccType" bun:"cc_type"`
	CCIDs     []string    `json:"ccIds" bun:"cc_ids,type:jsonb,nullzero"`
	FormField null.String `json:"formField" bun:"form_field,nullzero"`
}

// FlowFormField represents a flow form field definition.
type FlowFormField struct {
	orm.BaseModel `bun:"table:apv_flow_form_field,alias:afff"`
	orm.IDModel

	FlowVersionID string         `json:"flowVersionId" bun:"flow_version_id"`
	Name          string         `json:"name" bun:"name"`
	Kind          string         `json:"kind" bun:"kind"`
	Label         string         `json:"label" bun:"label"`
	Placeholder   null.String    `json:"placeholder" bun:"placeholder,nullzero"`
	DefaultValue  null.String    `json:"defaultValue" bun:"default_value,nullzero"`
	IsRequired    null.Bool      `json:"isRequired" bun:"is_required,nullzero"`
	IsReadonly    null.Bool      `json:"isReadonly" bun:"is_readonly,nullzero"`
	Validation    map[string]any `json:"validation" bun:"validation,type:jsonb,nullzero"`
	SortOrder     int            `json:"sortOrder" bun:"sort_order"`
	Meta          map[string]any `json:"meta" bun:"meta,type:jsonb,nullzero"`
}

// FlowInitiator represents a flow initiator configuration.
type FlowInitiator struct {
	orm.BaseModel `bun:"table:apv_flow_initiator,alias:afi"`
	orm.IDModel

	FlowID        string        `json:"flowId" bun:"flow_id"`
	InitiatorKind InitiatorKind `json:"initiatorKind" bun:"initiator_kind"`
	InitiatorIDs  []string      `json:"initiatorIds" bun:"initiator_ids,type:jsonb,nullzero"`
}

// ── Instance & Task ─────────────────────────────────────────────────────────

// Instance represents a flow instance.
type Instance struct {
	orm.BaseModel `bun:"table:apv_instance,alias:ai"`
	orm.Model

	FlowID           string      `json:"flowId" bun:"flow_id"`
	FlowVersionID    string      `json:"flowVersionId" bun:"flow_version_id"`
	ParentInstanceID null.String `json:"parentInstanceId" bun:"parent_instance_id,nullzero"`
	ParentNodeID     null.String `json:"parentNodeId" bun:"parent_node_id,nullzero"`
	Title            string      `json:"title" bun:"title"`
	SerialNo         string      `json:"serialNo" bun:"serial_no"`
	ApplicantID      string      `json:"applicantId" bun:"applicant_id"`
	ApplicantDeptID  null.String `json:"applicantDeptId" bun:"applicant_dept_id,nullzero"`
	Status           string      `json:"status" bun:"status"`
	CurrentNodeID    null.String `json:"currentNodeId" bun:"current_node_id,nullzero"`
	FinishedAt       null.DateTime `json:"finishedAt" bun:"finished_at,nullzero"`
	BusinessRecordID null.String `json:"businessRecordId" bun:"business_record_id,nullzero"`
	FormData         map[string]any `json:"formData" bun:"form_data,type:jsonb,nullzero"`
}

// Task represents an approval task.
type Task struct {
	orm.BaseModel `bun:"table:apv_task,alias:at"`
	orm.Model

	InstanceID      string        `json:"instanceId" bun:"instance_id"`
	NodeID          string        `json:"nodeId" bun:"node_id"`
	AssigneeID      string        `json:"assigneeId" bun:"assignee_id"`
	DelegateFromID  null.String   `json:"delegateFromId" bun:"delegate_from_id,nullzero"`
	SortOrder       int           `json:"sortOrder" bun:"sort_order"`
	Status          string        `json:"status" bun:"status"`
	ReadAt          null.DateTime `json:"readAt" bun:"read_at,nullzero"`
	ParentTaskID    null.String   `json:"parentTaskId" bun:"parent_task_id,nullzero"`
	AddAssigneeType null.String   `json:"addAssigneeType" bun:"add_assignee_type,nullzero"`
	Deadline        null.DateTime `json:"deadline" bun:"deadline,nullzero"`
	IsTimeout       bool          `json:"isTimeout" bun:"is_timeout"`
	FinishedAt      null.DateTime `json:"finishedAt" bun:"finished_at,nullzero"`
}

// FormSnapshot represents a form snapshot for rollback strategies.
type FormSnapshot struct {
	orm.BaseModel `bun:"table:apv_form_snapshot,alias:afs"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string         `json:"instanceId" bun:"instance_id"`
	NodeID     string         `json:"nodeId" bun:"node_id"`
	FormData   map[string]any `json:"formData" bun:"form_data,type:jsonb"`
}

// ── Records & Logs ──────────────────────────────────────────────────────────

// ActionLog represents an action log.
type ActionLog struct {
	orm.BaseModel `bun:"table:apv_action_log,alias:aal"`
	orm.IDModel
	orm.CreatedModel

	InstanceID        string         `json:"instanceId" bun:"instance_id"`
	NodeID            null.String    `json:"nodeId" bun:"node_id,nullzero"`
	TaskID            null.String    `json:"taskId" bun:"task_id,nullzero"`
	Action            string         `json:"action" bun:"action"`
	OperatorID        string         `json:"operatorId" bun:"operator_id"`
	OperatorName      null.String    `json:"operatorName" bun:"operator_name,nullzero"`
	OperatorDept      null.String    `json:"operatorDept" bun:"operator_dept,nullzero"`
	IPAddress         null.String    `json:"ipAddress" bun:"ip_address,nullzero"`
	UserAgent         null.String    `json:"userAgent" bun:"user_agent,nullzero"`
	Opinion           null.String    `json:"opinion" bun:"opinion,nullzero"`
	Meta              map[string]any `json:"meta" bun:"meta,type:jsonb,nullzero"`
	TransferToID      null.String    `json:"transferToId" bun:"transfer_to_id,nullzero"`
	RollbackToNodeID  null.String    `json:"rollbackToNodeId" bun:"rollback_to_node_id,nullzero"`
	AddAssigneeType   null.String    `json:"addAssigneeType" bun:"add_assignee_type,nullzero"`
	AddAssigneeToIDs  []string       `json:"addAssigneeToIds" bun:"add_assignee_to_ids,type:jsonb"`
	RemoveAssigneeIDs []string       `json:"removeAssigneeIds" bun:"remove_assignee_ids,type:jsonb"`
	CCUserIDs         []string       `json:"ccUserIds" bun:"cc_user_ids,type:jsonb"`
	Attachments       []any          `json:"attachments" bun:"attachments,type:jsonb"`
}

// CCRecord represents a CC record.
type CCRecord struct {
	orm.BaseModel `bun:"table:apv_cc_record,alias:acr"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string      `json:"instanceId" bun:"instance_id"`
	NodeID     null.String `json:"nodeId" bun:"node_id,nullzero"`
	TaskID     null.String `json:"taskId" bun:"task_id,nullzero"`
	CCUserID   string      `json:"ccUserId" bun:"cc_user_id"`
	IsManual   bool        `json:"isManual" bun:"is_manual"`
	ReadAt     null.Time   `json:"readAt" bun:"read_at,nullzero"`
}

// ParallelRecord represents a parallel approval record.
type ParallelRecord struct {
	orm.BaseModel `bun:"table:apv_parallel_record,alias:apr"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string      `json:"instanceId" bun:"instance_id"`
	NodeID     string      `json:"nodeId" bun:"node_id"`
	TaskID     string      `json:"taskId" bun:"task_id"`
	AssigneeID string      `json:"assigneeId" bun:"assignee_id"`
	Result     null.String `json:"result" bun:"result,nullzero"`
	Opinion    null.String `json:"opinion" bun:"opinion,nullzero"`
}

// Delegation represents an approval delegation.
type Delegation struct {
	orm.BaseModel `bun:"table:apv_delegation,alias:ad"`
	orm.Model

	DelegatorID    string      `json:"delegatorId" bun:"delegator_id"`
	DelegateeID    string      `json:"delegateeId" bun:"delegatee_id"`
	FlowCategoryID null.String `json:"flowCategoryId" bun:"flow_category_id,nullzero"`
	FlowID         null.String `json:"flowId" bun:"flow_id,nullzero"`
	StartTime      null.Time   `json:"startTime" bun:"start_time"`
	EndTime        null.Time   `json:"endTime" bun:"end_time"`
	IsActive       bool        `json:"isActive" bun:"is_active"`
	Reason         null.String `json:"reason" bun:"reason,nullzero"`
}

// EventOutbox represents an event outbox for transactional event publishing.
type EventOutbox struct {
	orm.BaseModel `bun:"table:apv_event_outbox,alias:aeo"`
	orm.Model

	EventID     string         `json:"eventId" bun:"event_id"`
	EventType   string         `json:"eventType" bun:"event_type"`
	Payload     map[string]any `json:"payload" bun:"payload,type:jsonb"`
	Status      string         `json:"status" bun:"status"`
	RetryCount  int            `json:"retryCount" bun:"retry_count"`
	LastError   null.String    `json:"lastError" bun:"last_error,nullzero"`
	ProcessedAt null.Time      `json:"processedAt" bun:"processed_at,nullzero"`
}
