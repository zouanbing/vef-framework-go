package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// FlowSetup holds references to a deployed and published flow for E2E testing.
type FlowSetup struct {
	FlowID   string
	FlowCode string
}

// InstanceResourceTestSuite tests instance lifecycle and query operations via HTTP.
type InstanceResourceTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string
	simple     FlowSetup // Start → End
	approval   FlowSetup // Start → Approval(user-1/user-2, sequential) → End
	complex    FlowSetup // Start → Condition → [Approval/Handle] → End
}

func TestInstanceResource(t *testing.T) {
	suite.Run(t, new(InstanceResourceTestSuite))
}

func (s *InstanceResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, s.token = setupResourceApp(&s.Suite)

	// Create shared category
	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "inst-test-cat",
		Name:     "Instance Test Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID

	// Pre-build flows
	s.simple = s.createAndPublishFlow("res-simple", "Simple Flow", simpleFlowDef())
	s.approval = s.createAndPublishFlow("res-approval", "Approval Flow", approvalFlowDef())
	s.complex = s.createAndPublishFlow("res-complex", "Complex Flow", complexFlowDef())
}

func (s *InstanceResourceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	s.TearDownApp()
}

func (s *InstanceResourceTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

// --- Suite-local RPC helpers ---

// createAndPublishFlow creates, deploys, and publishes a flow via RPC calls.
func (s *InstanceResourceTestSuite) createAndPublishFlow(code, name string, def approval.FlowDefinition) FlowSetup {
	// Create flow
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "create", Version: "v1"},
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   code,
			"name":                   name,
			"categoryId":             s.categoryID,
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  name + " {{.InstanceNo}}",
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should create flow: "+code)
	flowData := s.ReadDataAsMap(res.Data)
	flowID := flowData["id"].(string)

	// Deploy
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "deploy", Version: "v1"},
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(def),
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	res = s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow: "+code)
	versionData := s.ReadDataAsMap(res.Data)
	versionID := versionData["id"].(string)

	// Publish
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "publish_version", Version: "v1"},
		Params:     map[string]any{"versionId": versionID},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	res = s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should publish flow: "+code)

	return FlowSetup{FlowID: flowID, FlowCode: code}
}

// startInstance starts an instance via RPC and returns the response data map.
func (s *InstanceResourceTestSuite) startInstance(flowCode string, formData map[string]any) map[string]any {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "start", Version: "v1"},
		Params: map[string]any{
			"tenantId": "default",
			"flowCode": flowCode,
			"formData": formData,
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should start instance for flow: "+flowCode)
	return s.ReadDataAsMap(res.Data)
}

// findPendingTasks queries pending tasks for the given instance.
func (s *InstanceResourceTestSuite) findPendingTasks(instanceID string) []map[string]any {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "find_tasks", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"status":     "pending",
			"page":       1,
			"pageSize":   50,
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should find tasks")

	data := s.ReadDataAsMap(res.Data)
	items, ok := data["items"].([]any)
	if !ok || len(items) == 0 {
		return nil
	}

	tasks := make([]map[string]any, len(items))
	for i, item := range items {
		tasks[i] = item.(map[string]any)
	}
	return tasks
}

// processTask sends a process_task RPC and returns the result.
func (s *InstanceResourceTestSuite) processTask(taskID, action, opinion string) result.Result {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "process_task", Version: "v1"},
		Params: map[string]any{
			"taskId":  taskID,
			"action":  action,
			"opinion": opinion,
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	return s.ReadResult(resp)
}

// --- Test methods ---

func (s *InstanceResourceTestSuite) TestStartSimpleFlow() {
	data := s.startInstance(s.simple.FlowCode, nil)

	instanceID, ok := data["id"].(string)
	s.Require().True(ok, "Should return instance ID")
	s.Assert().NotEmpty(instanceID)

	// Simple flow (Start → End) should auto-complete
	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceApproved, inst.Status, "Simple flow should auto-complete")
}

func (s *InstanceResourceTestSuite) TestStartApprovalFlow() {
	data := s.startInstance(s.approval.FlowCode, nil)

	instanceID := data["id"].(string)

	// Should be running (waiting for approval)
	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceRunning, inst.Status, "Approval flow should be running")

	// Should have pending tasks
	tasks := s.findPendingTasks(instanceID)
	s.Assert().NotEmpty(tasks, "Should have pending tasks")
}

func (s *InstanceResourceTestSuite) TestStartFlowNotFound() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "start", Version: "v1"},
		Params: map[string]any{
			"tenantId": "default",
			"flowCode": "non-existent-flow",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().False(res.IsOk(), "Should fail for non-existent flow")
}

func (s *InstanceResourceTestSuite) TestProcessTaskApprove() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	// Sequential approval: approve tasks one by one until instance completes
	for i := 0; i < 10; i++ {
		tasks := s.findPendingTasks(instanceID)
		if len(tasks) == 0 {
			break
		}
		res := s.processTask(tasks[0]["id"].(string), "approve", "Approved")
		s.Assert().True(res.IsOk(), "Should approve task")
	}

	// Verify instance completed
	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceApproved, inst.Status, "Instance should be approved after all tasks approved")
}

func (s *InstanceResourceTestSuite) TestProcessTaskReject() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	tasks := s.findPendingTasks(instanceID)
	s.Require().NotEmpty(tasks, "Should have pending tasks")

	res := s.processTask(tasks[0]["id"].(string), "reject", "Rejected")
	s.Assert().True(res.IsOk(), "Should reject task")

	// Verify instance rejected
	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceRejected, inst.Status, "Instance should be rejected")
}

func (s *InstanceResourceTestSuite) TestProcessTaskTransfer() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	tasks := s.findPendingTasks(instanceID)
	s.Require().NotEmpty(tasks, "Should have pending tasks")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "process_task", Version: "v1"},
		Params: map[string]any{
			"taskId":       tasks[0]["id"].(string),
			"action":       "transfer",
			"opinion":      "Please handle this",
			"transferToId": "new-assignee-1",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should transfer task")

	// Verify original task is transferred
	var original approval.Task
	original.ID = tasks[0]["id"].(string)
	s.Require().NoError(s.db.NewSelect().Model(&original).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.TaskTransferred, original.Status, "Original task should be transferred")
}

func (s *InstanceResourceTestSuite) TestWithdraw() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "withdraw", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"reason":     "Changed my mind",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should withdraw instance")

	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceWithdrawn, inst.Status, "Instance should be withdrawn")
}

func (s *InstanceResourceTestSuite) TestResubmit() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	// Withdraw first (resubmit requires withdrawn or returned state)
	withdrawResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "withdraw", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"reason":     "Need to revise",
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, withdrawResp.StatusCode)
	withdrawRes := s.ReadResult(withdrawResp)
	s.Require().True(withdrawRes.IsOk(), "Should withdraw instance")

	// Resubmit
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "resubmit", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"formData":   map[string]any{"revised": true},
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should resubmit instance")

	var inst approval.Instance
	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.InstanceRunning, inst.Status, "Instance should be running after resubmit")
}

func (s *InstanceResourceTestSuite) TestAddCC() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "add_cc", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"cc-user-1", "cc-user-2"},
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should add CC")

	// Verify CC records exist
	count, err := s.db.NewSelect().Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).Count(s.ctx)
	s.Require().NoError(err)
	// auto CC (cc-user-1 from flow def) + manual CC (cc-user-1, cc-user-2) with dedup
	s.Assert().GreaterOrEqual(count, int64(2), "Should have CC records")
}

func (s *InstanceResourceTestSuite) TestMarkCCRead() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	// Add CC first
	addResp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "add_cc", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"ccUserIds":  []string{"test-admin"},
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, addResp.StatusCode)

	// Mark as read
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "mark_cc_read", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should mark CC as read")
}

func (s *InstanceResourceTestSuite) TestUrgeTask() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	tasks := s.findPendingTasks(instanceID)
	s.Require().NotEmpty(tasks, "Should have pending tasks")

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "urge_task", Version: "v1"},
		Params: map[string]any{
			"taskId":  tasks[0]["id"].(string),
			"message": "Please review ASAP",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should urge task")

	// Verify urge record
	count, err := s.db.NewSelect().Model((*approval.UrgeRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", tasks[0]["id"].(string)) }).Count(s.ctx)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), count, "Should have one urge record")
}

func (s *InstanceResourceTestSuite) TestFindInstances() {
	// Start multiple instances
	s.startInstance(s.approval.FlowCode, nil)
	s.startInstance(s.approval.FlowCode, nil)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "find_instances", Version: "v1"},
		Params: map[string]any{
			"tenantId": "default",
			"page":     1,
			"pageSize": 10,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should find instances")

	data := s.ReadDataAsMap(res.Data)
	total, ok := data["total"].(float64)
	s.Require().True(ok, "Total should be a number")
	s.Assert().GreaterOrEqual(int(total), 2, "Should find at least 2 instances")
}

func (s *InstanceResourceTestSuite) TestFindTasks() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "find_tasks", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
			"page":       1,
			"pageSize":   10,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should find tasks")

	resData := s.ReadDataAsMap(res.Data)
	total, ok := resData["total"].(float64)
	s.Require().True(ok)
	s.Assert().GreaterOrEqual(int(total), 1, "Should find at least 1 task")
}

func (s *InstanceResourceTestSuite) TestGetDetail() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "get_detail", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should get instance detail")

	detail := s.ReadDataAsMap(res.Data)
	s.Assert().NotNil(detail["instance"], "Detail should contain instance")
	s.Assert().NotNil(detail["tasks"], "Detail should contain tasks")
}

func (s *InstanceResourceTestSuite) TestGetActionLogs() {
	data := s.startInstance(s.approval.FlowCode, nil)
	instanceID := data["id"].(string)

	// Approve a task to generate action logs
	tasks := s.findPendingTasks(instanceID)
	if len(tasks) > 0 {
		s.processTask(tasks[0]["id"].(string), "approve", "Looks good")
	}

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/instance", Action: "get_action_logs", Version: "v1"},
		Params: map[string]any{
			"instanceId": instanceID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should get action logs")
}

func (s *InstanceResourceTestSuite) TestConditionRouting() {
	s.Run("HighAmount", func() {
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 2000})
		instanceID := data["id"].(string)

		var inst approval.Instance
		inst.ID = instanceID
		s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
		s.Assert().Equal(approval.InstanceRunning, inst.Status, "High amount should route to approval and be running")

		// Should have a task assigned to manager-1 (from approval-high node)
		tasks := s.findPendingTasks(instanceID)
		s.Assert().NotEmpty(tasks, "Should have pending tasks for high-amount branch")
	})

	s.Run("LowAmount", func() {
		data := s.startInstance(s.complex.FlowCode, map[string]any{"amount": 500})
		instanceID := data["id"].(string)

		var inst approval.Instance
		inst.ID = instanceID
		s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx))
		s.Assert().Equal(approval.InstanceRunning, inst.Status, "Low amount should route to handle and be running")

		// Should have a task assigned to handler-1 (from handle-default node)
		tasks := s.findPendingTasks(instanceID)
		s.Assert().NotEmpty(tasks, "Should have pending tasks for default branch")
	})
}
