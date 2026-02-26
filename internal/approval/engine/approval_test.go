package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
)

// MockAssigneeService provides a minimal mock for approval.AssigneeService used by unit tests.
type MockAssigneeService struct {
	superiors   map[string]string
	deptLeaders map[string][]string
	roleUsers   map[string][]string
}

func (m *MockAssigneeService) GetSuperior(_ context.Context, userID string) (string, error) {
	if m != nil && m.superiors != nil {
		if s, ok := m.superiors[userID]; ok {
			return s, nil
		}
	}
	return "", nil
}

func (m *MockAssigneeService) GetDeptLeaders(_ context.Context, deptID string) ([]string, error) {
	if m != nil && m.deptLeaders != nil {
		if leaders, ok := m.deptLeaders[deptID]; ok {
			return leaders, nil
		}
	}
	return nil, nil
}

func (m *MockAssigneeService) GetRoleUsers(_ context.Context, roleID string) ([]string, error) {
	if m != nil && m.roleUsers != nil {
		if users, ok := m.roleUsers[roleID]; ok {
			return users, nil
		}
	}
	return nil, nil
}

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

// TestPredictSameApplicant tests predict same applicant scenarios.
func TestPredictSameApplicant(t *testing.T) {
	p := NewApprovalProcessor(nil)

	t.Run("AutoPass", func(t *testing.T) {
		pc := &ProcessContext{
			Node:        &approval.FlowNode{SameApplicantAction: approval.SameApplicantAutoPass},
			ApplicantID: "u1",
		}
		ids, err := p.predictSameApplicant(t.Context(), pc)
		require.NoError(t, err, "Should not return error for auto pass")
		assert.Nil(t, ids, "Should return nil IDs for auto pass")
	})

	t.Run("DefaultReturnsApplicant", func(t *testing.T) {
		pc := &ProcessContext{
			Node:        &approval.FlowNode{SameApplicantAction: approval.SameApplicantSelfApprove},
			ApplicantID: "u1",
		}
		ids, err := p.predictSameApplicant(t.Context(), pc)
		require.NoError(t, err, "Should not return error for self approve")
		assert.Equal(t, []string{"u1"}, ids, "Should return applicant ID for self approve")
	})

	t.Run("TransferSuperiorNilOrgService", func(t *testing.T) {
		pc := &ProcessContext{
			Node:        &approval.FlowNode{SameApplicantAction: approval.SameApplicantTransferSuperior},
			ApplicantID: "u1",
		}
		_, err := p.predictSameApplicant(t.Context(), pc)
		assert.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee when orgService is nil and superior is empty")
	})

	t.Run("TransferSuperiorFound", func(t *testing.T) {
		mockOrg := &MockAssigneeService{
			superiors: map[string]string{
				"u1": "boss1",
			},
		}
		p2 := NewApprovalProcessor(mockOrg)
		pc := &ProcessContext{
			Node:        &approval.FlowNode{SameApplicantAction: approval.SameApplicantTransferSuperior},
			ApplicantID: "u1",
		}
		ids, err := p2.predictSameApplicant(t.Context(), pc)
		require.NoError(t, err, "Should not return error when superior found")
		assert.Equal(t, []string{"boss1"}, ids, "Should return superior ID")
	})

}
