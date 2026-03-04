package resource_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"go.uber.org/fx"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	internalApproval "github.com/ilxqx/vef-framework-go/internal/approval"
	"github.com/ilxqx/vef-framework-go/internal/apptest"
	"github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/internal/testx"
	"github.com/ilxqx/vef-framework-go/security"
)

// --- Mock implementations ---

// MockAssigneeService is a no-op implementation of approval.AssigneeService for testing.
type MockAssigneeService struct{}

func (m *MockAssigneeService) GetSuperior(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *MockAssigneeService) GetDeptLeaders(_ context.Context, _ string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockAssigneeService) GetRoleUsers(_ context.Context, _ string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockPrincipalDeptResolver is a test implementation of approval.PrincipalDeptResolver.
type MockPrincipalDeptResolver struct{}

func (m *MockPrincipalDeptResolver) Resolve(_ context.Context, _ *security.Principal) (*string, *string, error) {
	return nil, nil, nil
}

// MockInstanceNoGenerator is a test implementation of approval.InstanceNoGenerator.
type MockInstanceNoGenerator struct {
	counter atomic.Int64
}

func (g *MockInstanceNoGenerator) Generate(_ context.Context, flowCode string) (string, error) {
	n := g.counter.Add(1)
	return fmt.Sprintf("%s-%04d", flowCode, n), nil
}

// --- App setup helper ---

// setupResourceApp creates a Postgres container, boots the full app with approval modules,
// and returns the orm.DB and a JWT token for authenticated requests.
func setupResourceApp(s *apptest.Suite) (orm.DB, string) {
	ctx := context.Background()
	pgContainer := testx.NewPostgresContainer(ctx, s.T())

	var db orm.DB
	s.SetupApp(
		fx.Replace(
			pgContainer.DataSource,
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
			&config.ApprovalConfig{AutoMigrate: true},
		),
		fx.Provide(func() context.Context { return ctx }),
		internalApproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.PrincipalDeptResolver { return &MockPrincipalDeptResolver{} }, fx.As(new(approval.PrincipalDeptResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Populate(&db),
	)

	token := s.GenerateToken(security.NewUser("test-admin", "admin"))
	return db, token
}

// --- Cleanup helpers ---

// cleanRuntimeData removes runtime data (instances, tasks, etc.) while preserving flow definitions.
// Use in TearDownTest.
func cleanRuntimeData(ctx context.Context, db orm.DB) {
	delAll := func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }
	_, _ = db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.ActionLog)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.UrgeRecord)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.CCRecord)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Task)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Instance)(nil)).Where(delAll).Exec(ctx)
}

// cleanAllApprovalData removes all approval data in FK-safe order.
// Use in TearDownSuite.
func cleanAllApprovalData(ctx context.Context, db orm.DB) {
	cleanRuntimeData(ctx, db)
	delAll := func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }
	_, _ = db.NewDelete().Model((*approval.FlowEdge)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNodeCC)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNodeAssignee)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNode)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowVersion)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowInitiator)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Flow)(nil)).Where(delAll).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowCategory)(nil)).Where(delAll).Exec(ctx)
}

// --- JSON helper ---

// mustMarshal marshals v to json.RawMessage, panicking on error.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// toMap converts a struct to map[string]any via JSON round-trip.
// This is needed because mapstructure cannot decode json.RawMessage from map values.
func toMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	return m
}

// --- Flow definition builders ---

// simpleFlowDef returns a minimal valid flow definition: Start → End.
func simpleFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "end-1"},
		},
	}
}

// approvalFlowDef returns a flow definition with an approval node: Start → Approval → End.
func approvalFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"test-admin"}, SortOrder: 1},
					},
					CCs: []approval.CCDefinition{
						{Kind: approval.CCUser, IDs: []string{"cc-user-1"}, Timing: approval.CCTimingAlways},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
					IsTransferAllowed:   true,
				},
				IsManualCCAllowed: true,
				ApprovalMethod:    approval.ApprovalSequential,
				PassRule:          approval.PassAll,
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// complexFlowDef returns a flow with a condition node:
// Start → Condition → [branch "amount>1000": Approval → End] / [default: Handle → End]
func complexFlowDef() approval.FlowDefinition {
	branchHighID := "branch-high"
	branchDefaultID := "branch-default"

	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "condition-1", Kind: approval.NodeCondition, Data: mustMarshal(approval.ConditionNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "条件分支"},
				Branches: []approval.ConditionBranch{
					{
						ID:    branchHighID,
						Label: "大额审批",
						ConditionGroups: []approval.ConditionGroup{
							{Conditions: []approval.Condition{
								{Kind: approval.ConditionField, Subject: "amount", Operator: "gt", Value: 1000},
							}},
						},
						Priority: 1,
					},
					{
						ID:        branchDefaultID,
						Label:     "默认处理",
						IsDefault: true,
						Priority:  2,
					},
				},
			})},
			{ID: "approval-high", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "大额审批"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"manager-1"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
				ApprovalMethod: approval.ApprovalSequential,
				PassRule:       approval.PassAll,
			})},
			{ID: "handle-default", Kind: approval.NodeHandle, Data: mustMarshal(approval.HandleNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "默认办理"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"handler-1"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "condition-1"},
			{ID: "edge-2", Source: "condition-1", Target: "approval-high", SourceHandle: &branchHighID},
			{ID: "edge-3", Source: "condition-1", Target: "handle-default", SourceHandle: &branchDefaultID},
			{ID: "edge-4", Source: "approval-high", Target: "end-1"},
			{ID: "edge-5", Source: "handle-default", Target: "end-1"},
		},
	}
}
