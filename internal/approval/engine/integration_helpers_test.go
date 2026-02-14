package engine

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/internal/database"
	internalORM "github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
)

// allModels contains all approval models for table creation.
var allModels = []any{
	(*approval.Flow)(nil),
	(*approval.FlowVersion)(nil),
	(*approval.FlowNode)(nil),
	(*approval.FlowNodeAssignee)(nil),
	(*approval.FlowEdge)(nil),
	(*approval.Instance)(nil),
	(*approval.Task)(nil),
	(*approval.ActionLog)(nil),
	(*approval.FormSnapshot)(nil),
	(*approval.EventOutbox)(nil),
	(*approval.CCRecord)(nil),
	(*approval.Delegation)(nil),
	(*approval.ParallelRecord)(nil),
	(*approval.FlowCategory)(nil),
	(*approval.FlowInitiator)(nil),
	(*approval.FlowNodeCC)(nil),
	(*approval.FlowFormField)(nil),
}

// setupTestDB creates an in-memory SQLite database with all approval tables.
func setupTestDB(t *testing.T) (orm.DB, func()) {
	t.Helper()

	dsConfig := &config.DataSourceConfig{Kind: config.SQLite}

	bunDB, err := database.New(dsConfig)
	require.NoError(t, err)

	bunDB.RegisterModel(allModels...)

	ctx := context.Background()
	for _, m := range allModels {
		_, err := bunDB.NewCreateTable().Model(m).IfNotExists().Exec(ctx)
		require.NoError(t, err)
	}

	db := internalORM.New(bunDB)

	return db, func() { _ = bunDB.Close() }
}

// MockOrganizationService provides a mock implementation of OrganizationService.
type MockOrganizationService struct {
	superiors   map[string]struct{ id, name string }
	deptLeaders map[string][]string
	err         error
}

func (m *MockOrganizationService) GetSuperior(_ context.Context, userID string) (string, string, error) {
	if m != nil && m.superiors != nil {
		if s, ok := m.superiors[userID]; ok {
			return s.id, s.name, nil
		}
	}

	if m != nil && m.err != nil {
		return "", "", m.err
	}

	return "", "", nil
}

func (m *MockOrganizationService) GetDeptLeaders(_ context.Context, deptID string) ([]string, error) {
	if m != nil && m.deptLeaders != nil {
		if leaders, ok := m.deptLeaders[deptID]; ok {
			return leaders, nil
		}
	}

	if m != nil && m.err != nil {
		return nil, m.err
	}

	return nil, nil
}

// MockUserService provides a mock implementation of UserService.
type MockUserService struct {
	roleUsers map[string][]string
}

func (m *MockUserService) GetUsersByRole(_ context.Context, roleID string) ([]string, error) {
	if m != nil && m.roleUsers != nil {
		if users, ok := m.roleUsers[roleID]; ok {
			return users, nil
		}
	}

	return nil, nil
}

// setupEngine creates a FlowEngine with mock services and an optional EventPublisher.
func setupEngine(mockOrg *MockOrganizationService, mockUser *MockUserService, pub ...*publisher.EventPublisher) *FlowEngine {
	registry := strategy.NewStrategyRegistry(
		[]approval.PassRuleStrategy{
			strategy.NewAllPassStrategy(),
			strategy.NewOnePassStrategy(),
			strategy.NewRatioPassStrategy(),
			strategy.NewOneRejectStrategy(),
		},
		[]strategy.AssigneeResolver{
			strategy.NewUserResolver(),
			strategy.NewRoleResolver(),
			strategy.NewDeptResolver(),
			strategy.NewSelfResolver(),
			strategy.NewSuperiorResolver(),
			strategy.NewDeptLeaderResolver(),
			strategy.NewFormFieldResolver(),
		},
		[]approval.ConditionEvaluator{
			strategy.NewFieldConditionEvaluator(),
			strategy.NewExpressionConditionEvaluator(),
		},
	)

	subflowProcessor := NewSubFlowProcessor()
	processors := []NodeProcessor{
		NewStartProcessor(),
		NewEndProcessor(),
		NewConditionProcessor(),
		NewApprovalProcessor(mockOrg, mockUser),
		NewHandleProcessor(mockOrg, mockUser),
		subflowProcessor,
	}

	var eventPub *publisher.EventPublisher
	if len(pub) > 0 {
		eventPub = pub[0]
	}

	eng := NewFlowEngine(registry, processors, eventPub)
	subflowProcessor.SetFlowEngine(eng)

	return eng
}

// buildSimpleFlow creates: Start -> Approval(user1,user2 sequential, all pass) -> End.
func buildSimpleFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, "simple_flow", "Simple Flow")

	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	approvalNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval1",
		NodeKind:               approval.NodeApproval,
		Name:                   "Approval",
		ApprovalMethod:         approval.ApprovalSequential,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err)

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	for i, uid := range []string{"user1", "user2"} {
		insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{uid}, i)
	}

	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	return flow, version, startNode, approvalNode, endNode
}

// buildAutoCompleteFlow creates: Start -> End.
func buildAutoCompleteFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, "auto_complete_flow", "Auto Complete Flow")

	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")
	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertEdge(t, ctx, db, version.ID, startNode.ID, endNode.ID)

	return flow, version, startNode, endNode
}

// buildHandleFlow creates: Start -> Handle(user1,user2, any-pass) -> End.
func buildHandleFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, "handle_flow", "Handle Flow")

	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	handleNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "handle1",
		NodeKind:               approval.NodeHandle,
		Name:                   "Handle",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAny,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	handleNode.ID = id.Generate()
	handleNode.CreatedBy = "system"
	handleNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(handleNode).Exec(ctx)
	require.NoError(t, err)

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, handleNode.ID, approval.AssigneeUser, []string{"user1", "user2"}, 0)

	insertEdge(t, ctx, db, version.ID, startNode.ID, handleNode.ID)
	insertEdge(t, ctx, db, version.ID, handleNode.ID, endNode.ID)

	return flow, version, startNode, handleNode, endNode
}

// buildBranchFlow creates: Start -> Branch -> [condition: amount>1000] Approval1 -> End, [default] Approval2 -> End.
func buildBranchFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, "branch_flow", "Branch Flow")

	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")
	branchNode := createFlowNode(t, ctx, db, version.ID, "branch1", approval.NodeCondition, "Branch")

	approval1 := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval_high",
		NodeKind:               approval.NodeApproval,
		Name:                   "High Value Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approval1.ID = id.Generate()
	approval1.CreatedBy = "system"
	approval1.UpdatedBy = "system"
	_, err := db.NewInsert().Model(approval1).Exec(ctx)
	require.NoError(t, err)

	approval2 := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval_low",
		NodeKind:               approval.NodeApproval,
		Name:                   "Low Value Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approval2.ID = id.Generate()
	approval2.CreatedBy = "system"
	approval2.UpdatedBy = "system"
	_, err = db.NewInsert().Model(approval2).Exec(ctx)
	require.NoError(t, err)

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, approval1.ID, approval.AssigneeUser, []string{"manager1"}, 0)
	insertAssignee(t, ctx, db, approval2.ID, approval.AssigneeUser, []string{"user1"}, 0)

	// Set branches on the condition node
	highBranchID := id.Generate()
	defaultBranchID := id.Generate()
	branchNode.Branches = []approval.ConditionBranch{
		{
			ID:    highBranchID,
			Label: "High Value",
			Conditions: []approval.Condition{
				{Type: approval.ConditionField, Subject: "amount", Operator: ">", Value: float64(1000)},
			},
			Priority: 0,
		},
		{
			ID:        defaultBranchID,
			Label:     "Default",
			IsDefault: true,
			Priority:  1,
		},
	}
	_, err = db.NewUpdate().Model(branchNode).WherePK().Exec(ctx)
	require.NoError(t, err)

	insertEdge(t, ctx, db, version.ID, startNode.ID, branchNode.ID)
	insertEdge(t, ctx, db, version.ID, branchNode.ID, approval1.ID, highBranchID)
	insertEdge(t, ctx, db, version.ID, branchNode.ID, approval2.ID, defaultBranchID)
	insertEdge(t, ctx, db, version.ID, approval1.ID, endNode.ID)
	insertEdge(t, ctx, db, version.ID, approval2.ID, endNode.ID)

	return flow, version, startNode, branchNode, approval1, approval2, endNode
}

// buildFlowWithSameApplicant creates a flow where the approval node assignee is the same as applicant.
func buildFlowWithSameApplicant(t *testing.T, ctx context.Context, db orm.DB, sameAction approval.SameApplicantAction, adminIDs []string) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, "same_applicant_"+id.Generate(), "Same Applicant Flow")

	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	approvalNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval_same",
		NodeKind:               approval.NodeApproval,
		Name:                   "Same Applicant Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		SameApplicantAction:    sameAction,
		AdminUserIDs:           adminIDs,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err)

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{"user1"}, 0)
	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	return flow, version, startNode, approvalNode, endNode
}

// buildEmptyAssigneeFlow creates: Start -> CustomNode(no assignees) -> End.
// Used by tests that verify empty-assignee fallback behavior for both approval and handle nodes.
func buildEmptyAssigneeFlow(
	t *testing.T, ctx context.Context, db orm.DB,
	code string,
	nodeKind approval.NodeKind,
	emptyAction approval.EmptyHandlerAction,
	opts ...func(*approval.FlowNode),
) (*approval.Flow, *approval.FlowVersion, *approval.FlowNode) {
	t.Helper()

	flow, version := createFlowAndVersion(t, ctx, db, code, code)
	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	passRule := approval.PassAll
	if nodeKind == approval.NodeHandle {
		passRule = approval.PassAny
	}

	node := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "target_node",
		NodeKind:               nodeKind,
		Name:                   code,
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               passRule,
		PassRatio:              decimal.Zero,
		EmptyHandlerAction:     emptyAction,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	for _, opt := range opts {
		opt(node)
	}
	node.ID = id.Generate()
	node.CreatedBy = "system"
	node.UpdatedBy = "system"
	_, err := db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err)

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertEdge(t, ctx, db, version.ID, startNode.ID, node.ID)
	insertEdge(t, ctx, db, version.ID, node.ID, endNode.ID)

	return flow, version, node
}

// ---- Small Helpers ----

// createFlowAndVersion creates a Flow and published FlowVersion.
func createFlowAndVersion(t *testing.T, ctx context.Context, db orm.DB, code, name string) (*approval.Flow, *approval.FlowVersion) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 code,
		Name:                 name,
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{FlowID: flow.ID, Version: 1, Status: approval.VersionPublished}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	return flow, version
}

func createFlowNode(t *testing.T, ctx context.Context, db orm.DB, versionID, key string, kind approval.NodeKind, name string) *approval.FlowNode {
	t.Helper()

	node := &approval.FlowNode{
		FlowVersionID: versionID,
		NodeKey:       key,
		NodeKind:      kind,
		Name:          name,
		PassRatio:     decimal.Zero,
	}
	node.ID = id.Generate()
	node.CreatedBy = "system"
	node.UpdatedBy = "system"

	_, err := db.NewInsert().Model(node).Exec(ctx)
	require.NoError(t, err)

	return node
}

func insertAssignee(t *testing.T, ctx context.Context, db orm.DB, nodeID string, kind approval.AssigneeKind, ids []string, sortOrder int) {
	t.Helper()

	assignee := &approval.FlowNodeAssignee{
		NodeID:       nodeID,
		AssigneeKind: kind,
		AssigneeIDs:  ids,
		SortOrder:    sortOrder,
	}
	assignee.ID = id.Generate()

	_, err := db.NewInsert().Model(assignee).Exec(ctx)
	require.NoError(t, err)
}

func insertEdge(t *testing.T, ctx context.Context, db orm.DB, versionID, sourceID, targetID string, sourceHandle ...string) {
	t.Helper()

	edge := &approval.FlowEdge{
		FlowVersionID: versionID,
		SourceNodeID:  sourceID,
		TargetNodeID:  targetID,
	}
	edge.ID = id.Generate()

	if len(sourceHandle) > 0 && sourceHandle[0] != "" {
		edge.SourceHandle = null.StringFrom(sourceHandle[0])
	}

	_, err := db.NewInsert().Model(edge).Exec(ctx)
	require.NoError(t, err)
}

func createInstance(t *testing.T, ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion, applicantID string, formData map[string]any) *approval.Instance {
	t.Helper()

	instance := &approval.Instance{
		FlowID:        flow.ID,
		FlowVersionID: version.ID,
		Title:         "Test Instance",
		SerialNo:      id.Generate(),
		ApplicantID:   applicantID,
		Status:        string(approval.InstanceRunning),
		FormData:      formData,
	}
	instance.ID = id.Generate()
	instance.CreatedBy = applicantID
	instance.UpdatedBy = applicantID

	_, err := db.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	return instance
}

func queryTasksByNode(t *testing.T, ctx context.Context, db orm.DB, instanceID, nodeID string) []approval.Task {
	t.Helper()

	var tasks []approval.Task

	err := db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
		c.Equals("node_id", nodeID)
	}).OrderBy("sort_order").Scan(ctx)
	require.NoError(t, err)

	return tasks
}

func queryInstance(t *testing.T, ctx context.Context, db orm.DB, instanceID string) approval.Instance {
	t.Helper()

	var inst approval.Instance

	err := db.NewSelect().Model(&inst).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instanceID)
	}).Scan(ctx)
	require.NoError(t, err)

	return inst
}

func queryFormSnapshots(t *testing.T, ctx context.Context, db orm.DB, instanceID string) []approval.FormSnapshot {
	t.Helper()

	var snapshots []approval.FormSnapshot

	err := db.NewSelect().Model(&snapshots).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
	}).Scan(ctx)
	require.NoError(t, err)

	return snapshots
}

func createParentInstance(t *testing.T, ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion, applicantID string) *approval.Instance {
	t.Helper()

	instance := &approval.Instance{
		FlowID:        flow.ID,
		FlowVersionID: version.ID,
		Title:         "Parent Instance",
		SerialNo:      id.Generate(),
		ApplicantID:   applicantID,
		Status:        string(approval.InstanceRunning),
	}
	instance.ID = id.Generate()
	instance.CreatedBy = applicantID
	instance.UpdatedBy = applicantID

	_, err := db.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	return instance
}

func createRunningParentInstance(t *testing.T, ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion, currentNodeID string) *approval.Instance {
	t.Helper()

	instance := &approval.Instance{
		FlowID:        flow.ID,
		FlowVersionID: version.ID,
		Title:         "Parent Instance",
		SerialNo:      id.Generate(),
		ApplicantID:   "applicant1",
		Status:        string(approval.InstanceRunning),
		CurrentNodeID: null.StringFrom(currentNodeID),
	}
	instance.ID = id.Generate()
	instance.CreatedBy = "applicant1"
	instance.UpdatedBy = "applicant1"

	_, err := db.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	return instance
}

func createChildInstance(t *testing.T, ctx context.Context, db orm.DB, parentInstanceID, parentNodeID string, status approval.InstanceStatus) *approval.Instance {
	t.Helper()

	instance := &approval.Instance{
		FlowID:           id.Generate(),
		FlowVersionID:    id.Generate(),
		ParentInstanceID: null.StringFrom(parentInstanceID),
		ParentNodeID:     null.StringFrom(parentNodeID),
		Title:            "Child Instance",
		SerialNo:         id.Generate(),
		ApplicantID:      "applicant1",
		Status:           string(status),
	}
	instance.ID = id.Generate()
	instance.CreatedBy = "applicant1"
	instance.UpdatedBy = "applicant1"

	_, err := db.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	return instance
}

func insertDelegation(t *testing.T, ctx context.Context, db orm.DB, delegatorID, delegateeID string, flowID null.String, isActive bool) {
	t.Helper()

	delegation := &approval.Delegation{
		DelegatorID: delegatorID,
		DelegateeID: delegateeID,
		FlowID:      flowID,
		IsActive:    isActive,
	}
	delegation.ID = id.Generate()
	delegation.CreatedBy = "system"
	delegation.UpdatedBy = "system"

	_, err := db.NewInsert().Model(delegation).Exec(ctx)
	require.NoError(t, err)
}
