package strategy

import "github.com/ilxqx/vef-framework-go/approval"

// NewAllPassStrategy creates a new AllPassStrategy.
func NewAllPassStrategy() approval.PassRuleStrategy {
	return new(AllPassStrategy)
}

// AllPassStrategy requires all assignees to approve.
type AllPassStrategy struct{}

func (s *AllPassStrategy) Rule() approval.PassRule { return approval.PassAll }

func (s *AllPassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.RejectedCount > 0 {
		return approval.PassRuleRejected
	}

	if ctx.ApprovedCount == ctx.TotalCount && ctx.TotalCount > 0 {
		return approval.PassRulePassed
	}

	return approval.PassRulePending
}

// NewOnePassStrategy creates a new OnePassStrategy.
func NewOnePassStrategy() approval.PassRuleStrategy {
	return new(OnePassStrategy)
}

// OnePassStrategy passes when at least one assignee approves.
type OnePassStrategy struct{}

func (s *OnePassStrategy) Rule() approval.PassRule { return approval.PassAny }

func (s *OnePassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.ApprovedCount > 0 {
		return approval.PassRulePassed
	}

	if ctx.RejectedCount == ctx.TotalCount && ctx.TotalCount > 0 {
		return approval.PassRuleRejected
	}

	return approval.PassRulePending
}

// NewRatioPassStrategy creates a new RatioPassStrategy.
func NewRatioPassStrategy() approval.PassRuleStrategy {
	return new(RatioPassStrategy)
}

// RatioPassStrategy passes when approval ratio meets threshold.
type RatioPassStrategy struct{}

func (s *RatioPassStrategy) Rule() approval.PassRule { return approval.PassRatio }

func (s *RatioPassStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.TotalCount == 0 {
		return approval.PassRulePending
	}

	ratio := float64(ctx.ApprovedCount) / float64(ctx.TotalCount) * 100.0
	if ratio >= ctx.PassRatio {
		return approval.PassRulePassed
	}

	maxRatio := float64(ctx.TotalCount-ctx.RejectedCount) / float64(ctx.TotalCount) * 100.0
	if maxRatio < ctx.PassRatio {
		return approval.PassRuleRejected
	}

	return approval.PassRulePending
}

// NewOneRejectStrategy creates a new OneRejectStrategy.
func NewOneRejectStrategy() approval.PassRuleStrategy {
	return new(OneRejectStrategy)
}

// OneRejectStrategy fails when any assignee rejects (veto power).
type OneRejectStrategy struct{}

func (s *OneRejectStrategy) Rule() approval.PassRule { return approval.PassAnyReject }

func (s *OneRejectStrategy) Evaluate(ctx approval.PassRuleContext) approval.PassRuleResult {
	if ctx.RejectedCount > 0 {
		return approval.PassRuleRejected
	}

	if ctx.ApprovedCount == ctx.TotalCount && ctx.TotalCount > 0 {
		return approval.PassRulePassed
	}

	return approval.PassRulePending
}
