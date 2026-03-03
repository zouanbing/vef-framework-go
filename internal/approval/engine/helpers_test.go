package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/timex"
)

// TestDeduplicateAssignees tests deduplicate assignees scenarios.
func TestDeduplicateAssignees(t *testing.T) {
	tests := []struct {
		name      string
		action    approval.DuplicateHandlerAction
		assignees []approval.ResolvedAssignee
		expected  []approval.ResolvedAssignee
	}{
		{
			name:   "NoneAction",
			action: approval.DuplicateHandlerNone,
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u1"}, {UserID: "u2"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u1"}, {UserID: "u2"},
			},
		},
		{
			name:   "RemoveDuplicates",
			action: approval.DuplicateHandlerAutoPass,
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u1"}, {UserID: "u3"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
		},
		{
			name:   "NoDuplicates",
			action: approval.DuplicateHandlerAutoPass,
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u3"},
			},
		},
		{
			name:     "EmptySlice",
			action:   approval.DuplicateHandlerAutoPass,
			expected: []approval.ResolvedAssignee{},
		},
		{
			name:   "AllSame",
			action: approval.DuplicateHandlerAutoPass,
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u1"}, {UserID: "u1"},
			},
			expected: []approval.ResolvedAssignee{
				{UserID: "u1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &approval.FlowNode{DuplicateHandlerAction: tt.action}
			got := deduplicateAssignees(node, tt.assignees)
			assert.Equal(t, tt.expected, got, "Should return expected assignees")
		})
	}
}


// TestMatchDelegation tests match delegation scenarios.
func TestMatchDelegation(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name           string
		delegations    []approval.Delegation
		flowID         string
		flowCategoryID string
		expectedID     string // DelegateeID of expected match, empty if nil
	}{
		{
			name: "FlowSpecificMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("flow1")},
			},
			flowID:     "flow1",
			expectedID: "d1",
		},
		{
			name: "CategoryMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowCategoryID: new("cat1")},
			},
			flowCategoryID: "cat1",
			expectedID:     "d1",
		},
		{
			name: "GlobalMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1"},
			},
			expectedID: "d1",
		},
		{
			name: "FlowOverCategory",
			delegations: []approval.Delegation{
				{DelegateeID: "cat-match", FlowCategoryID: new("cat1")},
				{DelegateeID: "flow-match", FlowID: new("flow1")},
			},
			flowID:         "flow1",
			flowCategoryID: "cat1",
			expectedID:     "flow-match",
		},
		{
			name: "CategoryOverGlobal",
			delegations: []approval.Delegation{
				{DelegateeID: "global-match"},
				{DelegateeID: "cat-match", FlowCategoryID: new("cat1")},
			},
			flowCategoryID: "cat1",
			expectedID:     "cat-match",
		},
		{
			name: "ExpiredDelegation",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", EndTime: timex.DateTime(past)},
			},
			expectedID: "",
		},
		{
			name: "NotStartedDelegation",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", StartTime: timex.DateTime(future)},
			},
			expectedID: "",
		},
		{
			name: "NoMatch",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("other-flow")},
			},
			flowID:     "flow1",
			expectedID: "",
		},
		{
			name:       "EmptyList",
			expectedID: "",
		},
		{
			name: "WrongFlowID",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowID: new("wrong-flow")},
			},
			flowID:     "flow1",
			expectedID: "",
		},
		{
			name: "WrongCategoryID",
			delegations: []approval.Delegation{
				{DelegateeID: "d1", FlowCategoryID: new("wrong-cat")},
			},
			flowCategoryID: "cat1",
			expectedID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchDelegation(tt.delegations, now, tt.flowID, tt.flowCategoryID)
			if tt.expectedID == "" {
				assert.Nil(t, got, "Should not match any delegation")
			} else {
				assert.NotNil(t, got, "Should match a delegation")
				assert.Equal(t, tt.expectedID, got.DelegateeID, "Should match the expected delegatee")
			}
		})
	}
}

// TestGetSuperior tests get superior scenarios.
func TestGetSuperior(t *testing.T) {
	t.Run("NilOrgService", func(t *testing.T) {
		uid, err := getSuperior(t.Context(), nil, "user1")
		require.NoError(t, err, "Should not return error when orgService is nil")
		assert.Empty(t, uid, "Should return empty string when orgService is nil")
	})
}
