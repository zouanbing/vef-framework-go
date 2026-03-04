package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// FlowResource handles flow definition management.
type FlowResource struct {
	api.Resource

	bus cqrs.Bus
}

// NewFlowResource creates a new flow resource.
func NewFlowResource(bus cqrs.Bus) api.Resource {
	return &FlowResource{
		bus: bus,
		Resource: api.NewRPCResource(
			"approval/flow",
			api.WithOperations(
				api.OperationSpec{Action: "create"},
				api.OperationSpec{Action: "deploy"},
				api.OperationSpec{Action: "publish_version"},
				api.OperationSpec{Action: "get_graph"},
			),
		),
	}
}

// CreateFlowParams contains the parameters for creating a flow.
type CreateFlowParams struct {
	api.P

	TenantID               string                  `json:"tenantId" validate:"required"`
	Code                   string                  `json:"code" validate:"required"`
	Name                   string                  `json:"name" validate:"required"`
	CategoryID             string                  `json:"categoryId" validate:"required"`
	Icon                   *string                 `json:"icon"`
	Description            *string                 `json:"description"`
	BindingMode            approval.BindingMode    `json:"bindingMode" validate:"required"`
	BusinessTable          *string                 `json:"businessTable"`
	BusinessPkField        *string                 `json:"businessPkField"`
	BusinessTitleField     *string                 `json:"businessTitleField"`
	BusinessStatusField    *string                 `json:"businessStatusField"`
	AdminUserIDs           []string                `json:"adminUserIds"`
	IsAllInitiationAllowed bool                    `json:"isAllInitiationAllowed"`
	InstanceTitleTemplate  string                  `json:"instanceTitleTemplate"`
	Initiators             []CreateInitiatorParams `json:"initiators"`
}

// CreateInitiatorParams contains the parameters for a flow initiator.
type CreateInitiatorParams struct {
	Kind approval.InitiatorKind `json:"kind" validate:"required"`
	IDs  []string               `json:"ids" validate:"required"`
}

// Create creates a new flow.
func (r *FlowResource) Create(ctx fiber.Ctx, params CreateFlowParams) error {
	initiators := make([]shared.CreateFlowInitiatorCmd, len(params.Initiators))
	for i, initiator := range params.Initiators {
		initiators[i] = shared.CreateFlowInitiatorCmd{
			Kind: initiator.Kind,
			IDs:  initiator.IDs,
		}
	}

	flow, err := cqrs.Send[command.CreateFlowCmd, *approval.Flow](
		ctx.Context(),
		r.bus,
		command.CreateFlowCmd{
			TenantID:               params.TenantID,
			Code:                   params.Code,
			Name:                   params.Name,
			CategoryID:             params.CategoryID,
			Icon:                   params.Icon,
			Description:            params.Description,
			BindingMode:            params.BindingMode,
			BusinessTable:          params.BusinessTable,
			BusinessPkField:        params.BusinessPkField,
			BusinessTitleField:     params.BusinessTitleField,
			BusinessStatusField:    params.BusinessStatusField,
			AdminUserIDs:           params.AdminUserIDs,
			IsAllInitiationAllowed: params.IsAllInitiationAllowed,
			InstanceTitleTemplate:  params.InstanceTitleTemplate,
			Initiators:             initiators,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(flow).Response(ctx)
}

// DeployFlowParams contains the parameters for deploying a flow definition.
type DeployFlowParams struct {
	api.P

	FlowID         string                   `json:"flowId" validate:"required"`
	Description    *string                  `json:"description"`
	FlowDefinition approval.FlowDefinition  `json:"flowDefinition" validate:"required"`
	FormDefinition *approval.FormDefinition `json:"formDefinition"`
}

// Deploy deploys a flow definition.
func (r *FlowResource) Deploy(ctx fiber.Ctx, params DeployFlowParams) error {
	version, err := cqrs.Send[command.DeployFlowCmd, *approval.FlowVersion](
		ctx.Context(),
		r.bus,
		command.DeployFlowCmd{
			FlowID:         params.FlowID,
			Description:    params.Description,
			FlowDefinition: params.FlowDefinition,
			FormDefinition: params.FormDefinition,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(version).Response(ctx)
}

// PublishVersionParams contains the parameters for publishing a version.
type PublishVersionParams struct {
	api.P

	VersionID string `json:"versionId" validate:"required"`
}

// PublishVersion publishes a flow version.
func (r *FlowResource) PublishVersion(ctx fiber.Ctx, principal *security.Principal, params PublishVersionParams) error {
	if _, err := cqrs.Send[command.PublishVersionCmd, cqrs.Unit](
		ctx.Context(),
		r.bus,
		command.PublishVersionCmd{
			VersionID:  params.VersionID,
			OperatorID: principal.ID,
		},
	); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// GetGraphParams contains the parameters for getting a flow graph.
type GetGraphParams struct {
	api.P

	FlowID string `json:"flowId" validate:"required"`
}

// GetGraph returns the flow graph for the published version.
func (r *FlowResource) GetGraph(ctx fiber.Ctx, params GetGraphParams) error {
	graph, err := cqrs.Send[query.GetFlowGraphQuery, *shared.FlowGraph](
		ctx.Context(),
		r.bus,
		query.GetFlowGraphQuery{
			FlowID: params.FlowID,
		},
	)
	if err != nil {
		return err
	}

	return result.Ok(graph).Response(ctx)
}
