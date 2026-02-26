package approval

import (
	"context"

	"github.com/ilxqx/vef-framework-go/timex"
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

// EvaluationContext provides context for condition evaluation.
type EvaluationContext struct {
	FormData        FormData
	ApplicantID     string
	ApplicantDeptID string
}

// ConditionEvaluator evaluates branch conditions.
type ConditionEvaluator interface {
	Kind() ConditionKind
	Evaluate(ctx context.Context, cond Condition, ec *EvaluationContext) (bool, error)
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

// DomainEvent is the base interface for all approval domain events.
type DomainEvent interface {
	EventName() string
	OccurredAt() timex.DateTime
}

// EventDispatcher dispatches outbox events to external systems.
// Default implementation forwards to event.Bus
type EventDispatcher interface {
	Dispatch(ctx context.Context, record EventOutbox) error
}
