package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// QueryFixture holds the minimal set of records (category, flow, version)
// needed to satisfy FK constraints when directly inserting instances and tasks.
type QueryFixture struct {
	CategoryID string
	FlowID     string
	VersionID  string
	NodeIDs    []string
}

// setupQueryFixture creates a category → flow → version → nodes chain.
// nodeCount specifies how many approval nodes to create.
func setupQueryFixture(t testing.TB, ctx context.Context, db orm.DB, code string, nodeCount int) *QueryFixture {
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
		Status:  approval.VersionPublished,
	}
	_, err = db.NewInsert().Model(version).Exec(ctx)
	require.NoError(t, err, "Should insert version")

	var nodeIDs []string
	for i := range nodeCount {
		node := &approval.FlowNode{
			FlowVersionID: version.ID,
			Key:           code + "-node-" + string(rune('a'+i)),
			Kind:          approval.NodeApproval,
			Name:          code + " Node",
		}
		_, err = db.NewInsert().Model(node).Exec(ctx)
		require.NoError(t, err, "Should insert node")
		nodeIDs = append(nodeIDs, node.ID)
	}

	return &QueryFixture{
		CategoryID: category.ID,
		FlowID:     flow.ID,
		VersionID:  version.ID,
		NodeIDs:    nodeIDs,
	}
}

// cleanAllQueryData removes all approval data in FK-safe order.
func cleanAllQueryData(ctx context.Context, db orm.DB) {
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
