package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/decimal"
)

func TestFieldConditionEvaluator(t *testing.T) {
	e := NewFieldConditionEvaluator()
	assert.Equal(t, approval.ConditionField, e.Type(), "Should return ConditionField type")

	ctx := context.Background()
	ec := &approval.EvalContext{
		FormData: approval.FormData{
			"name":       "alice",
			"amount":     5000,
			"amountF":    5000.5,
			"department": "sales",
			"tags":       []string{"vip", "premium"},
			"greeting":   "hello world",
			"empty_str":  "",
			"int_val":    int64(100),
		},
		ApplicantID: "user1",
		DeptID:      "dept1",
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		expected bool
	}{
		// eq / ne
		{"EqStringMatch", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "eq", Value: "alice"}, true},
		{"EqStringNoMatch", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "eq", Value: "bob"}, false},
		{"NeString", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "ne", Value: "bob"}, true},
		{"NeStringSame", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "ne", Value: "alice"}, false},

		// gt / gte / lt / lte (int)
		{"GtIntTrue", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 3000}, true},
		{"GtIntFalse", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 5000}, false},
		{"GteIntEqual", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "gte", Value: 5000}, true},
		{"LtIntTrue", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "lt", Value: 6000}, true},
		{"LtIntFalse", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "lt", Value: 5000}, false},
		{"LteIntEqual", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "lte", Value: 5000}, true},

		// gt / lt (float64)
		{"GtFloatTrue", approval.Condition{Type: approval.ConditionField, Subject: "amountF", Operator: "gt", Value: 5000.0}, true},
		{"LtFloatTrue", approval.Condition{Type: approval.ConditionField, Subject: "amountF", Operator: "lt", Value: 5001.0}, true},

		// in / not_in
		{"InStringArray", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"alice", "bob"}}, true},
		{"InStringArrayNotFound", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "in", Value: []string{"bob", "charlie"}}, false},
		{"NotInStringArray", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"bob", "charlie"}}, true},
		{"NotInStringArrayFound", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "not_in", Value: []string{"alice", "bob"}}, false},
		{"InWithAnySlice", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "in", Value: []any{"alice", "charlie"}}, true},

		// contains / not_contains
		{"ContainsTrue", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "world"}, true},
		{"ContainsFalse", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "contains", Value: "mars"}, false},
		{"NotContainsTrue", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "not_contains", Value: "mars"}, true},
		{"NotContainsFalse", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "not_contains", Value: "world"}, false},

		// starts_with / ends_with
		{"StartsWithTrue", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "starts_with", Value: "hello"}, true},
		{"StartsWithFalse", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "starts_with", Value: "world"}, false},
		{"EndsWithTrue", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "ends_with", Value: "world"}, true},
		{"EndsWithFalse", approval.Condition{Type: approval.ConditionField, Subject: "greeting", Operator: "ends_with", Value: "hello"}, false},

		// is_empty / is_not_empty
		{"IsEmptyNil", approval.Condition{Type: approval.ConditionField, Subject: "nonexistent", Operator: "is_empty"}, true},
		{"IsEmptyEmptyString", approval.Condition{Type: approval.ConditionField, Subject: "empty_str", Operator: "is_empty"}, true},
		{"IsEmptyNonEmpty", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "is_empty"}, false},
		{"IsNotEmptyString", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "is_not_empty"}, true},
		{"IsNotEmptyNil", approval.Condition{Type: approval.ConditionField, Subject: "nonexistent", Operator: "is_not_empty"}, false},

		// Special subjects
		{"ApplicantSubject", approval.Condition{Type: approval.ConditionField, Subject: "applicant", Operator: "eq", Value: "user1"}, true},
		{"DeptSubject", approval.Condition{Type: approval.ConditionField, Subject: "dept", Operator: "eq", Value: "dept1"}, true},
		{"DepartmentSubjectAlias", approval.Condition{Type: approval.ConditionField, Subject: "department", Operator: "eq", Value: "dept1"}, true},

		// Unknown operator
		{"UnknownOperator", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "unknown_op", Value: "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.cond, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.Equal(t, tt.expected, result, "Should return expected result")
		})
	}
}

func TestFieldConditionEvaluatorSymbolOperators(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvalContext{
		FormData: approval.FormData{
			"name":   "alice",
			"amount": 5000,
			"tags":   []string{"vip", "premium"},
		},
	}

	tests := []struct {
		name     string
		cond     approval.Condition
		expected bool
	}{
		{"GreaterThan", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: ">", Value: 1000}, true},
		{"GreaterEqual", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: ">=", Value: 5000}, true},
		{"LessThan", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "<", Value: 10000}, true},
		{"LessEqual", approval.Condition{Type: approval.ConditionField, Subject: "amount", Operator: "<=", Value: 5000}, true},
		{"EqualSymbol", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "==", Value: "alice"}, true},
		{"NotEqualSymbol", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "!=", Value: "bob"}, true},
		{"NotInWithSpace", approval.Condition{Type: approval.ConditionField, Subject: "name", Operator: "not in", Value: []string{"bob"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.cond, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.Equal(t, tt.expected, result, "Should return expected result")
		})
	}
}

func TestFieldConditionEvaluatorEmptyCollections(t *testing.T) {
	e := NewFieldConditionEvaluator()
	ctx := context.Background()
	ec := &approval.EvalContext{
		FormData: approval.FormData{
			"empty_arr":     []string{},
			"empty_any_arr": []any{},
			"empty_map":     map[string]any{},
		},
	}

	tests := []struct {
		name    string
		subject string
	}{
		{"EmptyStringArray", "empty_arr"},
		{"EmptyAnyArray", "empty_any_arr"},
		{"EmptyMap", "empty_map"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, approval.Condition{Subject: tt.subject, Operator: "is_empty"}, ec)
			require.NoError(t, err, "Should evaluate without error")
			assert.True(t, result, "Should detect empty collection")
		})
	}
}

func TestExpressionConditionEvaluator(t *testing.T) {
	e := NewExpressionConditionEvaluator()
	assert.Equal(t, approval.ConditionExpression, e.Type(), "Should return ConditionExpression type")

	ctx := context.Background()

	t.Run("SimpleComparison", func(t *testing.T) {
		ec := &approval.EvalContext{
			FormData:    approval.FormData{"amount": 5000},
			ApplicantID: "user1",
			DeptID:      "dept1",
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: "form.amount > 3000"}, ec)
		require.NoError(t, err, "Should evaluate greater-than expression")
		assert.True(t, result, "Should be true for 5000 > 3000")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: "form.amount > 10000"}, ec)
		require.NoError(t, err, "Should evaluate greater-than expression")
		assert.False(t, result, "Should be false for 5000 > 10000")
	})

	t.Run("LogicalCombination", func(t *testing.T) {
		ec := &approval.EvalContext{
			FormData:    approval.FormData{"amount": 5000, "department": "sales"},
			ApplicantID: "user1",
			DeptID:      "dept1",
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `form.amount > 1000 && form.department == "sales"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.True(t, result, "Should be true when both conditions match")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `form.amount > 1000 && form.department == "hr"`}, ec)
		require.NoError(t, err, "Should evaluate AND expression")
		assert.False(t, result, "Should be false when department does not match")
	})

	t.Run("BuiltInVariables", func(t *testing.T) {
		ec := &approval.EvalContext{
			FormData:    approval.FormData{},
			ApplicantID: "user1",
			DeptID:      "dept_sales",
		}

		result, err := e.Evaluate(ctx, approval.Condition{Expression: `applicant == "user1"`}, ec)
		require.NoError(t, err, "Should evaluate applicant expression")
		assert.True(t, result, "Should match applicant ID")

		result, err = e.Evaluate(ctx, approval.Condition{Expression: `dept == "dept_sales"`}, ec)
		require.NoError(t, err, "Should evaluate dept expression")
		assert.True(t, result, "Should match dept ID")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		ec := &approval.EvalContext{FormData: approval.FormData{}}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "invalid @@@ syntax"}, ec)
		require.Error(t, err, "Should fail for invalid syntax")
		assert.Contains(t, err.Error(), "compile expression", "Should wrap compile error")
	})

	t.Run("RuntimeError", func(t *testing.T) {
		ec := &approval.EvalContext{
			FormData:    approval.FormData{"amount": "not_a_number"},
			ApplicantID: "user1",
			DeptID:      "dept1",
		}
		_, err := e.Evaluate(ctx, approval.Condition{Expression: "form.amount > 100"}, ec)
		require.Error(t, err, "Should fail when expression evaluation fails at runtime")
		assert.Contains(t, err.Error(), "run expression", "Should wrap runtime error")
	})
}

func TestToDecimal(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"Int", int(42), "42"},
		{"Int32", int32(42), "42"},
		{"Int64", int64(42), "42"},
		{"Float32", float32(3.14), "3.14"},
		{"Float64", float64(3.14), "3.14"},
		{"String", "99.9", "99.9"},
		{"Decimal", decimal.NewFromInt(7), "7"},
		{"Default", struct{}{}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, toDecimal(tt.input).String(), "Should convert to expected decimal")
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name      string
		container any
		item      any
		expected  bool
	}{
		{"IntSliceFound", []int{1, 2, 3}, 2, true},
		{"IntSliceNotFound", []int{1, 2, 3}, 4, false},
		{"IntSliceWrongType", []int{1, 2, 3}, "2", false},
		{"Int64SliceFound", []int64{10, 20}, int64(10), true},
		{"Int64SliceNotFound", []int64{10, 20}, int64(30), false},
		{"Int64SliceWrongType", []int64{10, 20}, "10", false},
		{"UnsupportedType", 42, "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, containsAny(tt.container, tt.item), "Should return expected containment result")
		})
	}
}

func TestIsEmpty(t *testing.T) {
	assert.False(t, isEmpty(42), "Should return false for non-nil non-container type")
}

func TestToString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"StringValue", "hello", "hello"},
		{"NonStringValue", 42, ""},
		{"NilValue", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, toString(tt.input), "Should convert to expected string")
		})
	}
}
