package approval

import (
	"context"

	"github.com/ilxqx/vef-framework-go/security"
)

// AssigneeService resolves approval assignees from organizational data (implemented by host app).
type AssigneeService interface {
	GetSuperior(ctx context.Context, userID string) (string, error)
	GetDeptLeaders(ctx context.Context, deptID string) ([]string, error)
	GetRoleUsers(ctx context.Context, roleID string) ([]string, error)
}

// ResolvedAssignee represents a resolved assignee with optional delegation info.
type ResolvedAssignee struct {
	UserID         string
	DelegateFromID *string
}

// PrincipalDeptResolver resolves department info from a security principal.
// Implemented by host apps since Principal.Details is business-specific.
type PrincipalDeptResolver interface {
	Resolve(ctx context.Context, principal *security.Principal) (deptID *string, deptName *string, err error)
}

// AssigneeDefinition represents an assignee configuration in the flow definition.
type AssigneeDefinition struct {
	Kind      AssigneeKind `json:"kind"`
	IDs       []string     `json:"ids,omitempty"`
	FormField *string      `json:"formField,omitempty"`
	SortOrder int          `json:"sortOrder"`
}

// CCDefinition represents a CC recipient in node data.
type CCDefinition struct {
	Kind      CCKind   `json:"kind"`
	IDs       []string `json:"ids,omitempty"`
	FormField *string  `json:"formField,omitempty"`
	Timing    CCTiming `json:"timing,omitempty"`
}
