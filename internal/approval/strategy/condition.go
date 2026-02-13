package strategy

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/constants"
	"github.com/ilxqx/vef-framework-go/decimal"
)

// NewFieldConditionEvaluator creates a new FieldConditionEvaluator.
func NewFieldConditionEvaluator() approval.ConditionEvaluator {
	return new(FieldConditionEvaluator)
}

// FieldConditionEvaluator evaluates field-based conditions.
type FieldConditionEvaluator struct{}

func (e *FieldConditionEvaluator) Type() approval.ConditionKind {
	return approval.ConditionField
}

func (e *FieldConditionEvaluator) Evaluate(_ context.Context, cond approval.Condition, ec *approval.EvalContext) (bool, error) {
	lhs := e.resolveSubject(cond.Subject, ec)

	return e.compare(lhs, cond.Operator, cond.Value), nil
}

func (e *FieldConditionEvaluator) resolveSubject(subject string, ec *approval.EvalContext) any {
	switch subject {
	case "applicant":
		return ec.ApplicantID
	case "dept", "department":
		return ec.DeptID
	default:
		return ec.FormData.Get(subject)
	}
}

func (e *FieldConditionEvaluator) compare(lhs any, operator string, rhs any) bool {
	operator = normalizeOperator(operator)

	switch operator {
	case "eq":
		return reflect.DeepEqual(lhs, rhs)
	case "ne":
		return !reflect.DeepEqual(lhs, rhs)
	case "gt":
		return toDecimal(lhs).GreaterThan(toDecimal(rhs))
	case "gte":
		return toDecimal(lhs).GreaterThanOrEqual(toDecimal(rhs))
	case "lt":
		return toDecimal(lhs).LessThan(toDecimal(rhs))
	case "lte":
		return toDecimal(lhs).LessThanOrEqual(toDecimal(rhs))
	case "in":
		return containsAny(rhs, lhs)
	case "not_in":
		return !containsAny(rhs, lhs)
	case "contains":
		return strings.Contains(toString(lhs), toString(rhs))
	case "not_contains":
		return !strings.Contains(toString(lhs), toString(rhs))
	case "starts_with":
		return strings.HasPrefix(toString(lhs), toString(rhs))
	case "ends_with":
		return strings.HasSuffix(toString(lhs), toString(rhs))
	case "is_empty":
		return isEmpty(lhs)
	case "is_not_empty":
		return !isEmpty(lhs)
	default:
		return false
	}
}

// NewExpressionConditionEvaluator creates a new ExpressionConditionEvaluator.
func NewExpressionConditionEvaluator() approval.ConditionEvaluator {
	return new(ExpressionConditionEvaluator)
}

// ExpressionConditionEvaluator evaluates expr-lang expressions.
type ExpressionConditionEvaluator struct{}

func (e *ExpressionConditionEvaluator) Type() approval.ConditionKind {
	return approval.ConditionExpression
}

func (e *ExpressionConditionEvaluator) Evaluate(_ context.Context, cond approval.Condition, ec *approval.EvalContext) (bool, error) {
	env := map[string]any{
		"form":      ec.FormData.ToMap(),
		"applicant": ec.ApplicantID,
		"dept":      ec.DeptID,
	}

	program, err := expr.Compile(cond.Expression, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression: %w", err)
	}

	b, _ := result.(bool)

	return b, nil
}

func normalizeOperator(operator string) string {
	op := strings.ToLower(strings.TrimSpace(operator))

	switch op {
	case "=", "==":
		return "eq"
	case "!=", "<>":
		return "ne"
	case ">":
		return "gt"
	case ">=":
		return "gte"
	case "<":
		return "lt"
	case "<=":
		return "lte"
	case "not in":
		return "not_in"
	default:
		return op
	}
}

func toDecimal(v any) decimal.Decimal {
	switch val := v.(type) {
	case int:
		return decimal.NewFromInt(int64(val))
	case int32:
		return decimal.NewFromInt32(val)
	case int64:
		return decimal.NewFromInt(val)
	case float32:
		return decimal.NewFromFloat32(val)
	case float64:
		return decimal.NewFromFloat(val)
	case string:
		d, _ := decimal.NewFromString(val)
		return d
	case decimal.Decimal:
		return val
	default:
		return decimal.Zero
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return constants.Empty
}

func containsAny(container, item any) bool {
	switch c := container.(type) {
	case []string:
		return slices.Contains(c, toString(item))
	case []any:
		return slices.ContainsFunc(c, func(v any) bool {
			return reflect.DeepEqual(v, item)
		})
	case []int:
		itemInt, ok := item.(int)
		if !ok {
			return false
		}

		return slices.Contains(c, itemInt)
	case []int64:
		itemInt, ok := item.(int64)
		if !ok {
			return false
		}

		return slices.Contains(c, itemInt)
	default:
		return false
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == constants.Empty
	case []string:
		return len(val) == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}
