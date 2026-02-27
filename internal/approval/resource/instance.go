package resource

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/query"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/page"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/security"
)

// InstanceResource handles instance lifecycle and queries.
type InstanceResource struct {
	api.Resource

	bus cqrs.Bus
}

// NewInstanceResource creates a new instance resource.
func NewInstanceResource(bus cqrs.Bus) *InstanceResource {
	return &InstanceResource{
		bus: bus,
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
	ApplicantDeptID  *string        `json:"applicantDeptId"`
	BusinessRecordID *string        `json:"businessRecordId"`
	FormData         map[string]any `json:"formData"`
}

// Start creates a new flow instance.
func (r *InstanceResource) Start(ctx fiber.Ctx, principal security.Principal, params StartParams) error {
	instance, err := cqrs.Send[command.StartInstanceCmd, *approval.Instance](ctx.Context(), r.bus, command.StartInstanceCmd{
		TenantID:         params.TenantID,
		FlowCode:         params.FlowCode,
		ApplicantID:      principal.ID,
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
	Opinion      string         `json:"opinion"`
	FormData     map[string]any `json:"formData"`
	TransferToID string         `json:"transferToId"`
	TargetNodeID string         `json:"targetNodeId"`
}

// ProcessTask handles task actions (approve/reject/transfer/rollback/handle).
func (r *InstanceResource) ProcessTask(ctx fiber.Ctx, principal security.Principal, params ProcessTaskParams) error {
	var err error

	switch params.Action {
	case "approve", "handle":
		_, err = cqrs.Send[command.ApproveTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.ApproveTaskCmd{
			InstanceID: params.InstanceID,
			TaskID:     params.TaskID,
			OperatorID: principal.ID,
			Opinion:    params.Opinion,
			FormData:   params.FormData,
		})
	case "reject":
		_, err = cqrs.Send[command.RejectTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.RejectTaskCmd{
			InstanceID: params.InstanceID,
			TaskID:     params.TaskID,
			OperatorID: principal.ID,
			Opinion:    params.Opinion,
			FormData:   params.FormData,
		})
	case "transfer":
		_, err = cqrs.Send[command.TransferTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.TransferTaskCmd{
			InstanceID:   params.InstanceID,
			TaskID:       params.TaskID,
			OperatorID:   principal.ID,
			Opinion:      params.Opinion,
			FormData:     params.FormData,
			TransferToID: params.TransferToID,
		})
	case "rollback":
		_, err = cqrs.Send[command.RollbackTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.RollbackTaskCmd{
			InstanceID:   params.InstanceID,
			TaskID:       params.TaskID,
			OperatorID:   principal.ID,
			Opinion:      params.Opinion,
			FormData:     params.FormData,
			TargetNodeID: params.TargetNodeID,
		})
	default:
		return fmt.Errorf("unsupported action: %s", params.Action)
	}

	if err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// WithdrawParams contains the parameters for withdrawing an instance.
type WithdrawParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
	Reason     string `json:"reason"`
}

// Withdraw withdraws an instance.
func (r *InstanceResource) Withdraw(ctx fiber.Ctx, principal security.Principal, params WithdrawParams) error {
	if _, err := cqrs.Send[command.WithdrawCmd, cqrs.Unit](ctx.Context(), r.bus, command.WithdrawCmd{
		InstanceID: params.InstanceID,
		OperatorID: principal.ID,
		Reason:     params.Reason,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// AddCcParams contains the parameters for adding CC records.
type AddCcParams struct {
	api.P

	InstanceID string   `json:"instanceId" validate:"required"`
	CcUserIDs  []string `json:"ccUserIds" validate:"required,min=1"`
}

// AddCc adds CC records for an instance.
func (r *InstanceResource) AddCc(ctx fiber.Ctx, principal security.Principal, params AddCcParams) error {
	if _, err := cqrs.Send[command.AddCCCmd, cqrs.Unit](ctx.Context(), r.bus, command.AddCCCmd{
		InstanceID: params.InstanceID,
		CCUserIDs:  params.CcUserIDs,
		OperatorID: principal.ID,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// MarkCcReadParams contains the parameters for marking CC records as read.
type MarkCcReadParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// MarkCcRead marks CC records as read for the user.
func (r *InstanceResource) MarkCcRead(ctx fiber.Ctx, principal security.Principal, params MarkCcReadParams) error {
	if _, err := cqrs.Send[command.MarkCCReadCmd, cqrs.Unit](ctx.Context(), r.bus, command.MarkCCReadCmd{
		InstanceID: params.InstanceID,
		UserID:     principal.ID,
	}); err != nil {
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
}

// AddAssignee dynamically adds assignees to a task.
func (r *InstanceResource) AddAssignee(ctx fiber.Ctx, principal security.Principal, params AddAssigneeParams) error {
	if _, err := cqrs.Send[command.AddAssigneeCmd, cqrs.Unit](ctx.Context(), r.bus, command.AddAssigneeCmd{
		InstanceID: params.InstanceID,
		TaskID:     params.TaskID,
		UserIDs:    params.UserIDs,
		AddType:    params.AddType,
		OperatorID: principal.ID,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// RemoveAssigneeParams contains the parameters for removing an assignee.
type RemoveAssigneeParams struct {
	api.P

	TaskID string `json:"taskId" validate:"required"`
}

// RemoveAssignee removes an assignee by canceling their task.
func (r *InstanceResource) RemoveAssignee(ctx fiber.Ctx, principal security.Principal, params RemoveAssigneeParams) error {
	if _, err := cqrs.Send[command.RemoveAssigneeCmd, cqrs.Unit](ctx.Context(), r.bus, command.RemoveAssigneeCmd{
		TaskID:     params.TaskID,
		OperatorID: principal.ID,
	}); err != nil {
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
	res, err := cqrs.Send[query.FindInstancesQuery, *service.PagedResult[approval.Instance]](ctx.Context(), r.bus, query.FindInstancesQuery{
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
		"list":  res.List,
		"total": res.Total,
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
	res, err := cqrs.Send[query.FindTasksQuery, *service.PagedResult[approval.Task]](ctx.Context(), r.bus, query.FindTasksQuery{
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
		"list":  res.List,
		"total": res.Total,
	}).Response(ctx)
}

// GetDetailParams contains the parameters for getting instance detail.
type GetDetailParams struct {
	api.P

	InstanceID string `json:"instanceId" validate:"required"`
}

// GetDetail returns the full detail of an instance.
func (r *InstanceResource) GetDetail(ctx fiber.Ctx, params GetDetailParams) error {
	detail, err := cqrs.Send[query.GetInstanceDetailQuery, *service.InstanceDetail](ctx.Context(), r.bus, query.GetInstanceDetailQuery{
		InstanceID: params.InstanceID,
	})
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
	logs, err := cqrs.Send[query.GetActionLogsQuery, []approval.ActionLog](ctx.Context(), r.bus, query.GetActionLogsQuery{
		InstanceID: params.InstanceID,
	})
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
	Message    string `json:"message"`
}

// UrgeTask sends an urge notification for a pending task.
func (r *InstanceResource) UrgeTask(ctx fiber.Ctx, principal security.Principal, params UrgeTaskParams) error {
	if _, err := cqrs.Send[command.UrgeTaskCmd, cqrs.Unit](ctx.Context(), r.bus, command.UrgeTaskCmd{
		InstanceID: params.InstanceID,
		TaskID:     params.TaskID,
		UrgerID:    principal.ID,
		Message:    params.Message,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}
