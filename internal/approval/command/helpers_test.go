package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/command"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/engine"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/approval/strategy"
	"github.com/ilxqx/vef-framework-go/orm"
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

// setupSimpleFlow creates a flow category, flow, deploys and publishes a simple flow
// (start -> approval -> end) with PassAll rule and returns the fixture.
func setupSimpleFlow(t testing.TB, ctx context.Context, db orm.DB) *FlowFixture {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "cmd-test",
		Name:     "Command Test Category",
	}
	_, err := db.NewInsert().Model(category).Exec(ctx)
	require.NoError(t, err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "cmd-test-flow",
		Name:                   "Command Test Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test {{.InstanceNo}}",
		IsActive:               true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert test flow")

	deployHandler := command.NewDeployFlowHandler(db, service.NewFlowDefinitionService())
	version, err := deployHandler.Handle(ctx, command.DeployFlowCmd{
		FlowID:         flow.ID,
		FlowDefinition: simpleFlowDef(),
	})
	require.NoError(t, err, "Should deploy flow")

	publishHandler := command.NewPublishVersionHandler(db, dispatcher.NewEventPublisher())
	_, err = publishHandler.Handle(ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin",
	})
	require.NoError(t, err, "Should publish version")

	// Load node IDs
	var nodes []approval.FlowNode
	err = db.NewSelect().Model(&nodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", version.ID) }).
		Scan(ctx)
	require.NoError(t, err, "Should load nodes")

	nodeIDs := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.Key] = n.ID
	}

	// Reload flow to get updated CurrentVersion
	_ = db.NewSelect().Model(flow).WherePK().Scan(ctx)

	return &FlowFixture{
		CategoryID: category.ID,
		FlowID:     flow.ID,
		VersionID:  version.ID,
		NodeIDs:    nodeIDs,
	}
}

// setupApprovalFlow creates a flow with an approval node using approvalFlowDef() and publishes it.
func setupApprovalFlow(t testing.TB, ctx context.Context, db orm.DB) *FlowFixture {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "apv-cmd-test",
		Name:     "Approval Command Test Category",
	}
	_, err := db.NewInsert().Model(category).Exec(ctx)
	require.NoError(t, err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "apv-cmd-test-flow",
		Name:                   "Approval Command Test Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Approval Test {{.InstanceNo}}",
		IsActive:               true,
	}
	_, err = db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert test flow")

	deployHandler := command.NewDeployFlowHandler(db, service.NewFlowDefinitionService())
	version, err := deployHandler.Handle(ctx, command.DeployFlowCmd{
		FlowID:         flow.ID,
		FlowDefinition: approvalFlowDef(),
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

// cleanAllApprovalData removes all approval-related data from the database.
func cleanAllApprovalData(ctx context.Context, db orm.DB) {
	_, _ = db.NewDelete().Model((*approval.EventOutbox)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.ActionLog)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.UrgeRecord)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.CCRecord)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowEdge)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNodeCC)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNodeAssignee)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowNode)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowVersion)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowInitiator)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.Flow)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
	_, _ = db.NewDelete().Model((*approval.FlowCategory)(nil)).Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).Exec(ctx)
}
