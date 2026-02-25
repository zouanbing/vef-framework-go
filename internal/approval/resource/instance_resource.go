package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/page"
	"github.com/ilxqx/vef-framework-go/result"
)

// InstanceResource handles instance lifecycle and queries.
type InstanceResource struct {
	api.Resource

	instanceService *service.InstanceService
	queryService    *service.QueryService
}

// NewInstanceResource creates a new instance resource.
func NewInstanceResource(instanceSvc *service.InstanceService, querySvc *service.QueryService) *InstanceResource {
	return &InstanceResource{
		instanceService: instanceSvc,
		queryService:    querySvc,
		Resource: api.NewRPCResource(
			"approval/instance",
			api.WithOperations(
				api.OperationSpec{Action: "start"},
				api.OperationSpec{Action: "process_task"},
				api.OperationSpec{Action: "withdraw"},
				api.OperationSpec{Action: "add_cc"},
				api.OperationSpec{Action: "mark_cc_read"},
				api.OperationSpec{Action: "add_assignee"},
				api.OperationSpec{Action: "remove_assignee"},
				api.OperationSpec{Action: "find_instances"},
				api.OperationSpec{Action: "find_tasks"},
				api.OperationSpec{Action: "get_detail"},
				api.OperationSpec{Action: "get_action_logs"},
				api.OperationSpec{Action: "urge_task"},
			),
		),
	}
}

// StartParams contains the parameters for starting a new instance.
type StartParams struct {
	api.P

	TenantID         string         `json:"tenantId" validate:"required"`
	FlowCode         string         `json:"flowCode" validate:"required"`
	ApplicantID      string         `json:"applicantId" validate:"required"`
	ApplicantDeptID  *string        `json:"applicantDeptId"`
	BusinessRecordID *string        `json:"businessRecordId"`
	FormData         map[string]any `json:"formData"`
}

// Start creates a new flow instance.
func (r *InstanceResource) Start(ctx fiber.Ctx, params StartParams) error {
	instance, err := r.instanceService.StartInstance(ctx.Context(), service.StartInstanceCmd{
		TenantID:         params.TenantID,
		FlowCode:         params.FlowCode,
		ApplicantID:      params.ApplicantID,
		ApplicantDeptID:  params.ApplicantDeptID,
		BusinessRecordID: params.BusinessRecordID,
		FormData:         params.FormData,
	})
	if err != nil {
		return err
	}

	return result.Ok(instance).Response(ctx)
}

// ProcessTaskParams contains the parameters for processing a task.
type ProcessTaskParams struct {
	api.P

	InstanceID   string         `json:"instanceId" validate:"required"`
	TaskID       string         `json:"taskId" validate:"required"`
	Action       string         `json:"action" validate:"required,oneof=approve reject transfer rollback handle"`
	OperatorID   string         `json:"operatorId" validate:"required"`
	Opinion      string         `json:"opinion"`
	FormData     map[string]any `json:"formData"`
	TransferToID string         `json:"transferToId"`
	TargetNodeID string         `json:"targetNodeId"`
}

// ProcessTask handles task actions (approve/reject/transfer/rollback/handle).
func (r *InstanceResource) ProcessTask(ctx fiber.Ctx, params ProcessTaskParams) error {
	if err := r.instanceService.ProcessTask(ctx.Context(), service.ProcessTaskCmd{
		InstanceID:   params.InstanceID,
		TaskID:       params.TaskID,
		Action:       params.Action,
		OperatorID:   params.OperatorID,
		Opinion:      params.Opinion,
		FormData:     params.FormData,
		TransferToID: params.TransferToID,
		TargetNodeID: params.TargetNodeID,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// WithdrawParams contains the parameters for withdrawing an instance.
type WithdrawParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	OperatorID string `json:"operatorId" validate:"required"`
	Reason     string `json:"reason"`
}

// Withdraw withdraws an instance.
func (r *InstanceResource) Withdraw(ctx fiber.Ctx, params WithdrawParams) error {
	if err := r.instanceService.Withdraw(ctx.Context(), params.InstanceID, params.OperatorID, params.Reason); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AddCcParams contains the parameters for adding CC records.
type AddCcParams struct {
	api.P

	InstanceID string   `json:"instanceId" validate:"required"`
	CcUserIDs  []string `json:"ccUserIds" validate:"required,min=1"`
	OperatorID string   `json:"operatorId" validate:"required"`
}

// AddCc adds CC records for an instance.
func (r *InstanceResource) AddCc(ctx fiber.Ctx, params AddCcParams) error {
	if err := r.instanceService.AddCC(ctx.Context(), params.InstanceID, params.CcUserIDs, params.OperatorID); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// MarkCcReadParams contains the parameters for marking CC records as read.
type MarkCcReadParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	UserID     string `json:"userId" validate:"required"`
}

// MarkCcRead marks CC records as read for the user.
func (r *InstanceResource) MarkCcRead(ctx fiber.Ctx, params MarkCcReadParams) error {
	if err := r.instanceService.MarkCCRead(ctx.Context(), params.InstanceID, params.UserID); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AddAssigneeParams contains the parameters for adding assignees.
type AddAssigneeParams struct {
	api.P

	InstanceID string   `json:"instanceId" validate:"required"`
	TaskID     string   `json:"taskId" validate:"required"`
	UserIDs    []string `json:"userIds" validate:"required,min=1,max=50"`
	AddType    string   `json:"addType" validate:"required,oneof=before after parallel"`
	OperatorID string   `json:"operatorId" validate:"required"`
}

// AddAssignee dynamically adds assignees to a task.
func (r *InstanceResource) AddAssignee(ctx fiber.Ctx, params AddAssigneeParams) error {
	if err := r.instanceService.AddAssignee(ctx.Context(), service.AddAssigneeCmd{
		InstanceID: params.InstanceID,
		TaskID:     params.TaskID,
		UserIDs:    params.UserIDs,
		AddType:    params.AddType,
		OperatorID: params.OperatorID,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// RemoveAssigneeParams contains the parameters for removing an assignee.
type RemoveAssigneeParams struct {
	api.P

	TaskID     string `json:"taskId" validate:"required"`
	OperatorID string `json:"operatorId" validate:"required"`
}

// RemoveAssignee removes an assignee by canceling their task.
func (r *InstanceResource) RemoveAssignee(ctx fiber.Ctx, params RemoveAssigneeParams) error {
	if err := r.instanceService.RemoveAssignee(ctx.Context(), params.TaskID, params.OperatorID); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// FindInstancesParams contains the query parameters for finding instances.
type FindInstancesParams struct {
	api.P

	TenantID    string `json:"tenantId"`
	ApplicantID string `json:"applicantId"`
	Status      string `json:"status"`
	FlowID      string `json:"flowId"`
	Keyword     string `json:"keyword"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
}

// FindInstances queries instances with filtering and pagination.
func (r *InstanceResource) FindInstances(ctx fiber.Ctx, params FindInstancesParams) error {
	instances, total, err := r.queryService.FindInstances(ctx.Context(), service.InstanceQuery{
		TenantID:    params.TenantID,
		ApplicantID: params.ApplicantID,
		Status:      params.Status,
		FlowID:      params.FlowID,
		Keyword:     params.Keyword,
		Pageable:    page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(fiber.Map{
		"list":  instances,
		"total": total,
	}).Response(ctx)
}

// FindTasksParams contains the query parameters for finding tasks.
type FindTasksParams struct {
	api.P

	TenantID   string `json:"tenantId"`
	AssigneeID string `json:"assigneeId"`
	InstanceID string `json:"instanceId"`
	Status     string `json:"status"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// FindTasks queries tasks with filtering and pagination.
func (r *InstanceResource) FindTasks(ctx fiber.Ctx, params FindTasksParams) error {
	tasks, total, err := r.queryService.FindTasks(ctx.Context(), service.TaskQuery{
		TenantID:   params.TenantID,
		AssigneeID: params.AssigneeID,
		InstanceID: params.InstanceID,
		Status:     params.Status,
		Pageable:   page.Pageable{Page: params.Page, Size: params.PageSize},
	})
	if err != nil {
		return err
	}

	return result.Ok(fiber.Map{
		"list":  tasks,
		"total": total,
	}).Response(ctx)
}

// GetDetailParams contains the parameters for getting instance detail.
type GetDetailParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// GetDetail returns the full detail of an instance.
func (r *InstanceResource) GetDetail(ctx fiber.Ctx, params GetDetailParams) error {
	detail, err := r.queryService.GetInstanceDetail(ctx.Context(), params.InstanceID)
	if err != nil {
		return err
	}

	return result.Ok(detail).Response(ctx)
}

// GetActionLogsParams contains the parameters for getting action logs.
type GetActionLogsParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// GetActionLogs returns action logs for an instance.
func (r *InstanceResource) GetActionLogs(ctx fiber.Ctx, params GetActionLogsParams) error {
	logs, err := r.queryService.GetActionLogs(ctx.Context(), params.InstanceID)
	if err != nil {
		return err
	}

	return result.Ok(logs).Response(ctx)
}

// UrgeTaskParams contains the parameters for urging a task.
type UrgeTaskParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	TaskID     string `json:"taskId" validate:"required"`
	UrgerID    string `json:"urgerId" validate:"required"`
	Message    string `json:"message"`
}

// UrgeTask sends an urge notification for a pending task.
func (r *InstanceResource) UrgeTask(ctx fiber.Ctx, params UrgeTaskParams) error {
	if err := r.instanceService.UrgeTask(ctx.Context(), service.UrgeTaskCmd{
		InstanceID: params.InstanceID,
		TaskID:     params.TaskID,
		UrgerID:    params.UrgerID,
		Message:    params.Message,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}
