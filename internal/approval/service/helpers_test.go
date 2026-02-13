package service

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/internal/database"
	internalORM "github.com/ilxqx/vef-framework-go/internal/orm"
	"github.com/ilxqx/vef-framework-go/orm"
)

// allModels lists all approval models for table creation.
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

	dsConfig := &config.DatasourceConfig{Type: config.SQLite}

	bunDB, err := database.New(dsConfig)
	require.NoError(t, err)

	// Register models
	bunDB.RegisterModel(allModels...)

	// Create tables
	ctx := context.Background()
	for _, m := range allModels {
		_, err := bunDB.NewCreateTable().Model(m).IfNotExists().Exec(ctx)
		require.NoError(t, err)
	}

	db := internalORM.New(bunDB)

	return db, func() { _ = bunDB.Close() }
}

// setupEngine creates a FlowEngine with mock services.
func setupEngine(mockOrg *MockOrganizationService, mockUser *MockUserService, pub ...*publisher.EventPublisher) *engine.FlowEngine {
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

	subflowProcessor := engine.NewSubFlowProcessor()
	processors := []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewConditionProcessor(),
		engine.NewApprovalProcessor(mockOrg, mockUser),
		engine.NewHandleProcessor(mockOrg, mockUser),
		subflowProcessor,
	}

	var eventPub *publisher.EventPublisher
	if len(pub) > 0 {
		eventPub = pub[0]
	}

	eng := engine.NewFlowEngine(registry, processors, eventPub)
	subflowProcessor.SetFlowEngine(eng)

	return eng
}

// buildSimpleFlow creates: Start -> Approval(user1,user2 sequential, all pass) -> End.
// Returns (flow, version, startNode, approvalNode, endNode).
func buildSimpleFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "simple_flow",
		Name:                 "Simple Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	startNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "start",
		NodeKind:      approval.NodeStart,
		Name:          "Start",
	}
	startNode.ID = id.Generate()
	startNode.CreatedBy = "system"
	startNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(startNode).Exec(ctx)
	require.NoError(t, err)

	approvalNode := &approval.FlowNode{
		FlowVersionID:           version.ID,
		NodeKey:                 "approval1",
		NodeKind:                approval.NodeApproval,
		Name:                    "Approval",
		ApprovalMethod:          approval.ApprovalSequential,
		PassRule:                approval.PassAll,
		PassRatio:               decimal.Zero,
		IsTransferAllowed:       true,
		IsRollbackAllowed:       true,
		RollbackType:            approval.RollbackAny,
		RollbackDataStrategy:    approval.RollbackDataKeep,
		IsAddAssigneeAllowed:    true,
		IsRemoveAssigneeAllowed: true,
		DuplicateHandlerAction:  approval.DuplicateHandlerAutoPass,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err)

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "end",
		NodeKind:      approval.NodeEnd,
		Name:          "End",
	}
	endNode.ID = id.Generate()
	endNode.CreatedBy = "system"
	endNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(endNode).Exec(ctx)
	require.NoError(t, err)

	// Assignees: user1, user2
	for i, uid := range []string{"user1", "user2"} {
		assignee := &approval.FlowNodeAssignee{
			NodeID:       approvalNode.ID,
			AssigneeKind: approval.AssigneeUser,
			AssigneeIDs:  []string{uid},
			SortOrder:    i,
		}
		assignee.ID = id.Generate()
		_, err = db.NewInsert().Model(assignee).Exec(ctx)
		require.NoError(t, err)
	}

	// Edges
	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	return flow, version, startNode, approvalNode, endNode
}

// buildAutoCompleteFlow creates: Start -> End.
func buildAutoCompleteFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "auto_complete_flow",
		Name:                 "Auto Complete Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	startNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "start",
		NodeKind:      approval.NodeStart,
		Name:          "Start",
	}
	startNode.ID = id.Generate()
	startNode.CreatedBy = "system"
	startNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(startNode).Exec(ctx)
	require.NoError(t, err)

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "end",
		NodeKind:      approval.NodeEnd,
		Name:          "End",
	}
	endNode.ID = id.Generate()
	endNode.CreatedBy = "system"
	endNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(endNode).Exec(ctx)
	require.NoError(t, err)

	insertEdge(t, ctx, db, version.ID, startNode.ID, endNode.ID)

	return flow, version, startNode, endNode
}

// buildHandleFlow creates: Start -> Handle(user1,user2, any-pass) -> End.
func buildHandleFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "handle_flow",
		Name:                 "Handle Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	startNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "start",
		NodeKind:      approval.NodeStart,
		Name:          "Start",
	}
	startNode.ID = id.Generate()
	startNode.CreatedBy = "system"
	startNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(startNode).Exec(ctx)
	require.NoError(t, err)

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
	_, err = db.NewInsert().Model(handleNode).Exec(ctx)
	require.NoError(t, err)

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "end",
		NodeKind:      approval.NodeEnd,
		Name:          "End",
	}
	endNode.ID = id.Generate()
	endNode.CreatedBy = "system"
	endNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(endNode).Exec(ctx)
	require.NoError(t, err)

	insertAssignee(t, ctx, db, handleNode.ID, approval.AssigneeUser, []string{"user1", "user2"}, 0)

	insertEdge(t, ctx, db, version.ID, startNode.ID, handleNode.ID)
	insertEdge(t, ctx, db, version.ID, handleNode.ID, endNode.ID)

	return flow, version, startNode, handleNode, endNode
}

// buildParallelFlow creates: Start -> Approval(3 users parallel, one_reject) -> End.
func buildParallelFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, *approval.FlowNode, *approval.FlowNode, *approval.FlowNode,
) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "parallel_flow",
		Name:                 "Parallel Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	startNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "start",
		NodeKind:      approval.NodeStart,
		Name:          "Start",
	}
	startNode.ID = id.Generate()
	startNode.CreatedBy = "system"
	startNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(startNode).Exec(ctx)
	require.NoError(t, err)

	approvalNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval1",
		NodeKind:               approval.NodeApproval,
		Name:                   "Parallel Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAnyReject,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err)

	endNode := &approval.FlowNode{
		FlowVersionID: version.ID,
		NodeKey:       "end",
		NodeKind:      approval.NodeEnd,
		Name:          "End",
	}
	endNode.ID = id.Generate()
	endNode.CreatedBy = "system"
	endNode.UpdatedBy = "system"
	_, err = db.NewInsert().Model(endNode).Exec(ctx)
	require.NoError(t, err)

	// Single assignee config with 3 users
	assignee := &approval.FlowNodeAssignee{
		NodeID:       approvalNode.ID,
		AssigneeKind: approval.AssigneeUser,
		AssigneeIDs:  []string{"user1", "user2", "user3"},
		SortOrder:    0,
	}
	assignee.ID = id.Generate()
	_, err = db.NewInsert().Model(assignee).Exec(ctx)
	require.NoError(t, err)

	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	return flow, version, startNode, approvalNode, endNode
}

// buildMultiStageFlow creates: Start -> Approval1(user1, sequential) -> Approval2(user2, sequential) -> End.
func buildMultiStageFlow(t *testing.T, ctx context.Context, db orm.DB) (
	*approval.Flow, *approval.FlowVersion, []*approval.FlowNode,
) {
	t.Helper()

	flow := &approval.Flow{
		TenantID:             "default",
		CategoryID:           id.Generate(),
		Code:                 "multi_stage_flow",
		Name:                 "Multi Stage Flow",
		IsActive:             true,
		IsAllInitiateAllowed: true,
		CurrentVersion:       1,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err)

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}
	version.ID = id.Generate()
	version.CreatedBy = "system"
	version.UpdatedBy = "system"
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err)

	startNode := createNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start", approval.ApprovalSequential, approval.PassAll)
	approval1 := createNode(t, ctx, db, version.ID, "approval1", approval.NodeApproval, "First Approval", approval.ApprovalSequential, approval.PassAll)
	approval2 := createNode(t, ctx, db, version.ID, "approval2", approval.NodeApproval, "Second Approval", approval.ApprovalSequential, approval.PassAll)
	endNode := createNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End", approval.ApprovalSequential, approval.PassAll)

	// Assignees
	insertAssignee(t, ctx, db, approval1.ID, approval.AssigneeUser, []string{"user1"}, 0)
	insertAssignee(t, ctx, db, approval2.ID, approval.AssigneeUser, []string{"user2"}, 0)

	// Edges
	insertEdge(t, ctx, db, version.ID, startNode.ID, approval1.ID)
	insertEdge(t, ctx, db, version.ID, approval1.ID, approval2.ID)
	insertEdge(t, ctx, db, version.ID, approval2.ID, endNode.ID)

	nodes := []*approval.FlowNode{startNode, approval1, approval2, endNode}

	return flow, version, nodes
}

// ---- Helpers ----

func createNode(t *testing.T, ctx context.Context, db orm.DB, versionID, key string, kind approval.NodeKind, name string, method approval.ApprovalMethod, rule approval.PassRule) *approval.FlowNode {
	t.Helper()

	node := &approval.FlowNode{
		FlowVersionID:          versionID,
		NodeKey:                key,
		NodeKind:               kind,
		Name:                   name,
		ApprovalMethod:         method,
		PassRule:               rule,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
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

func insertEdge(t *testing.T, ctx context.Context, db orm.DB, versionID, sourceID, targetID string) {
	t.Helper()

	edge := &approval.FlowEdge{
		FlowVersionID: versionID,
		SourceNodeID:  sourceID,
		TargetNodeID:  targetID,
	}
	edge.ID = id.Generate()

	_, err := db.NewInsert().Model(edge).Exec(ctx)
	require.NoError(t, err)
}

// queryTasks loads tasks for an instance from DB.
func queryTasks(t *testing.T, ctx context.Context, db orm.DB, instanceID string) []approval.Task {
	t.Helper()

	var tasks []approval.Task

	err := db.NewSelect().Model(&tasks).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", instanceID)
	}).OrderBy("sort_order").Scan(ctx)
	require.NoError(t, err)

	return tasks
}

// queryTasksByNode loads tasks for a specific node from DB.
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

// queryEvents loads event outbox records from DB.
func queryEvents(t *testing.T, ctx context.Context, db orm.DB) []approval.EventOutbox {
	t.Helper()

	var events []approval.EventOutbox

	err := db.NewSelect().Model(&events).OrderBy("created_at", "id").Scan(ctx)
	require.NoError(t, err)

	return events
}

// queryInstance loads an instance by ID.
func queryInstance(t *testing.T, ctx context.Context, db orm.DB, instanceID string) approval.Instance {
	t.Helper()

	var inst approval.Instance

	err := db.NewSelect().Model(&inst).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", instanceID)
	}).Scan(ctx)
	require.NoError(t, err)

	return inst
}

// minimalFlowDefinition returns a simple Start->End flow definition.
func minimalFlowDefinition() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{{Source: "start", Target: "end"}},
	}
}
