package resource

import (
	"github.com/gofiber/fiber/v3"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/result"
)

// FlowResource handles flow definition management.
type FlowResource struct {
	api.Resource

	flowService *service.FlowService
}

// NewFlowResource creates a new flow resource.
func NewFlowResource(svc *service.FlowService) *FlowResource {
	return &FlowResource{
		flowService: svc,
		Resource: api.NewRPCResource(
			"approval/flow",
			api.WithOperations(
				api.OperationSpec{Action: "deploy"},
				api.OperationSpec{Action: "publish_version"},
				api.OperationSpec{Action: "get_graph"},
			),
		),
	}
}

// DeployParams contains the parameters for deploying a flow.
type DeployParams struct {
	api.P

	TenantID   string `json:"tenantId" validate:"required"`
	FlowCode   string `json:"flowCode" validate:"required"`
	FlowName   string `json:"flowName" validate:"required"`
	CategoryID string `json:"categoryId" validate:"required"`
	Definition string `json:"definition" validate:"required"`
	OperatorID string `json:"operatorId" validate:"required"`
}

// Deploy deploys a flow definition.
func (r *FlowResource) Deploy(ctx fiber.Ctx, params DeployParams) error {
	flow, err := r.flowService.DeployFlow(ctx.Context(), service.DeployFlowCmd{
		TenantID:   params.TenantID,
		FlowCode:   params.FlowCode,
		FlowName:   params.FlowName,
		CategoryID: params.CategoryID,
		Definition: params.Definition,
		OperatorID: params.OperatorID,
	})
	if err != nil {
		return err
	}

	return result.Ok(flow).Response(ctx)
}

// PublishVersionParams contains the parameters for publishing a version.
type PublishVersionParams struct {
	api.P

	VersionID  string `json:"versionId" validate:"required"`
	OperatorID string `json:"operatorId" validate:"required"`
}

// PublishVersion publishes a flow version.
func (r *FlowResource) PublishVersion(ctx fiber.Ctx, params PublishVersionParams) error {
	if err := r.flowService.PublishVersion(ctx.Context(), params.VersionID, params.OperatorID); err != nil {
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
	graph, err := r.flowService.GetFlowGraph(ctx.Context(), params.FlowID)
	if err != nil {
		return err
	}

	return result.Ok(graph).Response(ctx)
}
