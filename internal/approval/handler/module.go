package handler

import (
	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
)

// Module provides all command/query handlers and registers them with the Bus.
var Module = fx.Module(
	"vef:approval:handler",

	fx.Provide(
		// Behavior
		fx.Annotate(
			NewTransactionBehavior,
			fx.As(new(cqrs.Behavior)),
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),

		// Commands — Flow
		NewCreateFlowHandler,
		NewDeployFlowHandler,
		NewPublishVersionHandler,
		// Commands — Task processing
		NewApproveTaskHandler,
		NewRejectTaskHandler,
		NewTransferTaskHandler,
		NewRollbackTaskHandler,
		// Commands — Instance lifecycle
		NewStartInstanceHandler,
		NewWithdrawHandler,
		NewAddCCHandler,
		NewMarkCCReadHandler,
		NewAddAssigneeHandler,
		NewRemoveAssigneeHandler,
		NewUrgeTaskHandler,
		// Queries
		NewFindInstancesHandler,
		NewFindTasksHandler,
		NewGetInstanceDetailHandler,
		NewGetActionLogsHandler,
		NewGetFlowGraphHandler,
	),

	fx.Invoke(registerHandlers),
)

func registerHandlers(
	bus cqrs.Bus,
	createFlow *CreateFlowHandler,
	deployFlow *DeployFlowHandler,
	publishVersion *PublishVersionHandler,
	approveTask *ApproveTaskHandler,
	rejectTask *RejectTaskHandler,
	transferTask *TransferTaskHandler,
	rollbackTask *RollbackTaskHandler,
	startInstance *StartInstanceHandler,
	withdraw *WithdrawHandler,
	addCC *AddCCHandler,
	markCCRead *MarkCCReadHandler,
	addAssignee *AddAssigneeHandler,
	removeAssignee *RemoveAssigneeHandler,
	urgeTask *UrgeTaskHandler,
	findInstances *FindInstancesHandler,
	findTasks *FindTasksHandler,
	getDetail *GetInstanceDetailHandler,
	getActionLogs *GetActionLogsHandler,
	getFlowGraph *GetFlowGraphHandler,
) {
	// Commands — Flow
	cqrs.Register[CreateFlowCmd, *approval.Flow](bus, createFlow)
	cqrs.Register[DeployFlowCmd, *approval.FlowVersion](bus, deployFlow)
	cqrs.Register[PublishVersionCmd, cqrs.Unit](bus, publishVersion)

	// Commands — Task processing
	cqrs.Register[ApproveTaskCmd, cqrs.Unit](bus, approveTask)
	cqrs.Register[RejectTaskCmd, cqrs.Unit](bus, rejectTask)
	cqrs.Register[TransferTaskCmd, cqrs.Unit](bus, transferTask)
	cqrs.Register[RollbackTaskCmd, cqrs.Unit](bus, rollbackTask)

	// Commands — Instance lifecycle
	cqrs.Register[StartInstanceCmd, *approval.Instance](bus, startInstance)
	cqrs.Register[WithdrawCmd, cqrs.Unit](bus, withdraw)
	cqrs.Register[AddCCCmd, cqrs.Unit](bus, addCC)
	cqrs.Register[MarkCCReadCmd, cqrs.Unit](bus, markCCRead)
	cqrs.Register[AddAssigneeCmd, cqrs.Unit](bus, addAssignee)
	cqrs.Register[RemoveAssigneeCmd, cqrs.Unit](bus, removeAssignee)
	cqrs.Register[UrgeTaskCmd, cqrs.Unit](bus, urgeTask)

	// Queries
	cqrs.Register[FindInstancesQuery, *service.PagedResult[approval.Instance]](bus, findInstances)
	cqrs.Register[FindTasksQuery, *service.PagedResult[approval.Task]](bus, findTasks)
	cqrs.Register[GetInstanceDetailQuery, *service.InstanceDetail](bus, getDetail)
	cqrs.Register[GetActionLogsQuery, []approval.ActionLog](bus, getActionLogs)
	cqrs.Register[GetFlowGraphQuery, *service.FlowGraph](bus, getFlowGraph)
}
