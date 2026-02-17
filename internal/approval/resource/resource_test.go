package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/api"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
)

// TestNewFlowResource tests new flow resource scenarios.
func TestNewFlowResource(t *testing.T) {
	svc := new(service.FlowService)
	r := NewFlowResource(svc)

	t.Run("ShouldSetResourceName", func(t *testing.T) {
		assert.Equal(t, "approval/flow", r.Name(), "Should have resource name 'approval/flow'")
	})

	t.Run("ShouldSetResourceKind", func(t *testing.T) {
		assert.Equal(t, api.KindRPC, r.Kind(), "Should be an RPC resource")
	})

	t.Run("ShouldStoreFlowService", func(t *testing.T) {
		assert.Same(t, svc, r.flowService, "Should store the injected FlowService")
	})

	t.Run("ShouldHaveThreeOperations", func(t *testing.T) {
		ops := r.Operations()
		require.Len(t, ops, 3, "Should have exactly 3 operations")

		actions := make([]string, len(ops))
		for i, op := range ops {
			actions[i] = op.Action
		}
		assert.Equal(t, []string{"deploy", "publish_version", "get_graph"}, actions,
			"Should have deploy, publish_version, get_graph actions in order")
	})
}

// TestNewInstanceResource tests new instance resource scenarios.
func TestNewInstanceResource(t *testing.T) {
	instanceSvc := new(service.InstanceService)
	querySvc := new(service.QueryService)
	r := NewInstanceResource(instanceSvc, querySvc)

	t.Run("ShouldSetResourceName", func(t *testing.T) {
		assert.Equal(t, "approval/instance", r.Name(), "Should have resource name 'approval/instance'")
	})

	t.Run("ShouldSetResourceKind", func(t *testing.T) {
		assert.Equal(t, api.KindRPC, r.Kind(), "Should be an RPC resource")
	})

	t.Run("ShouldStoreServices", func(t *testing.T) {
		assert.Same(t, instanceSvc, r.instanceService, "Should store the injected InstanceService")
		assert.Same(t, querySvc, r.queryService, "Should store the injected QueryService")
	})

	t.Run("ShouldHaveElevenOperations", func(t *testing.T) {
		ops := r.Operations()
		require.Len(t, ops, 11, "Should have exactly 11 operations")

		expectedActions := []string{
			"start",
			"process_task",
			"withdraw",
			"add_cc",
			"add_assignee",
			"remove_assignee",
			"find_instances",
			"find_tasks",
			"get_detail",
			"get_action_logs",
			"urge_task",
		}
		actions := make([]string, len(ops))
		for i, op := range ops {
			actions[i] = op.Action
		}
		assert.Equal(t, expectedActions, actions,
			"Should have all 11 expected actions in order")
	})
}

// TestNewCategoryResource tests new category resource scenarios.
func TestNewCategoryResource(t *testing.T) {
	r := NewCategoryResource()

	t.Run("ShouldSetResourceName", func(t *testing.T) {
		assert.Equal(t, "approval/category", r.Name(), "Should have resource name 'approval/category'")
	})

	t.Run("ShouldSetResourceKind", func(t *testing.T) {
		assert.Equal(t, api.KindRPC, r.Kind(), "Should be an RPC resource")
	})

	t.Run("ShouldHaveNoCustomOperations", func(t *testing.T) {
		ops := r.Operations()
		assert.Empty(t, ops, "Should have no custom operations")
	})
}

// TestNewDelegationResource tests new delegation resource scenarios.
func TestNewDelegationResource(t *testing.T) {
	r := NewDelegationResource()

	t.Run("ShouldSetResourceName", func(t *testing.T) {
		assert.Equal(t, "approval/delegation", r.Name(), "Should have resource name 'approval/delegation'")
	})

	t.Run("ShouldSetResourceKind", func(t *testing.T) {
		assert.Equal(t, api.KindRPC, r.Kind(), "Should be an RPC resource")
	})

	t.Run("ShouldHaveNoCustomOperations", func(t *testing.T) {
		ops := r.Operations()
		assert.Empty(t, ops, "Should have no custom operations")
	})
}
