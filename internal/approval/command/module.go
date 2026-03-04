package command

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// Module provides all command handlers and registers them with the Bus.
var Module = fx.Module(
	"vef:approval:command",

	fx.Provide(
		fx.Private,
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
		NewResubmitHandler,
		NewAddCCHandler,
		NewMarkCCReadHandler,
		NewAddAssigneeHandler,
		NewRemoveAssigneeHandler,
		NewUrgeTaskHandler,
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
	resubmit *ResubmitHandler,
	addCC *AddCCHandler,
	markCCRead *MarkCCReadHandler,
	addAssignee *AddAssigneeHandler,
	removeAssignee *RemoveAssigneeHandler,
	urgeTask *UrgeTaskHandler,
) {
	// Commands — Flow
	cqrs.Register(bus, createFlow)
	cqrs.Register(bus, deployFlow)
	cqrs.Register(bus, publishVersion)

	// Commands — Task processing
	cqrs.Register(bus, approveTask)
	cqrs.Register(bus, rejectTask)
	cqrs.Register(bus, transferTask)
	cqrs.Register(bus, rollbackTask)

	// Commands — Instance lifecycle
	cqrs.Register(bus, startInstance)
	cqrs.Register(bus, withdraw)
	cqrs.Register(bus, resubmit)
	cqrs.Register(bus, addCC)
	cqrs.Register(bus, markCCRead)
	cqrs.Register(bus, addAssignee)
	cqrs.Register(bus, removeAssignee)
	cqrs.Register(bus, urgeTask)
}
