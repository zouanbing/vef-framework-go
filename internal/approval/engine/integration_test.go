package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
)

// TestStartProcessAutoCompleteFlow tests start process auto complete flow scenarios.
func TestStartProcessAutoCompleteFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version, _, _ := buildAutoCompleteFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	err := eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed for auto-complete flow")

	inst := queryInstance(t, ctx, db, instance.ID)
	assert.Equal(t, approval.InstanceApproved, inst.Status, "Instance should be approved after Start->End")
	assert.True(t, inst.FinishedAt.Valid, "FinishedAt should be set")
}

// TestStartProcessSimpleApprovalFlow tests start process simple approval flow scenarios.
func TestStartProcessSimpleApprovalFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", map[string]any{"title": "test"})

	err := eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed for simple approval flow")

	inst := queryInstance(t, ctx, db, instance.ID)
	assert.Equal(t, approval.InstanceRunning, inst.Status, "Instance should be running while waiting for approval")
	assert.Equal(t, approvalNode.ID, inst.CurrentNodeID.String, "CurrentNodeID should point to approval node")

	tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
	require.Len(t, tasks, 2, "Should create 2 tasks for sequential approval")
	assert.Equal(t, approval.TaskPending, tasks[0].Status, "First task should be pending")
	assert.Equal(t, 1, tasks[0].SortOrder, "First task sort order should be 1")
	assert.Equal(t, approval.TaskWaiting, tasks[1].Status, "Second task should be waiting")
	assert.Equal(t, 2, tasks[1].SortOrder, "Second task sort order should be 2")

	snapshots := queryFormSnapshots(t, ctx, db, instance.ID)
	assert.Len(t, snapshots, 1, "Should create one form snapshot for the approval node")
}

// TestStartProcessHandleFlow tests start process handle flow scenarios.
func TestStartProcessHandleFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version, _, handleNode, _ := buildHandleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	err := eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed for handle flow")

	inst := queryInstance(t, ctx, db, instance.ID)
	assert.Equal(t, approval.InstanceRunning, inst.Status, "Instance should be running")
	assert.Equal(t, handleNode.ID, inst.CurrentNodeID.String, "CurrentNodeID should point to handle node")

	tasks := queryTasksByNode(t, ctx, db, instance.ID, handleNode.ID)
	require.Len(t, tasks, 2, "Should create 2 tasks for handle node")
	for _, task := range tasks {
		assert.Equal(t, 0, task.SortOrder, "Handle tasks should all have sortOrder=0")
		assert.Equal(t, approval.TaskPending, task.Status, "Handle tasks should be pending")
	}
}

// TestStartProcessBranchFlow tests start process branch flow scenarios.
func TestStartProcessBranchFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	t.Run("HighValueConditionMet", func(t *testing.T) {
		flow, version, _, _, approval1, _, _ := buildBranchFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", map[string]any{"amount": float64(2000)})

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for branch flow with high value")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceRunning, inst.Status, "Instance should be running at high-value approval")
		assert.Equal(t, approval1.ID, inst.CurrentNodeID.String, "Should route to high-value approval node")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approval1.ID)
		require.Len(t, tasks, 1, "Should create 1 task for high-value approval")
		assert.Equal(t, "manager1", tasks[0].AssigneeID, "Task should be assigned to manager1")
	})

	t.Run("LowValueDefaultCondition", func(t *testing.T) {
		flow, version, _, _, _, approval2, _ := buildBranchFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", map[string]any{"amount": float64(500)})

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for branch flow with low value")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval2.ID, inst.CurrentNodeID.String, "Should route to low-value approval node")
	})
}

// TestProcessNodeProcessorNotFound tests process node processor not found scenarios.
func TestProcessNodeProcessorNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := NewFlowEngine(nil, nil, nil)

	node := &approval.FlowNode{NodeKind: approval.NodeApproval}
	node.ID = id.Generate()
	instance := &approval.Instance{ApplicantID: "u1"}

	err := eng.ProcessNode(ctx, db, instance, node)
	require.Error(t, err, "Should return error for unknown processor")
	assert.ErrorIs(t, err, ErrProcessorNotFound, "Error should be ErrProcessorNotFound")
}

// TestAdvanceToNextNode tests advance to next node scenarios.
func TestAdvanceToNextNode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	t.Run("AdvancesFromStartToApproval", func(t *testing.T) {
		flow, version, startNode, approvalNode, _ := buildSimpleFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.AdvanceToNextNode(ctx, db, instance, startNode, "")
		require.NoError(t, err, "Should advance from start to approval")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approvalNode.ID, inst.CurrentNodeID.String, "Should be at approval node after advance")
	})

	t.Run("ErrorWhenNoEdge", func(t *testing.T) {
		flow, version := createFlowAndVersion(t, ctx, db, "orphan_flow", "Orphan Flow")
		node := createFlowNode(t, ctx, db, version.ID, "orphan", approval.NodeStart, "Orphan")
		instance := createInstance(t, ctx, db, flow, version, "u1", nil)

		err := eng.AdvanceToNextNode(ctx, db, instance, node, "")
		require.Error(t, err, "Should return error when no edge matches")
		assert.ErrorIs(t, err, ErrNoMatchingEdge, "Error should be ErrNoMatchingEdge")
	})
}

// TestEvaluateNodeCompletion tests evaluate node completion scenarios.
func TestEvaluateNodeCompletion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	for _, uid := range []string{"user1", "user2"} {
		task := &approval.Task{
			InstanceID: instance.ID,
			NodeID:     approvalNode.ID,
			AssigneeID: uid,
			Status:     approval.TaskPending,
		}
		task.ID = id.Generate()
		task.CreatedBy = "system"
		task.UpdatedBy = "system"
		_, err := db.NewInsert().Model(task).Exec(ctx)
		require.NoError(t, err, "Should insert task for %s", uid)
	}

	t.Run("AllPending", func(t *testing.T) {
		result, err := eng.EvaluateNodeCompletion(ctx, db, instance, approvalNode)
		require.NoError(t, err, "Should evaluate node completion without error")
		assert.Equal(t, approval.PassRulePending, result, "Should be pending when all tasks are pending")
	})

	tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
	tasks[0].Status = approval.TaskApproved
	_, err := db.NewUpdate().Model(&tasks[0]).WherePK().Exec(ctx)
	require.NoError(t, err, "Should update first task to approved")

	t.Run("OneApprovedOnePending", func(t *testing.T) {
		result, err := eng.EvaluateNodeCompletion(ctx, db, instance, approvalNode)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePending, result, "Should be pending with one approved and one pending")
	})

	tasks[1].Status = approval.TaskApproved
	_, err = db.NewUpdate().Model(&tasks[1]).WherePK().Exec(ctx)
	require.NoError(t, err, "Should update second task to approved")

	t.Run("AllApproved", func(t *testing.T) {
		result, err := eng.EvaluateNodeCompletion(ctx, db, instance, approvalNode)
		require.NoError(t, err, "Should evaluate without error")
		assert.Equal(t, approval.PassRulePassed, result, "Should pass when all tasks are approved")
	})
}

// TestPredictNextNode tests predict next node scenarios.
func TestPredictNextNode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	t.Run("PredictsApprovalNode", func(t *testing.T) {
		flow, version, startNode, approvalNode, _ := buildSimpleFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed")
		assert.Equal(t, approvalNode.ID, nextNode.ID, "Should predict approval node as next")
		assert.Equal(t, []string{"user1", "user2"}, assigneeIDs, "Should predict user1 and user2 as assignees")
	})

	t.Run("NoEdgeFromEndNode", func(t *testing.T) {
		flow, version, _, _, endNode := buildSimpleFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		_, _, err := eng.PredictNextNode(ctx, db, instance, endNode)
		assert.ErrorIs(t, err, ErrNoMatchingEdge, "Should fail when no edge from end node")
	})

	t.Run("PredictsHandleNode", func(t *testing.T) {
		flow, version, startNode, handleNode, _ := buildHandleFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed")
		assert.Equal(t, handleNode.ID, nextNode.ID, "Should predict handle node as next")
		assert.Contains(t, assigneeIDs, "user1", "Should include user1")
		assert.Contains(t, assigneeIDs, "user2", "Should include user2")
	})

	t.Run("PredictsEndNodeWithNilAssignees", func(t *testing.T) {
		flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, approvalNode)
		require.NoError(t, err, "PredictNextNode should succeed for end node")
		assert.Equal(t, approval.NodeEnd, nextNode.NodeKind, "Should predict end node")
		assert.Nil(t, assigneeIDs, "End node should have no assignees")
	})

	t.Run("SameApplicantAutoPass", func(t *testing.T) {
		flow, version, startNode, _, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantAutoPass, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed for same applicant auto_pass")
		assert.Equal(t, approval.NodeApproval, nextNode.NodeKind, "Should predict approval node")
		assert.Nil(t, assigneeIDs, "auto_pass same applicant should predict nil assignees")
	})
}

// TestApprovalProcessorEmptyAssignee consolidates all empty-assignee fallback scenarios.
func TestApprovalProcessorEmptyAssignee(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("AutoPass", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_autopass", approval.NodeApproval, approval.EmptyHandlerAutoPass)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with empty assignee auto_pass")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceApproved, inst.Status, "Instance should auto-complete when empty assignee auto_pass")
	})

	t.Run("TransferAdmin", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_admin", approval.NodeApproval, approval.EmptyHandlerTransferAdmin, func(n *approval.FlowNode) {
			n.AdminUserIDs = []string{"admin1", "admin2"}
		})
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with empty assignee transfer_admin")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceRunning, inst.Status, "Instance should be running")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 2, "Should create tasks for admin users")
		assert.Equal(t, "admin1", tasks[0].AssigneeID, "First task should be assigned to admin1")
		assert.Equal(t, "admin2", tasks[1].AssigneeID, "Second task should be assigned to admin2")
	})

	t.Run("TransferApplicant", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_applicant", approval.NodeApproval, approval.EmptyHandlerTransferApplicant)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with empty assignee transfer_applicant")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 1, "Should create 1 task for applicant")
		assert.Equal(t, "applicant1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})

	t.Run("TransferSpecified", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_specified", approval.NodeApproval, approval.EmptyHandlerTransferSpecified, func(n *approval.FlowNode) {
			n.FallbackUserIDs = []string{"fallback1"}
		})
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with empty assignee transfer_specified")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 1, "Should create 1 task for specified fallback user")
		assert.Equal(t, "fallback1", tasks[0].AssigneeID, "Task should be assigned to fallback user")
	})

	t.Run("TransferSuperior", func(t *testing.T) {
		mockOrg := &MockOrganizationService{
			superiors: map[string]struct{ id, name string }{
				"applicant1": {id: "superior1", name: "Superior"},
			},
		}
		eng := setupEngine(mockOrg, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_superior", approval.NodeApproval, approval.EmptyHandlerTransferSuperior)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with empty assignee transfer_superior")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 1, "Should create 1 task for superior")
		assert.Equal(t, "superior1", tasks[0].AssigneeID, "Task should be assigned to superior")
	})

	t.Run("TransferSuperiorOrgServiceError", func(t *testing.T) {
		mockOrg := &MockOrganizationService{err: errors.New("org service error")}
		eng := setupEngine(mockOrg, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_sup_err", approval.NodeApproval, approval.EmptyHandlerTransferSuperior)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.Error(t, err, "Should return error when orgService fails")
		assert.Contains(t, err.Error(), "org service error", "Error should contain org service message")
	})

	t.Run("DefaultErrorForUnknownAction", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "approval_empty_unknown", approval.NodeApproval, "unknown_action")
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee for unknown empty handler action")
	})

	t.Run("PredictWithTransferApplicant", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "approval_predict_empty", approval.NodeApproval, approval.EmptyHandlerTransferApplicant)

		startNode := createFlowNode(t, ctx, db, version.ID, "predict_start", approval.NodeStart, "Start")
		insertEdge(t, ctx, db, version.ID, startNode.ID, node.ID)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed for empty assignee")
		assert.Equal(t, node.ID, nextNode.ID, "Should predict approval node")
		assert.Equal(t, []string{"applicant1"}, assigneeIDs, "Should return applicant as fallback assignee")
	})
}

// TestApprovalProcessorSameApplicant tests same applicant handling.
func TestApprovalProcessorSameApplicant(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("AutoPass", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, approvalNode, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantAutoPass, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should auto-pass when same applicant")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceApproved, inst.Status, "Instance should be approved (auto-pass)")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		assert.Len(t, tasks, 0, "Should not create tasks for auto-pass")
	})

	t.Run("SelfApprove", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, approvalNode, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantSelfApprove, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for self-approve")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceRunning, inst.Status, "Instance should be running for self-approve")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 1, "Should create 1 task for self-approve")
		assert.Equal(t, "user1", tasks[0].AssigneeID, "Task should be assigned to the applicant")
	})

	t.Run("TransferAdmin", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, approvalNode, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantTransferAdmin, []string{"admin_user"})
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for transfer admin")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 1, "Should create 1 task for admin")
		assert.Equal(t, "admin_user", tasks[0].AssigneeID, "Task should be assigned to admin")
	})

	t.Run("TransferAdminEmptyAdmins", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, _, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantTransferAdmin, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee when admin list is empty")
	})

	t.Run("TransferSuperior", func(t *testing.T) {
		mockOrg := &MockOrganizationService{
			superiors: map[string]struct{ id, name string }{
				"user1": {id: "boss1", name: "Boss"},
			},
		}
		eng := setupEngine(mockOrg, nil)
		flow, version, _, approvalNode, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantTransferSuperior, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for transfer superior")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 1, "Should create 1 task for superior")
		assert.Equal(t, "boss1", tasks[0].AssigneeID, "Task should be assigned to superior")
	})

	t.Run("TransferSuperiorNoSuperior", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, _, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantTransferSuperior, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee when no superior found")
	})

	t.Run("TransferSuperiorOrgServiceError", func(t *testing.T) {
		mockOrg := &MockOrganizationService{err: errors.New("org service unavailable")}
		eng := setupEngine(mockOrg, nil)
		flow, version, _, _, _ := buildFlowWithSameApplicant(t, ctx, db, approval.SameApplicantTransferSuperior, nil)
		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.Error(t, err, "Should return error when orgService fails")
		assert.Contains(t, err.Error(), "org service unavailable", "Error should contain org service message")
	})

	t.Run("DefaultActionForUnknownType", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version := createFlowAndVersion(t, ctx, db, "same_default_"+id.Generate(), "Same Default")
		startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

		approvalNode := &approval.FlowNode{
			FlowVersionID:          version.ID,
			NodeKey:                "approval_same_default",
			NodeKind:               approval.NodeApproval,
			Name:                   "Default Same Applicant",
			ApprovalMethod:         approval.ApprovalParallel,
			PassRule:               approval.PassAll,
			PassRatio:              decimal.Zero,
			SameApplicantAction:    "some_unknown_action",
			DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
		}
		approvalNode.ID = id.Generate()
		approvalNode.CreatedBy = "system"
		approvalNode.UpdatedBy = "system"
		_, err := db.NewInsert().Model(approvalNode).Exec(ctx)
		require.NoError(t, err, "Should insert approval node")

		endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

		insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{"user1"}, 0)
		insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
		insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

		instance := createInstance(t, ctx, db, flow, version, "user1", nil)

		err = eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "Default same applicant action should create tasks")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 1, "Default same applicant should create task for the assignee")
		assert.Equal(t, "user1", tasks[0].AssigneeID, "Task should be assigned to user1")
	})
}

// TestHandleProcessorEmptyAssignee consolidates all handle-processor empty-assignee scenarios.
func TestHandleProcessorEmptyAssignee(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("AutoPass", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "handle_empty_autopass", approval.NodeHandle, approval.EmptyHandlerAutoPass)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for handle empty auto_pass")

		inst := queryInstance(t, ctx, db, instance.ID)
		assert.Equal(t, approval.InstanceApproved, inst.Status, "Instance should auto-complete")
	})

	t.Run("TransferAdmin", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "handle_empty_admin", approval.NodeHandle, approval.EmptyHandlerTransferAdmin, func(n *approval.FlowNode) {
			n.AdminUserIDs = []string{"admin1"}
		})
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with handle empty assignee transfer_admin")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 1, "Should create 1 task for admin")
		assert.Equal(t, "admin1", tasks[0].AssigneeID, "Task should be assigned to admin")
	})

	t.Run("TransferSuperior", func(t *testing.T) {
		mockOrg := &MockOrganizationService{
			superiors: map[string]struct{ id, name string }{
				"applicant1": {id: "superior1", name: "Superior"},
			},
		}
		eng := setupEngine(mockOrg, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "handle_empty_superior", approval.NodeHandle, approval.EmptyHandlerTransferSuperior)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "Should succeed with handle empty assignee transfer to superior")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, node.ID)
		require.Len(t, tasks, 1, "Should create 1 task for superior")
		assert.Equal(t, "superior1", tasks[0].AssigneeID, "Task should be assigned to superior")
	})

	t.Run("TransferSuperiorNoSuperior", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "handle_empty_no_sup", approval.NodeHandle, approval.EmptyHandlerTransferSuperior)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee when no superior found for handle node")
	})

	t.Run("DefaultErrorForUnknownAction", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _ := buildEmptyAssigneeFlow(t, ctx, db, "handle_empty_unknown", approval.NodeHandle, "unknown_action")
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee for unknown empty handler action on handle node")
	})

	t.Run("PredictAutoPassReturnsNilAssignees", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "handle_predict_empty", approval.NodeHandle, approval.EmptyHandlerAutoPass)

		startNode := createFlowNode(t, ctx, db, version.ID, "predict_start", approval.NodeStart, "Start")
		insertEdge(t, ctx, db, version.ID, startNode.ID, node.ID)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed for empty assignee handle node")
		assert.Equal(t, node.ID, nextNode.ID, "Should predict handle node")
		assert.Nil(t, assigneeIDs, "Should return nil assignees for auto_pass empty handler")
	})

	t.Run("PredictUnknownActionReturnsNilAssignees", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, node := buildEmptyAssigneeFlow(t, ctx, db, "handle_predict_err", approval.NodeHandle, "unknown_action")

		startNode := createFlowNode(t, ctx, db, version.ID, "predict_start2", approval.NodeStart, "Start")
		insertEdge(t, ctx, db, version.ID, startNode.ID, node.ID)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
		require.NoError(t, err, "PredictNextNode should succeed even when predict returns error internally")
		assert.Equal(t, node.ID, nextNode.ID, "Should predict handle node")
		assert.Nil(t, assigneeIDs, "Should return nil when prediction fails internally")
	})
}

// TestApprovalProcessorParallelApproval tests approval processor parallel approval scenarios.
func TestApprovalProcessorParallelApproval(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version := createFlowAndVersion(t, ctx, db, "parallel_flow", "Parallel Flow")
	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	approvalNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval_parallel",
		NodeKind:               approval.NodeApproval,
		Name:                   "Parallel Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err, "Should insert approval node")

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{"user1", "user2", "user3"}, 0)
	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	err = eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed for parallel approval")

	tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
	require.Len(t, tasks, 3, "Should create 3 tasks for parallel approval")

	for _, task := range tasks {
		assert.Equal(t, 0, task.SortOrder, "Parallel tasks should all have sortOrder=0")
		assert.Equal(t, approval.TaskPending, task.Status, "Parallel tasks should all be pending")
	}
}

// TestApprovalProcessorDuplicateHandlerNone tests approval processor duplicate handler none scenarios.
func TestApprovalProcessorDuplicateHandlerNone(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version := createFlowAndVersion(t, ctx, db, "dup_none_flow", "Dup None")
	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	approvalNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "approval_dup_none",
		NodeKind:               approval.NodeApproval,
		Name:                   "Dup None Approval",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAll,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerNone,
	}
	approvalNode.ID = id.Generate()
	approvalNode.CreatedBy = "system"
	approvalNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(approvalNode).Exec(ctx)
	require.NoError(t, err, "Should insert approval node")

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{"user1"}, 0)
	insertAssignee(t, ctx, db, approvalNode.ID, approval.AssigneeUser, []string{"user1"}, 1)

	insertEdge(t, ctx, db, version.ID, startNode.ID, approvalNode.ID)
	insertEdge(t, ctx, db, version.ID, approvalNode.ID, endNode.ID)

	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	err = eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed for duplicate handler none")

	tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
	assert.Len(t, tasks, 2, "DuplicateHandlerNone should keep duplicate assignees")
}

// TestLoadFlowCategoryID tests load flow category ID scenarios.
func TestLoadFlowCategoryID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("ExistingFlow", func(t *testing.T) {
		flow := &approval.Flow{
			TenantID:   "default",
			CategoryID: "cat_123",
			Code:       "test_flow_cat",
			Name:       "Test",
			IsActive:   true,
		}
		flow.ID = id.Generate()
		flow.CreatedBy = "system"
		flow.UpdatedBy = "system"
		_, err := db.NewInsert().Model(flow).Exec(ctx)
		require.NoError(t, err, "Should insert flow")

		catID := loadFlowCategoryID(ctx, db, flow.ID)
		assert.Equal(t, "cat_123", catID, "Should return the flow's category ID")
	})

	t.Run("NonExistingFlow", func(t *testing.T) {
		catID := loadFlowCategoryID(ctx, db, "non_existing_flow_id")
		assert.Empty(t, catID, "Should return empty for non-existing flow")
	})
}

// TestResolveDelegationChain tests resolve delegation chain scenarios.
func TestResolveDelegationChain(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("NoDelegation", func(t *testing.T) {
		finalID, originalID := resolveDelegationChain(ctx, db, "user1", "flow1", "cat1")
		assert.Equal(t, "user1", finalID, "Should return original user when no delegation")
		assert.Empty(t, originalID, "OriginalID should be empty when no delegation")
	})

	t.Run("SingleDelegation", func(t *testing.T) {
		insertDelegation(t, ctx, db, "delegator1", "delegatee1", null.String{}, true)

		finalID, originalID := resolveDelegationChain(ctx, db, "delegator1", "flow_x", "cat_x")
		assert.Equal(t, "delegatee1", finalID, "Should resolve to delegatee")
		assert.Equal(t, "delegator1", originalID, "OriginalID should be the original user")
	})

	t.Run("ChainDelegation", func(t *testing.T) {
		insertDelegation(t, ctx, db, "chain_a", "chain_b", null.String{}, true)
		insertDelegation(t, ctx, db, "chain_b", "chain_c", null.String{}, true)

		finalID, originalID := resolveDelegationChain(ctx, db, "chain_a", "flow_y", "cat_y")
		assert.Equal(t, "chain_c", finalID, "Should resolve to end of chain")
		assert.Equal(t, "chain_a", originalID, "OriginalID should be the starting user")
	})

	t.Run("CycleDetection", func(t *testing.T) {
		insertDelegation(t, ctx, db, "cycle_a", "cycle_b", null.String{}, true)
		insertDelegation(t, ctx, db, "cycle_b", "cycle_a", null.String{}, true)

		finalID, _ := resolveDelegationChain(ctx, db, "cycle_a", "flow_z", "cat_z")
		assert.Equal(t, "cycle_b", finalID, "Should stop at cycle and return last resolved")
	})
}

// TestApplyDelegation tests apply delegation scenarios.
func TestApplyDelegation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	flow := &approval.Flow{
		TenantID:   "default",
		CategoryID: "cat_deleg",
		Code:       "deleg_flow",
		Name:       "Delegation Flow",
		IsActive:   true,
	}
	flow.ID = id.Generate()
	flow.CreatedBy = "system"
	flow.UpdatedBy = "system"
	_, err := db.NewInsert().Model(flow).Exec(ctx)
	require.NoError(t, err, "Should insert flow")

	insertDelegation(t, ctx, db, "orig_user", "deleg_user", null.StringFrom(flow.ID), true)

	assignees := []approval.ResolvedAssignee{
		{UserID: "orig_user"},
		{UserID: "other_user"},
	}

	result := applyDelegation(ctx, db, flow.ID, assignees)
	require.Len(t, result, 2, "Should return same number of assignees")

	assert.Equal(t, "deleg_user", result[0].UserID, "Delegated user should replace original")
	assert.Equal(t, "orig_user", result[0].DelegateFromID, "DelegateFromID should point to original")

	assert.Equal(t, "other_user", result[1].UserID, "Non-delegated user should remain")
	assert.Empty(t, result[1].DelegateFromID, "Non-delegated user should have empty DelegateFromID")
}

// TestCreateTasksForUsers tests create tasks for users scenarios.
func TestCreateTasksForUsers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	pc := &ProcessContext{
		DB:       db,
		Instance: instance,
		Node:     approvalNode,
	}

	t.Run("EmptyUserIDs", func(t *testing.T) {
		_, err := createTasksForUsers(ctx, pc, nil)
		assert.ErrorIs(t, err, ErrNoAssignee, "Should return ErrNoAssignee for empty user IDs")
	})

	t.Run("ValidUserIDs", func(t *testing.T) {
		result, err := createTasksForUsers(ctx, pc, []string{"u1", "u2"})
		require.NoError(t, err, "Should create tasks successfully")
		assert.Equal(t, NodeActionWait, result.Action, "Should return Wait action")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		assert.Len(t, tasks, 2, "Should create 2 tasks")
	})
}

// TestSaveFormSnapshot tests save form snapshot scenarios.
func TestSaveFormSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", map[string]any{"field1": "value1"})

	pc := &ProcessContext{
		DB:       db,
		Instance: instance,
		Node:     approvalNode,
		FormData: approval.NewFormData(map[string]any{"field1": "value1"}),
	}

	err := saveFormSnapshot(ctx, pc)
	require.NoError(t, err, "Should save form snapshot successfully")

	snapshots := queryFormSnapshots(t, ctx, db, instance.ID)
	require.Len(t, snapshots, 1, "Should have 1 form snapshot")
	assert.Equal(t, approvalNode.ID, snapshots[0].NodeID, "Snapshot should reference the correct node")
	assert.Equal(t, "value1", snapshots[0].FormData["field1"], "Snapshot should contain correct form data")
}

// TestCreateTasksWithDelegation tests create tasks with delegation scenarios.
func TestCreateTasksWithDelegation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	pc := &ProcessContext{
		DB:       db,
		Instance: instance,
		Node:     approvalNode,
	}

	assignees := []approval.ResolvedAssignee{
		{UserID: "delegate1", DelegateFromID: "original1"},
		{UserID: "normal_user"},
	}

	err := createTasksWithDelegation(ctx, pc, assignees)
	require.NoError(t, err, "Should create tasks with delegation")

	tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
	require.Len(t, tasks, 2, "Should create 2 tasks")

	var delegatedTask, normalTask *approval.Task
	for i := range tasks {
		if tasks[i].DelegateFromID.Valid {
			delegatedTask = &tasks[i]
		} else {
			normalTask = &tasks[i]
		}
	}

	require.NotNil(t, delegatedTask, "Should have a delegated task")
	assert.Equal(t, "delegate1", delegatedTask.AssigneeID, "Delegated task should be assigned to delegate")
	assert.Equal(t, "original1", delegatedTask.DelegateFromID.String, "DelegateFromID should reference original user")

	require.NotNil(t, normalTask, "Should have a normal task")
	assert.Equal(t, "normal_user", normalTask.AssigneeID, "Normal task should be assigned to normal_user")
	assert.False(t, normalTask.DelegateFromID.Valid, "Normal task should not have delegation")
}

// TestDelegation tests delegation scenarios.
func TestDelegation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("ApprovalWithDelegation", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
		insertDelegation(t, ctx, db, "user1", "delegatee1", null.StringFrom(flow.ID), true)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed with delegation")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 2, "Should create 2 tasks")

		var delegatedTask *approval.Task
		for i := range tasks {
			if tasks[i].DelegateFromID.Valid {
				delegatedTask = &tasks[i]
			}
		}

		require.NotNil(t, delegatedTask, "Should have a delegated task")
		assert.Equal(t, "delegatee1", delegatedTask.AssigneeID, "Delegated task should be assigned to delegatee")
		assert.Equal(t, "user1", delegatedTask.DelegateFromID.String, "DelegateFromID should point to original user")
	})

	t.Run("SequentialWithDelegation", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version, _, approvalNode, _ := buildSimpleFlow(t, ctx, db)
		insertDelegation(t, ctx, db, "user1", "delegatee_seq", null.StringFrom(flow.ID), true)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err := eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "Should start process with sequential delegation")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, approvalNode.ID)
		require.Len(t, tasks, 2, "Should create 2 sequential tasks")

		assert.Equal(t, "delegatee_seq", tasks[0].AssigneeID, "First task should be assigned to delegate")
		assert.Equal(t, "user1", tasks[0].DelegateFromID.String, "First task DelegateFromID should reference user1")
		assert.Equal(t, 1, tasks[0].SortOrder, "First task sort order should be 1")
		assert.Equal(t, approval.TaskPending, tasks[0].Status, "First task should be pending")

		assert.Equal(t, "user2", tasks[1].AssigneeID, "Second task should remain as user2")
		assert.Equal(t, 2, tasks[1].SortOrder, "Second task sort order should be 2")
		assert.Equal(t, approval.TaskWaiting, tasks[1].Status, "Second task should be waiting")
	})

	t.Run("HandleWithDelegation", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		flow, version := createFlowAndVersion(t, ctx, db, "handle_deleg_flow", "Handle Delegation")
		startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

		handleNode := &approval.FlowNode{
			FlowVersionID:          version.ID,
			NodeKey:                "handle_deleg",
			NodeKind:               approval.NodeHandle,
			Name:                   "Delegated Handle",
			ApprovalMethod:         approval.ApprovalParallel,
			PassRule:               approval.PassAny,
			PassRatio:              decimal.Zero,
			DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
		}
		handleNode.ID = id.Generate()
		handleNode.CreatedBy = "system"
		handleNode.UpdatedBy = "system"
		_, err := db.NewInsert().Model(handleNode).Exec(ctx)
		require.NoError(t, err, "Should insert handle node")

		endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

		insertAssignee(t, ctx, db, handleNode.ID, approval.AssigneeUser, []string{"handler1"}, 0)
		insertEdge(t, ctx, db, version.ID, startNode.ID, handleNode.ID)
		insertEdge(t, ctx, db, version.ID, handleNode.ID, endNode.ID)

		insertDelegation(t, ctx, db, "handler1", "delegate1", null.StringFrom(flow.ID), true)

		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		err = eng.StartProcess(ctx, db, instance)
		require.NoError(t, err, "StartProcess should succeed for handle node with delegation")

		tasks := queryTasksByNode(t, ctx, db, instance.ID, handleNode.ID)
		require.Len(t, tasks, 1, "Should create 1 task")
		assert.Equal(t, "delegate1", tasks[0].AssigneeID, "Task should be assigned to delegate")
		assert.Equal(t, "handler1", tasks[0].DelegateFromID.String, "DelegateFromID should reference original handler")
	})
}

// TestHandleProcessorPredictWithAssignees tests handle processor predict with assignees scenarios.
func TestHandleProcessorPredictWithAssignees(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	flow, version := createFlowAndVersion(t, ctx, db, "handle_predict_full", "Handle Predict Full")
	startNode := createFlowNode(t, ctx, db, version.ID, "start", approval.NodeStart, "Start")

	handleNode := &approval.FlowNode{
		FlowVersionID:          version.ID,
		NodeKey:                "handle_full",
		NodeKind:               approval.NodeHandle,
		Name:                   "Full Handle",
		ApprovalMethod:         approval.ApprovalParallel,
		PassRule:               approval.PassAny,
		PassRatio:              decimal.Zero,
		DuplicateHandlerAction: approval.DuplicateHandlerAutoPass,
	}
	handleNode.ID = id.Generate()
	handleNode.CreatedBy = "system"
	handleNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(handleNode).Exec(ctx)
	require.NoError(t, err, "Should insert handle node")

	endNode := createFlowNode(t, ctx, db, version.ID, "end", approval.NodeEnd, "End")

	insertAssignee(t, ctx, db, handleNode.ID, approval.AssigneeUser, []string{"h1", "h2"}, 0)
	insertEdge(t, ctx, db, version.ID, startNode.ID, handleNode.ID)
	insertEdge(t, ctx, db, version.ID, handleNode.ID, endNode.ID)

	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	nextNode, assigneeIDs, err := eng.PredictNextNode(ctx, db, instance, startNode)
	require.NoError(t, err, "PredictNextNode should succeed")
	assert.Equal(t, handleNode.ID, nextNode.ID, "Should predict handle node")
	assert.ElementsMatch(t, []string{"h1", "h2"}, assigneeIDs, "Should predict handle assignees")
}

// TestStartProcessWithEventPublisher tests start process with event publisher scenarios.
func TestStartProcessWithEventPublisher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eventPub := publisher.NewEventPublisher()
	eng := setupEngine(nil, nil, eventPub)

	flow, version, _, _ := buildAutoCompleteFlow(t, ctx, db)
	instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

	err := eng.StartProcess(ctx, db, instance)
	require.NoError(t, err, "StartProcess should succeed with event publisher")

	var events []approval.EventOutbox
	err = db.NewSelect().Model(&events).Scan(ctx)
	require.NoError(t, err, "Should query events without error")
	assert.True(t, len(events) > 0, "Should publish at least one event for auto-complete flow")
}

// TestPredictSameApplicantTransferAdmin tests predict same applicant transfer admin scenarios.
func TestPredictSameApplicantTransferAdmin(t *testing.T) {
	p := NewApprovalProcessor(nil, nil)

	pc := &ProcessContext{
		Node: &approval.FlowNode{
			SameApplicantAction: approval.SameApplicantTransferAdmin,
			AdminUserIDs:        []string{"admin1"},
		},
		ApplicantID: "u1",
	}
	ids, err := p.predictSameApplicant(context.Background(), pc)
	require.NoError(t, err, "Should not return error for transfer_admin same applicant")
	assert.Equal(t, []string{"u1"}, ids, "transfer_admin falls into default which returns applicant ID")
}

// TestPredictSameApplicantTransferSuperiorError tests predict same applicant transfer superior error scenarios.
func TestPredictSameApplicantTransferSuperiorError(t *testing.T) {
	mockOrg := &MockOrganizationService{err: errors.New("predict org error")}
	p := NewApprovalProcessor(mockOrg, nil)

	pc := &ProcessContext{
		Node:        &approval.FlowNode{SameApplicantAction: approval.SameApplicantTransferSuperior},
		ApplicantID: "u1",
	}
	_, err := p.predictSameApplicant(context.Background(), pc)
	require.Error(t, err, "Should return error when orgService fails")
	assert.Contains(t, err.Error(), "predict org error", "Error should contain org service message")
}

// --- Sub-flow tests ---

// TestSubFlowProcessorProcess tests sub flow processor process scenarios.
func TestSubFlowProcessorProcess(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	subFlow, _, _, subApprovalNode, _ := buildSimpleFlow(t, ctx, db)

	parentFlow, parentVersion := createFlowAndVersion(t, ctx, db, "parent_flow", "Parent Flow")
	parentStart := createFlowNode(t, ctx, db, parentVersion.ID, "start", approval.NodeStart, "Start")

	subFlowNode := &approval.FlowNode{
		FlowVersionID: parentVersion.ID,
		NodeKey:       "subflow1",
		NodeKind:      approval.NodeSubFlow,
		Name:          "Sub Flow",
		SubFlowConfig: map[string]any{
			"flowId": subFlow.ID,
			"dataMapping": []any{
				map[string]any{"sourceField": "title", "targetField": "sub_title"},
			},
		},
	}
	subFlowNode.ID = id.Generate()
	subFlowNode.CreatedBy = "system"
	subFlowNode.UpdatedBy = "system"
	_, err := db.NewInsert().Model(subFlowNode).Exec(ctx)
	require.NoError(t, err, "Should insert sub-flow node")

	parentEnd := createFlowNode(t, ctx, db, parentVersion.ID, "end", approval.NodeEnd, "End")

	insertEdge(t, ctx, db, parentVersion.ID, parentStart.ID, subFlowNode.ID)
	insertEdge(t, ctx, db, parentVersion.ID, subFlowNode.ID, parentEnd.ID)

	parentInstance := createInstance(t, ctx, db, parentFlow, parentVersion, "applicant1", map[string]any{"title": "parent"})

	err = eng.StartProcess(ctx, db, parentInstance)
	require.NoError(t, err, "StartProcess should succeed with sub-flow")

	parentInst := queryInstance(t, ctx, db, parentInstance.ID)
	assert.Equal(t, approval.InstanceRunning, parentInst.Status, "Parent should be running while sub-flow is pending")
	assert.Equal(t, subFlowNode.ID, parentInst.CurrentNodeID.String, "Parent should wait at sub-flow node")

	var subInstances []approval.Instance
	err = db.NewSelect().Model(&subInstances).Where(func(c orm.ConditionBuilder) {
		c.Equals("parent_instance_id", parentInstance.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query sub-instances without error")
	require.Len(t, subInstances, 1, "Should create one sub-flow instance")

	subInst := subInstances[0]
	assert.Equal(t, approval.InstanceRunning, subInst.Status, "Sub-flow should be running")
	assert.Equal(t, parentInstance.ID, subInst.ParentInstanceID.String, "Sub-flow should reference parent")
	assert.Equal(t, subFlowNode.ID, subInst.ParentNodeID.String, "Sub-flow should reference parent node")
	assert.Equal(t, "applicant1", subInst.ApplicantID, "Sub-flow should inherit applicant")

	subTasks := queryTasksByNode(t, ctx, db, subInst.ID, subApprovalNode.ID)
	assert.True(t, len(subTasks) > 0, "Sub-flow should have tasks at its approval node")
}

// TestSubFlowProcessorValidation tests sub flow processor validation scenarios.
func TestSubFlowProcessorValidation(t *testing.T) {
	t.Run("NilEngine", func(t *testing.T) {
		p := NewSubFlowProcessor()
		pc := &ProcessContext{
			Node: &approval.FlowNode{
				SubFlowConfig: map[string]any{"flowId": "f1"},
			},
			Instance: &approval.Instance{},
		}
		_, err := p.Process(context.Background(), pc)
		require.Error(t, err, "Should return error when engine is nil")
		assert.Contains(t, err.Error(), "FlowEngine not initialized", "Error should describe the nil engine")
	})

	t.Run("MissingConfig", func(t *testing.T) {
		p := NewSubFlowProcessor()
		eng := NewFlowEngine(nil, nil, nil)
		p.SetFlowEngine(eng)

		pc := &ProcessContext{
			Node:     &approval.FlowNode{},
			Instance: &approval.Instance{},
		}
		_, err := p.Process(context.Background(), pc)
		require.Error(t, err, "Should return error when sub flow config is nil")
		assert.Contains(t, err.Error(), "sub flow config is required", "Error should describe missing config")
	})

	t.Run("MissingFlowId", func(t *testing.T) {
		p := NewSubFlowProcessor()
		eng := NewFlowEngine(nil, nil, nil)
		p.SetFlowEngine(eng)

		pc := &ProcessContext{
			Node: &approval.FlowNode{
				SubFlowConfig: map[string]any{"other": "value"},
			},
			Instance: &approval.Instance{},
		}
		_, err := p.Process(context.Background(), pc)
		require.Error(t, err, "Should return error when flowId is missing")
		assert.Contains(t, err.Error(), "missing flowId", "Error should describe missing flowId")
	})
}

// TestSubFlowProcessorCycleDetection tests sub flow processor cycle detection scenarios.
func TestSubFlowProcessorCycleDetection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	eng := setupEngine(nil, nil)

	t.Run("DirectCycle", func(t *testing.T) {
		flow, version, _, _ := buildAutoCompleteFlow(t, ctx, db)
		instance := createInstance(t, ctx, db, flow, version, "applicant1", nil)

		subflowNode := &approval.FlowNode{
			FlowVersionID: version.ID,
			NodeKey:       "subflow_cycle",
			NodeKind:      approval.NodeSubFlow,
			Name:          "Cycle SubFlow",
			SubFlowConfig: map[string]any{"flowId": flow.ID},
		}
		subflowNode.ID = id.Generate()
		subflowNode.CreatedBy = "system"
		subflowNode.UpdatedBy = "system"
		_, err := db.NewInsert().Model(subflowNode).Exec(ctx)
		require.NoError(t, err, "Should insert sub-flow node")

		pc := &ProcessContext{
			DB:          db,
			Instance:    instance,
			Node:        subflowNode,
			FormData:    approval.NewFormData(nil),
			ApplicantID: "applicant1",
			Registry:    eng.registry,
		}

		subP := eng.processors[approval.NodeSubFlow].(*SubFlowProcessor)
		_, err = subP.Process(ctx, pc)
		require.Error(t, err, "Should detect circular sub-flow reference")
		assert.ErrorIs(t, err, ErrSubFlowCycle, "Error should be ErrSubFlowCycle")
	})

	t.Run("ParentChainCycle", func(t *testing.T) {
		flowA, versionA := createFlowAndVersion(t, ctx, db, "flow_a_chain", "Flow A")
		flowB, versionB := createFlowAndVersion(t, ctx, db, "flow_b_chain", "Flow B")

		parentInstance := createParentInstance(t, ctx, db, flowA, versionA, "applicant1")

		childInstance := &approval.Instance{
			FlowID:           flowB.ID,
			FlowVersionID:    versionB.ID,
			ParentInstanceID: null.StringFrom(parentInstance.ID),
			ParentNodeID:     null.StringFrom(id.Generate()),
			Title:            "Child Instance",
			SerialNo:         id.Generate(),
			ApplicantID:      "applicant1",
			Status:           approval.InstanceRunning,
		}
		childInstance.ID = id.Generate()
		childInstance.CreatedBy = "applicant1"
		childInstance.UpdatedBy = "applicant1"
		_, err := db.NewInsert().Model(childInstance).Exec(ctx)
		require.NoError(t, err, "Should insert child instance")

		subP := eng.processors[approval.NodeSubFlow].(*SubFlowProcessor)
		err = subP.detectSubFlowCycle(ctx, db, childInstance, flowA.ID)
		require.Error(t, err, "Should detect cycle when ancestor instance belongs to target flow")
		assert.ErrorIs(t, err, ErrSubFlowCycle, "Error should be ErrSubFlowCycle")
	})

	t.Run("NoParentNoCycle", func(t *testing.T) {
		flowA, versionA := createFlowAndVersion(t, ctx, db, "flow_a_np", "Flow A")
		_, _ = createFlowAndVersion(t, ctx, db, "flow_b_np", "Flow B")

		instance := createParentInstance(t, ctx, db, flowA, versionA, "applicant1")

		subP := eng.processors[approval.NodeSubFlow].(*SubFlowProcessor)
		err := subP.detectSubFlowCycle(ctx, db, instance, "some_other_flow")
		assert.NoError(t, err, "Should not detect cycle when targeting a different flow and no parent chain")
	})
}

// TestResumeParentFlow tests resume parent flow scenarios.
func TestResumeParentFlow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("ApprovedChild", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		parentFlow, parentVersion := createFlowAndVersion(t, ctx, db, "parent_resume_flow", "Parent Resume")
		parentStart := createFlowNode(t, ctx, db, parentVersion.ID, "start", approval.NodeStart, "Start")
		subFlowNode := createFlowNode(t, ctx, db, parentVersion.ID, "subflow", approval.NodeSubFlow, "SubFlow")
		parentEnd := createFlowNode(t, ctx, db, parentVersion.ID, "end", approval.NodeEnd, "End")

		insertEdge(t, ctx, db, parentVersion.ID, parentStart.ID, subFlowNode.ID)
		insertEdge(t, ctx, db, parentVersion.ID, subFlowNode.ID, parentEnd.ID)

		parentInstance := createRunningParentInstance(t, ctx, db, parentFlow, parentVersion, subFlowNode.ID)
		childInstance := createChildInstance(t, ctx, db, parentInstance.ID, subFlowNode.ID, approval.InstanceApproved)

		err := eng.ResumeParentFlow(ctx, db, childInstance, approval.InstanceApproved)
		require.NoError(t, err, "ResumeParentFlow should succeed when child is approved")

		parentInst := queryInstance(t, ctx, db, parentInstance.ID)
		assert.Equal(t, approval.InstanceApproved, parentInst.Status, "Parent should be approved after child approved")
	})

	t.Run("RejectedChild", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		parentFlow, parentVersion := createFlowAndVersion(t, ctx, db, "parent_reject_flow", "Parent Reject")
		subFlowNode := createFlowNode(t, ctx, db, parentVersion.ID, "subflow", approval.NodeSubFlow, "SubFlow")

		parentInstance := createRunningParentInstance(t, ctx, db, parentFlow, parentVersion, subFlowNode.ID)
		childInstance := createChildInstance(t, ctx, db, parentInstance.ID, subFlowNode.ID, approval.InstanceRejected)

		err := eng.ResumeParentFlow(ctx, db, childInstance, approval.InstanceRejected)
		require.NoError(t, err, "ResumeParentFlow should succeed when child is rejected")

		parentInst := queryInstance(t, ctx, db, parentInstance.ID)
		assert.Equal(t, approval.InstanceRejected, parentInst.Status, "Parent should be rejected when child is rejected")
		assert.True(t, parentInst.FinishedAt.Valid, "Parent FinishedAt should be set")
	})

	t.Run("ParentNotRunning", func(t *testing.T) {
		eng := setupEngine(nil, nil)
		parentFlow, parentVersion := createFlowAndVersion(t, ctx, db, "parent_finished_flow", "Parent Finished")
		subFlowNode := createFlowNode(t, ctx, db, parentVersion.ID, "subflow", approval.NodeSubFlow, "SubFlow")

		parentInstance := &approval.Instance{
			FlowID:        parentFlow.ID,
			FlowVersionID: parentVersion.ID,
			Title:         "Parent Instance",
			SerialNo:      id.Generate(),
			ApplicantID:   "applicant1",
			Status:        approval.InstanceApproved,
			CurrentNodeID: null.StringFrom(subFlowNode.ID),
		}
		parentInstance.ID = id.Generate()
		parentInstance.CreatedBy = "applicant1"
		parentInstance.UpdatedBy = "applicant1"
		_, err := db.NewInsert().Model(parentInstance).Exec(ctx)
		require.NoError(t, err, "Should insert parent instance")

		childInstance := createChildInstance(t, ctx, db, parentInstance.ID, subFlowNode.ID, approval.InstanceApproved)

		err = eng.ResumeParentFlow(ctx, db, childInstance, approval.InstanceApproved)
		require.NoError(t, err, "ResumeParentFlow should be no-op when parent is not running")

		parentInst := queryInstance(t, ctx, db, parentInstance.ID)
		assert.Equal(t, approval.InstanceApproved, parentInst.Status, "Parent status should remain unchanged")
	})

	t.Run("WithEventPublisher", func(t *testing.T) {
		eventPub := publisher.NewEventPublisher()
		eng := setupEngine(nil, nil, eventPub)

		parentFlow, parentVersion := createFlowAndVersion(t, ctx, db, "parent_event_flow", "Parent Event")
		parentStart := createFlowNode(t, ctx, db, parentVersion.ID, "start", approval.NodeStart, "Start")
		subFlowNode := createFlowNode(t, ctx, db, parentVersion.ID, "subflow", approval.NodeSubFlow, "SubFlow")
		parentEnd := createFlowNode(t, ctx, db, parentVersion.ID, "end", approval.NodeEnd, "End")

		insertEdge(t, ctx, db, parentVersion.ID, parentStart.ID, subFlowNode.ID)
		insertEdge(t, ctx, db, parentVersion.ID, subFlowNode.ID, parentEnd.ID)

		parentInstance := createRunningParentInstance(t, ctx, db, parentFlow, parentVersion, subFlowNode.ID)
		childInstance := createChildInstance(t, ctx, db, parentInstance.ID, subFlowNode.ID, approval.InstanceRejected)

		err := eng.ResumeParentFlow(ctx, db, childInstance, approval.InstanceRejected)
		require.NoError(t, err, "ResumeParentFlow should succeed with event publisher")

		var events []approval.EventOutbox
		err = db.NewSelect().Model(&events).Scan(ctx)
		require.NoError(t, err, "Should query events without error")
		assert.True(t, len(events) >= 2, "Should publish sub-flow completed and instance completed events")
	})
}

// TestHandleProcessResultUnknownAction tests handle process result unknown action scenarios.
func TestHandleProcessResultUnknownAction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	eng := NewFlowEngine(nil, nil, nil)
	instance := &approval.Instance{ApplicantID: "u1"}
	node := &approval.FlowNode{}
	node.ID = id.Generate()

	result := &ProcessResult{Action: NodeAction(99)}
	err := eng.handleProcessResult(context.Background(), db, instance, node, result)
	require.Error(t, err, "Should return error for unknown node action")
	assert.Contains(t, err.Error(), "unknown node action", "Error should describe the unknown action")
}
