package query

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// Module provides all query handlers and registers them with the Bus.
var Module = fx.Module(
	"vef:approval:query",

	fx.Provide(
		fx.Private,
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
	findInstances *FindInstancesHandler,
	findTasks *FindTasksHandler,
	getDetail *GetInstanceDetailHandler,
	getActionLogs *GetActionLogsHandler,
	getFlowGraph *GetFlowGraphHandler,
) {
	cqrs.Register(bus, findInstances)
	cqrs.Register(bus, findTasks)
	cqrs.Register(bus, getDetail)
	cqrs.Register(bus, getActionLogs)
	cqrs.Register(bus, getFlowGraph)
}
