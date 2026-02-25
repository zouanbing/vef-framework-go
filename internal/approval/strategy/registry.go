package strategy

import (
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
)

// StrategyRegistry holds all strategy implementations indexed by their type.
type StrategyRegistry struct {
	passRules  map[approval.PassRule]approval.PassRuleStrategy
	assignees  map[approval.AssigneeKind]AssigneeResolver
	conditions map[approval.ConditionKind]approval.ConditionEvaluator
	composite  *CompositeAssigneeResolver
}

// NewStrategyRegistry creates a registry from slices (designed for FX group injection).
func NewStrategyRegistry(
	passRules []approval.PassRuleStrategy,
	assignees []AssigneeResolver,
	conditions []approval.ConditionEvaluator,
) *StrategyRegistry {
	r := &StrategyRegistry{
		passRules:  make(map[approval.PassRule]approval.PassRuleStrategy, len(passRules)),
		assignees:  make(map[approval.AssigneeKind]AssigneeResolver, len(assignees)),
		conditions: make(map[approval.ConditionKind]approval.ConditionEvaluator, len(conditions)),
	}

	for _, rule := range passRules {
		r.passRules[rule.Rule()] = rule
	}

	for _, assignee := range assignees {
		r.assignees[assignee.Kind()] = assignee
	}

	for _, condition := range conditions {
		r.conditions[condition.Kind()] = condition
	}

	r.composite = NewCompositeAssigneeResolver(assignees...)

	return r
}

// GetPassRuleStrategy returns the pass rule strategy for the given rule.
func (r *StrategyRegistry) GetPassRuleStrategy(rule approval.PassRule) (approval.PassRuleStrategy, error) {
	s, ok := r.passRules[rule]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPassRuleNotFound, rule)
	}

	return s, nil
}

// GetConditionEvaluator returns the condition evaluator for the given type.
func (r *StrategyRegistry) GetConditionEvaluator(t approval.ConditionKind) (approval.ConditionEvaluator, error) {
	s, ok := r.conditions[t]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrConditionEvaluatorNotFound, t)
	}

	return s, nil
}

// CompositeAssigneeResolver returns the cached CompositeAssigneeResolver.
func (r *StrategyRegistry) CompositeAssigneeResolver() *CompositeAssigneeResolver {
	return r.composite
}
