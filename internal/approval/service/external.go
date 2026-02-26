package service

import (
	"context"

	"github.com/ilxqx/vef-framework-go/approval"
)

// AssigneeService provides organization-related operations for resolving assignees (implemented by host app).
type AssigneeService = approval.AssigneeService

// InstanceNoGenerator generates unique instance numbers for flow instances.
type InstanceNoGenerator interface {
	Generate(ctx context.Context, flowCode string) (string, error)
}
