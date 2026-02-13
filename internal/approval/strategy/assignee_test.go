package strategy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/null"
)

type mockOrgService struct {
	superiors   map[string]struct{ id, name string }
	deptLeaders map[string][]string
}

func (m *mockOrgService) GetSuperior(_ context.Context, userID string) (string, string, error) {
	if s, ok := m.superiors[userID]; ok {
		return s.id, s.name, nil
	}

	return "", "", nil
}

func (m *mockOrgService) GetDeptLeaders(_ context.Context, deptID string) ([]string, error) {
	if leaders, ok := m.deptLeaders[deptID]; ok {
		return leaders, nil
	}

	return nil, nil
}

type mockUserService struct {
	roleUsers map[string][]string
}

func (m *mockUserService) GetUsersByRole(_ context.Context, roleID string) ([]string, error) {
	if users, ok := m.roleUsers[roleID]; ok {
		return users, nil
	}

	return nil, nil
}

type errOrgService struct{}

func (e *errOrgService) GetSuperior(context.Context, string) (string, string, error) {
	return "", "", errors.New("org service error")
}

func (e *errOrgService) GetDeptLeaders(context.Context, string) ([]string, error) {
	return nil, errors.New("org service error")
}

type errUserService struct{}

func (e *errUserService) GetUsersByRole(context.Context, string) ([]string, error) {
	return nil, errors.New("user service error")
}

func TestUserResolver(t *testing.T) {
	r := NewUserResolver()
	assert.Equal(t, approval.AssigneeUser, r.Kind(), "Should return AssigneeUser kind")

	t.Run("MultipleIDs", func(t *testing.T) {
		rc := &ResolveContext{
			Config: &approval.FlowNodeAssignee{AssigneeIDs: []string{"u1", "u2", "u3"}},
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 3, "Should resolve all user IDs")
		assert.Equal(t, "u1", result[0].UserID, "Should resolve first user")
		assert.Equal(t, "u2", result[1].UserID, "Should resolve second user")
		assert.Equal(t, "u3", result[2].UserID, "Should resolve third user")
	})

	t.Run("EmptyIDs", func(t *testing.T) {
		rc := &ResolveContext{Config: &approval.FlowNodeAssignee{}}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Empty(t, result, "Should return empty for no IDs")
	})
}

func TestSelfResolver(t *testing.T) {
	r := NewSelfResolver()
	assert.Equal(t, approval.AssigneeSelf, r.Kind(), "Should return AssigneeSelf kind")

	t.Run("WithApplicant", func(t *testing.T) {
		rc := &ResolveContext{ApplicantID: "applicant1"}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 1, "Should resolve single applicant")
		assert.Equal(t, "applicant1", result[0].UserID, "Should resolve applicant ID")
	})

	t.Run("EmptyApplicant", func(t *testing.T) {
		rc := &ResolveContext{ApplicantID: ""}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil for empty applicant")
	})
}

func TestRoleResolver(t *testing.T) {
	r := NewRoleResolver()
	assert.Equal(t, approval.AssigneeRole, r.Kind(), "Should return AssigneeRole kind")

	t.Run("MultipleRoles", func(t *testing.T) {
		svc := &mockUserService{
			roleUsers: map[string][]string{
				"role_admin":   {"u1", "u2"},
				"role_manager": {"u3"},
			},
		}
		rc := &ResolveContext{
			Config:      &approval.FlowNodeAssignee{AssigneeIDs: []string{"role_admin", "role_manager"}},
			UserService: svc,
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 3, "Should resolve users from all roles")
		assert.Equal(t, "u1", result[0].UserID, "Should resolve first admin user")
		assert.Equal(t, "u2", result[1].UserID, "Should resolve second admin user")
		assert.Equal(t, "u3", result[2].UserID, "Should resolve manager user")
	})

	t.Run("NilService", func(t *testing.T) {
		rc := &ResolveContext{Config: &approval.FlowNodeAssignee{AssigneeIDs: []string{"r1"}}}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil when UserService is nil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		rc := &ResolveContext{
			Config:      &approval.FlowNodeAssignee{AssigneeIDs: []string{"role1"}},
			UserService: &errUserService{},
		}
		_, err := r.Resolve(context.Background(), rc)
		require.Error(t, err, "Should propagate user service error")
	})
}

func TestSuperiorResolver(t *testing.T) {
	r := NewSuperiorResolver()
	assert.Equal(t, approval.AssigneeSuperior, r.Kind(), "Should return AssigneeSuperior kind")

	t.Run("WithSuperior", func(t *testing.T) {
		svc := &mockOrgService{
			superiors: map[string]struct{ id, name string }{
				"emp1": {id: "mgr1", name: "Manager One"},
			},
		}
		rc := &ResolveContext{ApplicantID: "emp1", OrgService: svc}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 1, "Should resolve single superior")
		assert.Equal(t, "mgr1", result[0].UserID, "Should resolve superior ID")
	})

	t.Run("NoSuperior", func(t *testing.T) {
		svc := &mockOrgService{superiors: map[string]struct{ id, name string }{}}
		rc := &ResolveContext{ApplicantID: "emp1", OrgService: svc}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil when no superior found")
	})

	t.Run("NilService", func(t *testing.T) {
		rc := &ResolveContext{ApplicantID: "emp1"}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil when OrgService is nil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		rc := &ResolveContext{ApplicantID: "emp1", OrgService: &errOrgService{}}
		_, err := r.Resolve(context.Background(), rc)
		require.Error(t, err, "Should propagate org service error")
	})
}

func TestDeptLeaderResolver(t *testing.T) {
	r := NewDeptLeaderResolver()
	assert.Equal(t, approval.AssigneeDeptLeader, r.Kind(), "Should return AssigneeDeptLeader kind")

	t.Run("WithLeaders", func(t *testing.T) {
		svc := &mockOrgService{
			deptLeaders: map[string][]string{"dept1": {"leader1", "leader2"}},
		}
		rc := &ResolveContext{DeptID: "dept1", OrgService: svc}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 2, "Should resolve all leaders")
		assert.Equal(t, "leader1", result[0].UserID, "Should resolve first leader")
		assert.Equal(t, "leader2", result[1].UserID, "Should resolve second leader")
	})

	t.Run("NilService", func(t *testing.T) {
		rc := &ResolveContext{DeptID: "dept1"}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil when OrgService is nil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		rc := &ResolveContext{DeptID: "dept1", OrgService: &errOrgService{}}
		_, err := r.Resolve(context.Background(), rc)
		require.Error(t, err, "Should propagate org service error")
	})
}

func TestDeptResolver(t *testing.T) {
	r := NewDeptResolver()
	assert.Equal(t, approval.AssigneeDept, r.Kind(), "Should return AssigneeDept kind")

	t.Run("SingleDept", func(t *testing.T) {
		svc := &mockOrgService{
			deptLeaders: map[string][]string{"dept1": {"leader1"}},
		}
		rc := &ResolveContext{
			DeptID:     "dept1",
			OrgService: svc,
			Config:     &approval.FlowNodeAssignee{AssigneeIDs: []string{"dept1"}},
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 1, "Should resolve single leader")
		assert.Equal(t, "leader1", result[0].UserID, "Should resolve leader ID")
	})

	t.Run("MultipleDepts", func(t *testing.T) {
		svc := &mockOrgService{
			deptLeaders: map[string][]string{
				"dept1": {"leader1"},
				"dept2": {"leader2", "leader3"},
			},
		}
		rc := &ResolveContext{
			Config:     &approval.FlowNodeAssignee{AssigneeIDs: []string{"dept1", "dept2"}},
			OrgService: svc,
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 3, "Should resolve leaders from all departments")
		assert.Equal(t, "leader1", result[0].UserID, "Should resolve first dept leader")
		assert.Equal(t, "leader2", result[1].UserID, "Should resolve second dept first leader")
		assert.Equal(t, "leader3", result[2].UserID, "Should resolve second dept second leader")
	})

	t.Run("NilService", func(t *testing.T) {
		rc := &ResolveContext{Config: &approval.FlowNodeAssignee{AssigneeIDs: []string{"dept1"}}}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil when OrgService is nil")
	})

	t.Run("ServiceError", func(t *testing.T) {
		rc := &ResolveContext{
			Config:     &approval.FlowNodeAssignee{AssigneeIDs: []string{"dept1"}},
			OrgService: &errOrgService{},
		}
		_, err := r.Resolve(context.Background(), rc)
		require.Error(t, err, "Should propagate org service error")
	})
}

func TestFormFieldResolver(t *testing.T) {
	r := NewFormFieldResolver()
	assert.Equal(t, approval.AssigneeFormField, r.Kind(), "Should return AssigneeFormField kind")

	tests := []struct {
		name     string
		field    string
		formData approval.FormData
		expected []string
	}{
		{"SingleStringValue", "approver", approval.FormData{"approver": "user1"}, []string{"user1"}},
		{"StringArrayValue", "approvers", approval.FormData{"approvers": []string{"u1", "u2"}}, []string{"u1", "u2"}},
		{"AnyArrayValue", "approvers", approval.FormData{"approvers": []any{"u1", "u2"}}, []string{"u1", "u2"}},
		{"EmptyStringValue", "approver", approval.FormData{"approver": ""}, nil},
		{"NonexistentField", "missing", approval.FormData{}, nil},
		{"NonStringNonSliceValue", "count", approval.FormData{"count": 42}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ResolveContext{
				Config:   &approval.FlowNodeAssignee{FormField: null.StringFrom(tt.field)},
				FormData: tt.formData,
			}
			result, err := r.Resolve(context.Background(), rc)
			require.NoError(t, err, "Should resolve without error")

			if tt.expected == nil {
				assert.Nil(t, result, "Should return nil for %s", tt.name)
			} else {
				require.Len(t, result, len(tt.expected), "Should resolve correct count")
				for i, expected := range tt.expected {
					assert.Equal(t, expected, result[i].UserID, "Should resolve user at index %d", i)
				}
			}
		})
	}

	t.Run("EmptyFieldName", func(t *testing.T) {
		rc := &ResolveContext{
			Config:   &approval.FlowNodeAssignee{FormField: null.String{}},
			FormData: approval.FormData{"approver": "user1"},
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		assert.Nil(t, result, "Should return nil for empty field name")
	})

	t.Run("AnySliceWithMixedValues", func(t *testing.T) {
		rc := &ResolveContext{
			Config:   &approval.FlowNodeAssignee{FormField: null.StringFrom("approvers")},
			FormData: approval.FormData{"approvers": []any{"user1", "", "user2", 42}},
		}
		result, err := r.Resolve(context.Background(), rc)
		require.NoError(t, err, "Should resolve without error")
		require.Len(t, result, 2, "Should filter out empty strings and non-string values")
		assert.Equal(t, "user1", result[0].UserID, "Should resolve first valid user")
		assert.Equal(t, "user2", result[1].UserID, "Should resolve second valid user")
	})
}

func TestCompositeResolver(t *testing.T) {
	t.Run("ResolveAll", func(t *testing.T) {
		composite := NewCompositeResolver(NewUserResolver(), NewSelfResolver())
		configs := []*approval.FlowNodeAssignee{
			{AssigneeKind: approval.AssigneeUser, AssigneeIDs: []string{"u1", "u2"}},
			{AssigneeKind: approval.AssigneeSelf},
		}
		baseCtx := &ResolveContext{ApplicantID: "applicant1"}

		result, err := composite.ResolveAll(context.Background(), configs, baseCtx)
		require.NoError(t, err, "Should resolve all without error")
		require.Len(t, result, 3, "Should resolve assignees from all configs")
		assert.Equal(t, "u1", result[0].UserID, "Should resolve first user")
		assert.Equal(t, "u2", result[1].UserID, "Should resolve second user")
		assert.Equal(t, "applicant1", result[2].UserID, "Should resolve applicant as self")
	})

	t.Run("UnknownKindSkipped", func(t *testing.T) {
		composite := NewCompositeResolver(NewUserResolver())
		configs := []*approval.FlowNodeAssignee{
			{AssigneeKind: approval.AssigneeRole, AssigneeIDs: []string{"r1"}},
		}

		result, err := composite.ResolveAll(context.Background(), configs, &ResolveContext{})
		require.NoError(t, err, "Should resolve without error")
		assert.Empty(t, result, "Should skip unknown resolver kind")
	})

	t.Run("ErrorPropagation", func(t *testing.T) {
		composite := NewCompositeResolver(NewSuperiorResolver())
		configs := []*approval.FlowNodeAssignee{
			{AssigneeKind: approval.AssigneeSuperior},
		}
		baseCtx := &ResolveContext{ApplicantID: "emp1", OrgService: &errOrgService{}}

		_, err := composite.ResolveAll(context.Background(), configs, baseCtx)
		require.Error(t, err, "Should propagate resolver error")
	})
}
