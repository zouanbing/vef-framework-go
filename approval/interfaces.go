package approval

import "context"

// OrganizationService provides org-related operations (implemented by host app).
type OrganizationService interface {
	GetSuperior(ctx context.Context, userID string) (userID2 string, userName string, err error)
	GetDeptLeaders(ctx context.Context, deptID string) ([]string, error)
}

// UserService provides user-related operations (implemented by host app).
type UserService interface {
	GetUsersByRole(ctx context.Context, roleID string) ([]string, error)
}

// ResolvedAssignee represents a resolved assignee with optional delegation info.
type ResolvedAssignee struct {
	UserID         string
	DelegateFromID string
}

// EvalContext provides context for condition evaluation.
type EvalContext struct {
	FormData    FormData
	ApplicantID string
	DeptID      string
}

// ConditionEvaluator evaluates branch conditions.
type ConditionEvaluator interface {
	Type() ConditionKind
	Evaluate(ctx context.Context, cond Condition, ec *EvalContext) (bool, error)
}

// PassRuleResult indicates the outcome of pass rule evaluation.
type PassRuleResult int

const (
	PassRulePending  PassRuleResult = iota // Still waiting for more actions
	PassRulePassed                         // Node passed
	PassRuleRejected                       // Node rejected
)

// PassRuleContext provides context for pass rule evaluation.
type PassRuleContext struct {
	ApprovedCount int
	RejectedCount int
	TotalCount    int
	PassRatio     float64
}

// PassRuleStrategy evaluates whether a node passes based on task results.
type PassRuleStrategy interface {
	Rule() PassRule
	Evaluate(ctx PassRuleContext) PassRuleResult
}
