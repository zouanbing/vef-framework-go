package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

// TestNewApprovalProcessor tests new approval processor scenarios.
func TestNewApprovalProcessor(t *testing.T) {
	p := NewApprovalProcessor(nil)
	require.NotNil(t, p, "Should return a non-nil processor")
	assert.Equal(t, approval.NodeApproval, p.NodeKind(), "Should return NodeApproval kind")
}

// TestIsSameApplicant tests is same applicant scenarios.
func TestIsSameApplicant(t *testing.T) {
	p := &ApprovalProcessor{}

	tests := []struct {
		name        string
		assignees   []approval.ResolvedAssignee
		applicantID string
		expected    bool
	}{
		{
			name: "AllSame",
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u1"}, {UserID: "u1"},
			},
			applicantID: "u1",
			expected:    true,
		},
		{
			name: "OneDifferent",
			assignees: []approval.ResolvedAssignee{
				{UserID: "u1"}, {UserID: "u2"}, {UserID: "u1"},
			},
			applicantID: "u1",
			expected:    false,
		},
		{
			name:        "Empty",
			applicantID: "u1",
			expected:    false,
		},
		{
			name:        "SingleMatch",
			assignees:   []approval.ResolvedAssignee{{UserID: "u1"}},
			applicantID: "u1",
			expected:    true,
		},
		{
			name:        "SingleNoMatch",
			assignees:   []approval.ResolvedAssignee{{UserID: "u2"}},
			applicantID: "u1",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.isSameApplicant(tt.assignees, tt.applicantID)
			assert.Equal(t, tt.expected, got, "Should return expected same-applicant result")
		})
	}
}

