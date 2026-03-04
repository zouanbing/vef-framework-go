package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// buildTestEngine constructs a FlowEngine with all built-in processors and strategies
// suitable for integration tests. AssigneeService is nil since tests insert tasks directly.
func buildTestEngine() *engine.FlowEngine {
	passRules := []approval.PassRuleStrategy{
		strategy.NewAllPassStrategy(),
		strategy.NewOnePassStrategy(),
		strategy.NewRatioPassStrategy(),
		strategy.NewOneRejectStrategy(),
	}

	assigneeResolvers := []strategy.AssigneeResolver{
		strategy.NewUserAssigneeResolver(),
		strategy.NewSelfAssigneeResolver(),
	}

	registry := strategy.NewStrategyRegistry(passRules, assigneeResolvers, nil)

	processors := []engine.NodeProcessor{
		engine.NewStartProcessor(),
		engine.NewEndProcessor(),
		engine.NewConditionProcessor(),
		engine.NewApprovalProcessor(nil),
		engine.NewHandleProcessor(nil),
		engine.NewCCProcessor(),
	}

	return engine.NewFlowEngine(registry, processors, dispatcher.NewEventPublisher())
}

// buildTestServices creates the standard service instances for command tests.
func buildTestServices(eng *engine.FlowEngine) (*service.TaskService, *service.NodeService, *service.ValidationService) {
	taskSvc := service.NewTaskService()
	pub := dispatcher.NewEventPublisher()
	nodeSvc := service.NewNodeService(eng, pub, taskSvc)
	validSvc := service.NewValidationService(nil)
	return taskSvc, nodeSvc, validSvc
}

// FlowFixture holds references to a fully deployed and published flow for testing.
type FlowFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
	NodeIDs    map[string]string // key -> ID mapping
}

// MinimalFixture holds the minimal set of records (category, flow, version)
// needed to satisfy FK constraints when directly inserting instances and tasks.
type MinimalFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
}

// setupMinimalFixture creates the minimum chain of records to satisfy FK constraints:
// category -> flow -> version. Tests can then add nodes referencing VersionID.
func setupMinimalFixture(t testing.TB, ctx context.Context, db orm.DB, code string) *MinimalFixture {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     code + "-cat",
		Name:     code + " Category",
	}
	_, err := db.NewInsert().Model(category).Exec(ctx)
	require.NoError(t, err, "Should insert category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   code,
		Name:                   code + " Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert flow")

	version := &approval.FlowVersion{
		FlowID:  flow.ID,
		Version: 1,
		Status:  approval.VersionDraft,
	}
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should insert version")

	return &MinimalFixture{
		CategoryID: category.ID,
		FlowID:     flow.ID,
		VersionID:  version.ID,
	}
}

// deployAndPublishFlow creates a category, flow, deploys the given definition, and publishes it.
// The code parameter distinguishes different test suites (e.g., "cmd-test", "apv-cmd-test").
func deployAndPublishFlow(t testing.TB, ctx context.Context, db orm.DB, code string, def approval.FlowDefinition) *FlowFixture {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     code + "-cat",
		Name:     code + " Category",
	}
	_, err := db.NewInsert().Model(category).Exec(ctx)
	require.NoError(t, err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   code + "-flow",
		Name:                   code + " Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  code + " {{.InstanceNo}}",
		IsActive:               true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert test flow")

	deployHandler := command.NewDeployFlowHandler(db, service.NewFlowDefinitionService())
	version, err := deployHandler.Handle(ctx, command.DeployFlowCmd{
		FlowID:         flow.ID,
		FlowDefinition: def,
	})
	require.NoError(t, err, "Should deploy flow")

	publishHandler := command.NewPublishVersionHandler(db, dispatcher.NewEventPublisher())
	_, err = publishHandler.Handle(ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin",
	})
	require.NoError(t, err, "Should publish version")

	var nodes []approval.FlowNode
	err = db.NewSelect().Model(&nodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx)
	require.NoError(t, err, "Should load nodes")

	nodeIDs := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.Key] = n.ID
	}

	return &FlowFixture{
		CategoryID: category.ID,
		FlowID:     flow.ID,
		VersionID:  version.ID,
		NodeIDs:    nodeIDs,
	}
}

// setupSimpleFlow creates and publishes a minimal Start -> End flow.
func setupSimpleFlow(t testing.TB, ctx context.Context, db orm.DB) *FlowFixture {
	return deployAndPublishFlow(t, ctx, db, "cmd-test", simpleFlowDef())
}

// setupApprovalFlow creates and publishes a Start -> Approval -> End flow.
func setupApprovalFlow(t testing.TB, ctx context.Context, db orm.DB) *FlowFixture {
	return deployAndPublishFlow(t, ctx, db, "apv-cmd-test", approvalFlowDef())
}

// deleteAll removes all rows from the given models in order (FK-safe).
func deleteAll(ctx context.Context, db orm.DB, models ...any) {
	for _, model := range models {
		_, _ = db.NewDelete().Model(model).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	}
}

// cleanRuntimeData removes runtime data (instances, tasks, logs, etc.) while preserving flow definitions.
// Used in TearDownTest for suites that operate on runtime data.
func cleanRuntimeData(ctx context.Context, db orm.DB) {
	deleteAll(ctx, db,
		(*approval.EventOutbox)(nil),
		(*approval.ActionLog)(nil),
		(*approval.UrgeRecord)(nil),
		(*approval.CCRecord)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
	)
}

// cleanAllApprovalData removes all approval-related data in FK-safe order.
func cleanAllApprovalData(ctx context.Context, db orm.DB) {
	cleanRuntimeData(ctx, db)
	deleteAll(ctx, db,
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
		(*approval.FlowInitiator)(nil),
		(*approval.Flow)(nil),
		(*approval.FlowCategory)(nil),
	)
}

// setupRunningInstance creates a running instance with a pending task for the given assignee.
// It finds the first non-start/non-end node from the fixture to use as the approval node.
func setupRunningInstance(
	t require.TestingT,
	ctx context.Context,
	db orm.DB,
	fixture *FlowFixture,
	assigneeID string,
) (*approval.Instance, *approval.Task) {
	var approvalNodeID string
	for key, id := range fixture.NodeIDs {
		if key != "start-1" && key != "end-1" {
			approvalNodeID = id
			break
		}
	}
	require.NotEmpty(t, approvalNodeID, "Should find approval node ID")

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        fixture.FlowID,
		FlowVersionID: fixture.VersionID,
		Title:         "Test Instance",
		InstanceNo:    "TEST-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &approvalNodeID,
	}
	_, err := db.NewInsert().Model(inst).Exec(ctx)
	require.NoError(t, err, "Should insert instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     approvalNodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = db.NewInsert().Model(task).Exec(ctx)
	require.NoError(t, err, "Should insert task")

	return inst, task
}
