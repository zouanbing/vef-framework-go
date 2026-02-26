package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/decimal"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
)

// TestNormalizePassRatio tests normalize pass ratio scenarios.
func TestNormalizePassRatio(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"Negative", -1.0, 0},
		{"Zero", 0, 0},
		{"ProportionHalf", 0.5, 50},
		{"ProportionSixtyPercent", 0.6, 60},
		{"ProportionOne", 1.0, 100},
		{"PercentageFifty", 50, 50},
		{"PercentageHundred", 100, 100},
		{"AboveHundred", 150, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.expected, NormalizePassRatio(tt.input), 0.001, "NormalizePassRatio(%v) should produce expected result", tt.input)
		})
	}
}

// TestNewFlowEngine tests new flow engine scenarios.
func TestNewFlowEngine(t *testing.T) {
	t.Run("RegistersProcessors", func(t *testing.T) {
		processors := []NodeProcessor{
			NewStartProcessor(),
			NewEndProcessor(),
			NewConditionProcessor(),
		}
		engine := NewFlowEngine(nil, processors, nil)

		assert.Len(t, engine.processors, 3, "Should register all 3 processor types")
		assert.IsType(t, &StartProcessor{}, engine.processors[approval.NodeStart], "Should register StartProcessor")
		assert.IsType(t, &EndProcessor{}, engine.processors[approval.NodeEnd], "Should register EndProcessor")
		assert.IsType(t, &ConditionProcessor{}, engine.processors[approval.NodeCondition], "Should register ConditionProcessor")
	})

	t.Run("EmptyProcessors", func(t *testing.T) {
		engine := NewFlowEngine(nil, nil, nil)
		assert.Empty(t, engine.processors, "Should have no processors when none provided")
	})
}

// TestBuildPassRuleContext tests build pass rule context scenarios.
func TestBuildPassRuleContext(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []approval.Task
		wantTotal     int
		wantApproved  int
		wantRejected  int
	}{
		{
			name: "AllPending",
			tasks: []approval.Task{
				{Status: approval.TaskPending},
				{Status: approval.TaskPending},
				{Status: approval.TaskPending},
			},
			wantTotal: 3, wantApproved: 0, wantRejected: 0,
		},
		{
			name: "MixedStatuses",
			tasks: []approval.Task{
				{Status: approval.TaskApproved},
				{Status: approval.TaskRejected},
				{Status: approval.TaskPending},
				{Status: approval.TaskApproved},
			},
			wantTotal: 4, wantApproved: 2, wantRejected: 1,
		},
		{
			name: "ExcludesNonActionable",
			tasks: []approval.Task{
				{Status: approval.TaskApproved},
				{Status: approval.TaskTransferred},
				{Status: approval.TaskCanceled},
				{Status: approval.TaskRemoved},
				{Status: approval.TaskSkipped},
				{Status: approval.TaskPending},
			},
			wantTotal: 2, wantApproved: 1, wantRejected: 0,
		},
		{
			name: "HandledCountsAsApproved",
			tasks: []approval.Task{
				{Status: approval.TaskHandled},
				{Status: approval.TaskApproved},
			},
			wantTotal: 2, wantApproved: 2, wantRejected: 0,
		},
		{
			name:         "EmptyTasks",
			wantTotal:    0,
			wantApproved: 0,
			wantRejected: 0,
		},
	}

	node := &approval.FlowNode{PassRatio: decimal.NewFromInt(0)}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := buildPassRuleContext(node, tt.tasks)
			assert.Equal(t, tt.wantTotal, ctx.TotalCount, "TotalCount should match")
			assert.Equal(t, tt.wantApproved, ctx.ApprovedCount, "ApprovedCount should match")
			assert.Equal(t, tt.wantRejected, ctx.RejectedCount, "RejectedCount should match")
		})
	}
}

// TestEvaluatePassRuleWithTasks tests evaluate pass rule with tasks scenarios.
func TestEvaluatePassRuleWithTasks(t *testing.T) {
	registry := strategy.NewStrategyRegistry(
		[]approval.PassRuleStrategy{strategy.NewAllPassStrategy()},
		nil,
		nil,
	)
	engine := NewFlowEngine(registry, nil, nil)
	node := &approval.FlowNode{PassRule: approval.PassAll, PassRatio: decimal.NewFromInt(0)}

	t.Run("AllApproved", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskApproved},
		}
		result, err := engine.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePassed, result, "Should pass when all tasks approved")
	})

	t.Run("HasPending", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskPending},
		}
		result, err := engine.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePending, result, "Should be pending when one task is pending")
	})

	t.Run("HasRejected", func(t *testing.T) {
		tasks := []approval.Task{
			{Status: approval.TaskApproved},
			{Status: approval.TaskRejected},
		}
		result, err := engine.EvaluatePassRuleWithTasks(node, tasks)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRuleRejected, result, "Should reject when one task is rejected")
	})

	t.Run("UnknownRule", func(t *testing.T) {
		unknownNode := &approval.FlowNode{PassRule: "nonexistent", PassRatio: decimal.NewFromInt(0)}
		_, err := engine.EvaluatePassRuleWithTasks(unknownNode, nil)
		assert.Error(t, err, "Should return error for unknown pass rule")
	})
}

// TestEvaluateBranchConditions tests evaluate branch conditions scenarios.
func TestEvaluateBranchConditions(t *testing.T) {
	registry := strategy.NewStrategyRegistry(
		nil,
		nil,
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
			strategy.NewExpressionConditionEvaluator(),
		},
	)

	t.Run("SingleFieldConditionMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "amount", Operator: "gt", Value: float64(500)},
		}
		formData := approval.NewFormData(map[string]any{"amount": float64(1000)})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for valid field condition")
		assert.True(t, match, "Should match when amount 1000 > 500")
	})

	t.Run("SingleFieldConditionNoMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "amount", Operator: "gt", Value: float64(2000)},
		}
		formData := approval.NewFormData(map[string]any{"amount": float64(1000)})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for valid field condition")
		assert.False(t, match, "Should not match when amount 1000 is not > 2000")
	})

	t.Run("AllConditionsMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "amount", Operator: "gte", Value: float64(100)},
			{Type: approval.ConditionField, Subject: "category", Operator: "eq", Value: "travel"},
		}
		formData := approval.NewFormData(map[string]any{
			"amount":   float64(500),
			"category": "travel",
		})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error when all conditions are valid")
		assert.True(t, match, "Should match when all AND conditions are satisfied")
	})

	t.Run("OneConditionFails", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "amount", Operator: "gte", Value: float64(100)},
			{Type: approval.ConditionField, Subject: "category", Operator: "eq", Value: "travel"},
		}
		formData := approval.NewFormData(map[string]any{
			"amount":   float64(500),
			"category": "purchase",
		})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for valid conditions")
		assert.False(t, match, "Should not match when one AND condition fails")
	})

	t.Run("UnknownConditionType", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionKind("unknown_type"), Subject: "x", Operator: "eq", Value: "y"},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		_, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		assert.Error(t, err, "Should return error for unknown condition type")
	})

	t.Run("ExpressionConditionMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `formData["amount"] > 500`},
		}
		formData := approval.NewFormData(map[string]any{"amount": float64(1000)})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for valid expression")
		assert.True(t, match, "Should match when expression evaluates to true")
	})

	t.Run("ExpressionConditionNoMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `formData["amount"] > 5000`},
		}
		formData := approval.NewFormData(map[string]any{"amount": float64(100)})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for valid expression")
		assert.False(t, match, "Should not match when expression evaluates to false")
	})

	t.Run("ExpressionWithApplicantContext", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `applicantId == "admin_user"`},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "admin_user"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for applicant expression")
		assert.True(t, match, "Should match when applicant equals expected value")
	})

	t.Run("ExpressionCompileError", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `formData[`},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		_, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		assert.Error(t, err, "Should return error for invalid expression syntax")
	})

	t.Run("EmptyConditions", func(t *testing.T) {
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, nil)
		require.NoError(t, err, "Should not return error for empty conditions")
		assert.True(t, match, "Should match when there are no conditions (vacuous truth)")
	})

	t.Run("FieldConditionWithApplicantSubject", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "applicantId", Operator: "eq", Value: "user_42"},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "user_42"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for applicant field condition")
		assert.True(t, match, "Should match when applicant ID equals expected value")
	})

	t.Run("MixedFieldAndExpressionAllMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "status", Operator: "eq", Value: "active"},
			{Type: approval.ConditionExpression, Expression: `formData["priority"] == "high"`},
		}
		formData := approval.NewFormData(map[string]any{
			"status":   "active",
			"priority": "high",
		})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for mixed conditions")
		assert.True(t, match, "Should match when all mixed conditions are satisfied")
	})

	t.Run("MixedFieldAndExpressionOneFails", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "status", Operator: "eq", Value: "active"},
			{Type: approval.ConditionExpression, Expression: `formData["priority"] == "low"`},
		}
		formData := approval.NewFormData(map[string]any{
			"status":   "active",
			"priority": "high",
		})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error when expression simply does not match")
		assert.False(t, match, "Should not match when expression condition fails in AND group")
	})
}


// TestPublishEventsNilPublisher tests publish events nil publisher scenarios.
func TestPublishEventsNilPublisher(t *testing.T) {
	engine := NewFlowEngine(nil, nil, nil)

	t.Run("NilPublisherNoOp", func(t *testing.T) {
		err := engine.publishEvents(t.Context(), nil)
		assert.NoError(t, err, "Should return nil when publisher is nil")
	})

	t.Run("NilPublisherWithEvents", func(t *testing.T) {
		err := engine.publishEvents(t.Context(), nil, nil)
		assert.NoError(t, err, "Should return nil when publisher is nil even with events")
	})
}


// TestResumeParentFlowNoOp tests resume parent flow no op scenarios.
func TestResumeParentFlowNoOp(t *testing.T) {
	engine := NewFlowEngine(nil, nil, nil)

	t.Run("NoParentInstance", func(t *testing.T) {
		child := &approval.Instance{ApplicantID: "u1"}
		err := engine.ResumeParentFlow(t.Context(), nil, child, approval.InstanceApproved)
		assert.NoError(t, err, "Should be no-op when child has no parent")
	})
}

// TestEvaluateBranchConditionsWithDeptSubject tests evaluate branch conditions with dept subject scenarios.
func TestEvaluateBranchConditionsWithDeptSubject(t *testing.T) {
	registry := strategy.NewStrategyRegistry(
		nil,
		nil,
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
		},
	)

	t.Run("DeptSubjectMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "applicantDeptId", Operator: "eq", Value: "dept_001"},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1", ApplicantDeptID: "dept_001"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for dept condition")
		assert.True(t, match, "Should match when dept ID equals expected value")
	})

	t.Run("DeptSubjectNoMatch", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionField, Subject: "applicantDeptId", Operator: "eq", Value: "dept_999"},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1", ApplicantDeptID: "dept_001"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for dept condition")
		assert.False(t, match, "Should not match when dept ID differs from expected value")
	})
}

// TestFlowEngineProcessorRegistration tests flow engine processor registration scenarios.
func TestFlowEngineProcessorRegistration(t *testing.T) {
	t.Run("AllProcessorTypes", func(t *testing.T) {
		processors := []NodeProcessor{
			NewStartProcessor(),
			NewEndProcessor(),
			NewConditionProcessor(),
			NewApprovalProcessor(nil),
			NewHandleProcessor(nil),
			NewSubFlowProcessor(),
		}
		engine := NewFlowEngine(nil, processors, nil)

		assert.Len(t, engine.processors, 6, "Should register all 6 processor types")
		assert.IsType(t, &StartProcessor{}, engine.processors[approval.NodeStart], "Should register StartProcessor")
		assert.IsType(t, &EndProcessor{}, engine.processors[approval.NodeEnd], "Should register EndProcessor")
		assert.IsType(t, &ConditionProcessor{}, engine.processors[approval.NodeCondition], "Should register ConditionProcessor")
		assert.IsType(t, &ApprovalProcessor{}, engine.processors[approval.NodeApproval], "Should register ApprovalProcessor")
		assert.IsType(t, &HandleProcessor{}, engine.processors[approval.NodeHandle], "Should register HandleProcessor")
		assert.IsType(t, &SubFlowProcessor{}, engine.processors[approval.NodeSubFlow], "Should register SubFlowProcessor")
	})

	t.Run("DuplicateOverrides", func(t *testing.T) {
		p1 := NewStartProcessor()
		p2 := NewStartProcessor()
		processors := []NodeProcessor{p1, p2}
		engine := NewFlowEngine(nil, processors, nil)

		assert.Same(t, p2, engine.processors[approval.NodeStart], "Should use the last registered processor for duplicate kinds")
	})
}


// TestEvaluateBranchConditionsExpressionWithDept tests evaluate branch conditions expression with dept scenarios.
func TestEvaluateBranchConditionsExpressionWithDept(t *testing.T) {
	registry := strategy.NewStrategyRegistry(
		nil,
		nil,
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
			strategy.NewExpressionConditionEvaluator(),
		},
	)

	t.Run("DeptInExpression", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `applicantDeptId == "finance"`},
		}
		formData := approval.NewFormData(nil)
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1", ApplicantDeptID: "finance"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for dept expression")
		assert.True(t, match, "Should match when dept equals expected value in expression")
	})

	t.Run("ComplexExpression", func(t *testing.T) {
		conditions := []approval.Condition{
			{Type: approval.ConditionExpression, Expression: `formData["amount"] > 100 && applicantId != "admin"`},
		}
		formData := approval.NewFormData(map[string]any{"amount": float64(500)})
		evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "user1"}

		match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
		require.NoError(t, err, "Should not return error for complex expression")
		assert.True(t, match, "Should match for complex boolean expression")
	})
}

// TestEvaluateBranchConditionsFieldOperators tests evaluate branch conditions field operators scenarios.
func TestEvaluateBranchConditionsFieldOperators(t *testing.T) {
	registry := strategy.NewStrategyRegistry(
		nil,
		nil,
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
		},
	)

	tests := []struct {
		name     string
		formData map[string]any
		operator string
		value    any
		expected bool
	}{
		{
			name:     "LessThan",
			formData: map[string]any{"amount": float64(100)},
			operator: "lt",
			value:    float64(200),
			expected: true,
		},
		{
			name:     "LessThanOrEqual",
			formData: map[string]any{"amount": float64(200)},
			operator: "lte",
			value:    float64(200),
			expected: true,
		},
		{
			name:     "NotEqual",
			formData: map[string]any{"amount": float64(100)},
			operator: "ne",
			value:    float64(200),
			expected: true,
		},
		{
			name:     "Equal",
			formData: map[string]any{"amount": float64(100)},
			operator: "eq",
			value:    float64(100),
			expected: true,
		},
		{
			name:     "ContainsString",
			formData: map[string]any{"name": "hello world"},
			operator: "contains",
			value:    "world",
			expected: true,
		},
		{
			name:     "NotContainsString",
			formData: map[string]any{"name": "hello world"},
			operator: "not_contains",
			value:    "foo",
			expected: true,
		},
		{
			name:     "StartsWithString",
			formData: map[string]any{"name": "hello world"},
			operator: "starts_with",
			value:    "hello",
			expected: true,
		},
		{
			name:     "EndsWithString",
			formData: map[string]any{"name": "hello world"},
			operator: "ends_with",
			value:    "world",
			expected: true,
		},
		{
			name:     "IsEmpty",
			formData: map[string]any{"name": ""},
			operator: "is_empty",
			value:    nil,
			expected: true,
		},
		{
			name:     "IsNotEmpty",
			formData: map[string]any{"name": "hello"},
			operator: "is_not_empty",
			value:    nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject := "amount"
			if _, ok := tt.formData["name"]; ok {
				subject = "name"
			}

			conditions := []approval.Condition{
				{Type: approval.ConditionField, Subject: subject, Operator: tt.operator, Value: tt.value},
			}
			formData := approval.NewFormData(tt.formData)
			evalCtx := &approval.EvaluationContext{FormData: formData, ApplicantID: "u1"}

			match, err := evaluateGroupConditions(registry, t.Context(), evalCtx, conditions)
			require.NoError(t, err, "Should not return error for valid condition")
			assert.Equal(t, tt.expected, match, "Should return expected match result for operator %s", tt.operator)
		})
	}
}
