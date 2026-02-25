package service

import (
	"context"

	"github.com/ilxqx/vef-framework-go/approval"
)

// OrganizationService provides org-related operations (implemented by host app).
type OrganizationService = approval.OrganizationService

// UserService provides user-related operations (implemented by host app).
type UserService = approval.UserService

// InstanceNoGenerator generates unique instance numbers for flow instances.
type InstanceNoGenerator interface {
	Generate(ctx context.Context, flowCode string) (string, error)
}
