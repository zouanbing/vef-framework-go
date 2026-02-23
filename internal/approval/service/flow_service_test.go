package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/orm"
)

type FlowServiceTestSuite struct {
	suite.Suite
	ctx     context.Context
	db      orm.DB
	svc     *FlowService
	cleanup func()
}

// TestFlowServiceTestSuite tests flow service test suite scenarios.
func TestFlowServiceTestSuite(t *testing.T) {
	suite.Run(t, new(FlowServiceTestSuite))
}

func (s *FlowServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.db, s.cleanup = setupTestDB(s.T())
	pub := publisher.NewEventPublisher()
	s.svc = NewFlowService(s.db, pub)
}

func (s *FlowServiceTestSuite) TearDownTest() {
	s.cleanup()
}

func (s *FlowServiceTestSuite) validDefinitionJSON() string {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start Node"}},
			{ID: "approval", Type: "approval", Data: map[string]any{"label": "Approval Node"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End Node"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "approval"},
			{Source: "approval", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)

	return string(data)
}

func (s *FlowServiceTestSuite) TestDeployFlowNewFlow() {
	cmd := DeployFlowCmd{
		FlowCode:   "leave_apply",
		FlowName:   "Leave Application",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")
	s.Require().NotNil(flow, "Should not be nil")

	s.Equal("leave_apply", flow.Code)
	s.Equal("Leave Application", flow.Name)
	s.Equal(1, flow.CurrentVersion)
	s.True(flow.IsActive)

	// Check version was created
	var version approval.FlowVersion

	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.VersionDraft, version.Status)
	s.Equal(1, version.Version)

	// Check nodes were created
	var nodes []approval.FlowNode

	err = s.db.NewSelect().Model(&nodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Len(nodes, 3)

	// Check edges were created
	var edges []approval.FlowEdge

	err = s.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Len(edges, 2)
}

func (s *FlowServiceTestSuite) TestDeployFlowUpdateExistingFlow() {
	cmd := DeployFlowCmd{
		FlowCode:   "leave_apply",
		FlowName:   "Leave V1",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow1, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")
	s.Equal(1, flow1.CurrentVersion)

	// Deploy again (update)
	cmd.FlowName = "Leave V2"
	flow2, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")
	s.Equal(2, flow2.CurrentVersion)
	s.Equal("Leave V2", flow2.Name)
	s.Equal(flow1.ID, flow2.ID) // Same flow ID

	// Check two versions exist
	var versions []approval.FlowVersion

	err = s.db.NewSelect().Model(&versions).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow2.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Len(versions, 2)
}

func (s *FlowServiceTestSuite) TestDeployFlowInvalidJSON() {
	cmd := DeployFlowCmd{
		FlowCode:   "bad_flow",
		FlowName:   "Bad Flow",
		CategoryID: "cat1",
		Definition: "not-valid-json",
		OperatorID: "admin",
	}

	_, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
}

func (s *FlowServiceTestSuite) TestDeployFlowQueryErrorReturned() {
	_, err := s.db.NewRaw("DROP TABLE apv_flow").Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "query_error_flow",
		FlowName:   "Query Error Flow",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.Contains(err.Error(), "query flow by code")
}

func (s *FlowServiceTestSuite) TestPublishVersionSuccess() {
	// Deploy a flow first
	cmd := DeployFlowCmd{
		FlowCode:   "pub_flow",
		FlowName:   "Publish Flow",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	// Get the version
	var version approval.FlowVersion

	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.VersionDraft, version.Status)

	// Publish
	err = s.svc.PublishVersion(s.ctx, version.ID, "admin")
	s.Require().NoError(err, "Should not return error")

	// Verify version is published
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.VersionPublished, version.Status)
	s.True(version.PublishedAt.Valid)
	s.Equal("admin", version.PublishedBy.String)

	// Verify event was published
	events := queryEvents(s.T(), s.ctx, s.db)
	s.NotEmpty(events)

	found := false
	for _, e := range events {
		if e.EventType == "approval.flow.published" {
			found = true

			break
		}
	}

	s.True(found)
}

func (s *FlowServiceTestSuite) TestPublishVersionArchivesOldVersion() {
	// Deploy and publish V1
	cmd := DeployFlowCmd{
		FlowCode:   "archive_flow",
		FlowName:   "Archive Flow V1",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var v1 approval.FlowVersion
	err = s.db.NewSelect().Model(&v1).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
		c.Equals("version", 1)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	err = s.svc.PublishVersion(s.ctx, v1.ID, "admin")
	s.Require().NoError(err, "Should not return error")

	// Deploy V2
	cmd.FlowName = "Archive Flow V2"
	_, err = s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var v2 approval.FlowVersion
	err = s.db.NewSelect().Model(&v2).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
		c.Equals("version", 2)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Publish V2 (should archive V1)
	err = s.svc.PublishVersion(s.ctx, v2.ID, "admin")
	s.Require().NoError(err, "Should not return error")

	// V1 should be archived
	err = s.db.NewSelect().Model(&v1).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", v1.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Equal(approval.VersionArchived, v1.Status)
}

func (s *FlowServiceTestSuite) TestPublishVersionNotDraft() {
	// Deploy and publish first
	cmd := DeployFlowCmd{
		FlowCode:   "dup_pub",
		FlowName:   "Dup Publish",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var version approval.FlowVersion

	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	err = s.svc.PublishVersion(s.ctx, version.ID, "admin")
	s.Require().NoError(err, "Should not return error")

	// Try to publish again
	err = s.svc.PublishVersion(s.ctx, version.ID, "admin")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrVersionNotDraft)
}

func (s *FlowServiceTestSuite) TestGetFlowGraph() {
	// Build and publish a flow
	cmd := DeployFlowCmd{
		FlowCode:   "graph_flow",
		FlowName:   "Graph Flow",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var version approval.FlowVersion

	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	err = s.svc.PublishVersion(s.ctx, version.ID, "admin")
	s.Require().NoError(err, "Should not return error")

	// Get graph
	graph, err := s.svc.GetFlowGraph(s.ctx, flow.ID)
	s.Require().NoError(err, "Should not return error")
	s.Require().NotNil(graph, "Should not be nil")

	s.Equal(flow.ID, graph.Flow.ID)
	s.Equal(approval.VersionPublished, graph.Version.Status)
	s.Len(graph.Nodes, 3)
	s.Len(graph.Edges, 2)
}

func (s *FlowServiceTestSuite) TestGetFlowGraphFlowNotFound() {
	_, err := s.svc.GetFlowGraph(s.ctx, "nonexistent")
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrFlowNotFound)
}

func (s *FlowServiceTestSuite) TestDeployFlowEdgesUseRealNodeIDs() {
	cmd := DeployFlowCmd{
		FlowCode:   "edge_test",
		FlowName:   "Edge Test",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	// Build nodeKey→nodeID map
	var nodes []approval.FlowNode
	err = s.db.NewSelect().Model(&nodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	nodeKeyToID := make(map[string]string)
	for _, n := range nodes {
		nodeKeyToID[n.NodeKey] = n.ID
	}

	var edges []approval.FlowEdge
	err = s.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.Require().Len(edges, 2, "Length should match expected value")

	// Verify edges reference real node IDs (not canvas keys)
	s.Equal(nodeKeyToID["start"], edges[0].SourceNodeID)
	s.Equal(nodeKeyToID["approval"], edges[0].TargetNodeID)
	s.Equal(nodeKeyToID["approval"], edges[1].SourceNodeID)
	s.Equal(nodeKeyToID["end"], edges[1].TargetNodeID)

	// Verify they are NOT canvas keys
	s.NotEqual("start", edges[0].SourceNodeID)
	s.NotEqual("approval", edges[0].TargetNodeID)
}

func (s *FlowServiceTestSuite) TestDeployFlowPropertiesMappedToNode() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "approval", Type: "approval", Data: map[string]any{
				"label":                "Approval",
				"approvalMethod":       "parallel",
				"passRule":             "ratio",
				"isTransferAllowed":    true,
				"isRollbackAllowed":    true,
				"isManualCcAllowed":    true,
				"isOpinionRequired":    true,
				"rollbackType":         "previous",
				"rollbackDataStrategy": "keep",
			}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "approval"},
			{Source: "approval", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	cmd := DeployFlowCmd{
		FlowCode:   "props_test",
		FlowName:   "Properties Test",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	var version approval.FlowVersion
	err = s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	var approvalNode approval.FlowNode
	err = s.db.NewSelect().Model(&approvalNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
		c.Equals("node_key", "approval")
	}).Scan(s.ctx)
	s.Require().NoError(err, "Should not return error")

	s.Equal(approval.ApprovalParallel, approvalNode.ApprovalMethod)
	s.Equal(approval.PassRatio, approvalNode.PassRule)
	s.True(approvalNode.IsTransferAllowed)
	s.True(approvalNode.IsRollbackAllowed)
	s.True(approvalNode.IsManualCCAllowed)
	s.True(approvalNode.IsOpinionRequired)
	s.Equal(approval.RollbackPrevious, approvalNode.RollbackType)
	s.Equal(approval.RollbackDataKeep, approvalNode.RollbackDataStrategy)
}

func (s *FlowServiceTestSuite) TestDeployFlowInvalidSourceNodeKey() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "nonexistent", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode:   "bad_key_flow",
		FlowName:   "Bad Key",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
}

// ==================== P1-3: Flow Structure Validation ====================

func (s *FlowServiceTestSuite) TestDeployFlowNoStartNode() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "approval", Type: "approval", Data: map[string]any{"label": "Approval"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{{Source: "approval", Target: "end"}},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "no_start", FlowName: "No Start", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "exactly 1 start node")
}

func (s *FlowServiceTestSuite) TestDeployFlowMultipleStartNodes() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start1", Type: "start", Data: map[string]any{"label": "Start 1"}},
			{ID: "start2", Type: "start", Data: map[string]any{"label": "Start 2"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start1", Target: "end"},
			{Source: "start2", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "multi_start", FlowName: "Multi Start", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "exactly 1 start node")
}

func (s *FlowServiceTestSuite) TestDeployFlowNoEndNode() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "approval", Type: "approval", Data: map[string]any{"label": "Approval"}},
		},
		Edges: []approval.EdgeDefinition{{Source: "start", Target: "approval"}},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "no_end", FlowName: "No End", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "at least 1 end node")
}

func (s *FlowServiceTestSuite) TestDeployFlowInvalidNodeKind() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "unknown", Type: "magic", Data: map[string]any{"label": "Magic Node"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "unknown"},
			{Source: "unknown", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "bad_kind", FlowName: "Bad Kind", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "invalid node kind")
}

func (s *FlowServiceTestSuite) TestDeployFlowDuplicateNodeID() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "start", Type: "end", Data: map[string]any{"label": "Also Start"}},
		},
		Edges: []approval.EdgeDefinition{{Source: "start", Target: "start"}},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "dup_id", FlowName: "Dup ID", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "duplicate node ID")
}

func (s *FlowServiceTestSuite) TestDeployFlowSubFlowMissingConfig() {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "sf", Type: "sub_flow", Data: map[string]any{"label": "SubFlow"}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "sf"},
			{Source: "sf", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	_, err := s.svc.DeployFlow(s.ctx, DeployFlowCmd{
		FlowCode: "sf_no_cfg", FlowName: "SF No Cfg", CategoryID: "cat1",
		Definition: string(data), OperatorID: "admin",
	})
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrInvalidFlowDesign)
	s.Contains(err.Error(), "subFlowConfig")
}

func (s *FlowServiceTestSuite) TestGetFlowGraphNoPublishedVersion() {
	cmd := DeployFlowCmd{
		FlowCode:   "no_pub",
		FlowName:   "No Published",
		CategoryID: "cat1",
		Definition: s.validDefinitionJSON(),
		OperatorID: "admin",
	}

	flow, err := s.svc.DeployFlow(s.ctx, cmd)
	s.Require().NoError(err, "Should not return error")

	// Flow exists but version is draft
	_, err = s.svc.GetFlowGraph(s.ctx, flow.ID)
	s.Require().Error(err, "Should return error")
	s.ErrorIs(err, ErrNoPublishedVersion)
}

// TestToStringSlice tests to string slice scenarios.
func TestToStringSlice(t *testing.T) {
	t.Run("ValidStringSlice", func(t *testing.T) {
		result, ok := toStringSlice([]any{"a", "b", "c"})
		assert.True(t, ok, "Should return true for valid string slice")
		assert.Equal(t, []string{"a", "b", "c"}, result, "Should convert all elements")
	})

	t.Run("NonArrayInput", func(t *testing.T) {
		result, ok := toStringSlice("not an array")
		assert.False(t, ok, "Should return false for non-array input")
		assert.Nil(t, result, "Should return nil for non-array input")
	})

	t.Run("EmptyArray", func(t *testing.T) {
		result, ok := toStringSlice([]any{})
		assert.False(t, ok, "Empty array should return false")
		assert.Empty(t, result, "Empty array should return empty slice")
	})

	t.Run("MixedTypesOnlyStrings", func(t *testing.T) {
		result, ok := toStringSlice([]any{"a", 123, "b"})
		assert.True(t, ok, "Mixed array with strings should return true")
		assert.Equal(t, []string{"a", "b"}, result, "Non-string elements should be skipped")
	})
}

// TestApplyNodeDataExtendedCoverage tests apply node data extended coverage scenarios.
func TestApplyNodeDataExtendedCoverage(t *testing.T) {
	t.Run("AllProperties", func(t *testing.T) {
		node := &approval.FlowNode{}
		data := map[string]any{
			"executionType":          "auto",
			"emptyHandlerAction":     "auto_pass",
			"sameApplicantAction":    "auto_pass",
			"duplicateHandlerAction": "auto_pass",
			"timeoutHours":           float64(24),
			"adminUserIds":           []any{"admin1", "admin2"},
			"fallbackUserIds":        []any{"fallback1"},
			"addAssigneeTypes":       []any{"before", "after"},
			"subFlowConfig":          map[string]any{"flowId": "test"},
			"fieldPermissions":       map[string]any{"name": "editable"},
			"passRatio":              "0.6",
		}
		applyNodeData(node, data)

		assert.Equal(t, approval.ExecutionAuto, node.ExecutionType, "ExecutionType should be auto")
		assert.Equal(t, approval.EmptyHandlerAutoPass, node.EmptyHandlerAction, "EmptyHandlerAction should be auto_pass")
		assert.Equal(t, approval.SameApplicantAutoPass, node.SameApplicantAction, "SameApplicantAction should be auto_pass")
		assert.Equal(t, approval.DuplicateHandlerAutoPass, node.DuplicateHandlerAction, "DuplicateHandlerAction should be auto_pass")
		assert.Equal(t, 24, node.TimeoutHours, "TimeoutHours should be 24")
		assert.Equal(t, []string{"admin1", "admin2"}, node.AdminUserIDs, "AdminUserIDs should match")
		assert.Equal(t, []string{"fallback1"}, node.FallbackUserIDs, "FallbackUserIDs should match")
		assert.Equal(t, []string{"before", "after"}, node.AddAssigneeTypes, "AddAssigneeTypes should match")
		assert.NotNil(t, node.SubFlowConfig, "SubFlowConfig should be set")
		assert.NotNil(t, node.FieldPermissions, "FieldPermissions should be set")
		assert.Equal(t, "0.6", node.PassRatio.String(), "PassRatio should be 0.6")
	})

	t.Run("PassRatioAsFloat", func(t *testing.T) {
		node := &approval.FlowNode{}
		applyNodeData(node, map[string]any{"passRatio": float64(0.75)})
		assert.Equal(t, "0.75", node.PassRatio.String(), "Float passRatio should be converted correctly")
	})

	t.Run("EmptyData", func(t *testing.T) {
		node := &approval.FlowNode{}
		applyNodeData(node, nil)
		assert.Equal(t, approval.ApprovalMethod(""), node.ApprovalMethod, "Nil data should leave defaults")
	})
}

// TestValidateFlowDefinition tests validate flow definition scenarios.
func TestValidateFlowDefinition(t *testing.T) {
	tests := []struct {
		name        string
		def         *approval.FlowDefinition
		errContains string
	}{
		{
			name:        "EmptyNodes",
			def:         &approval.FlowDefinition{Nodes: []approval.NodeDefinition{}, Edges: []approval.EdgeDefinition{}},
			errContains: "at least one node",
		},
		{
			name: "EmptyNodeID",
			def: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{{ID: "", Type: "start"}},
			},
			errContains: "node ID must not be empty",
		},
		{
			name: "DuplicateNodeID",
			def: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{{ID: "start", Type: "start"}, {ID: "start", Type: "end"}},
			},
			errContains: "duplicate node ID",
		},
		{
			name: "EdgeUnknownSource",
			def: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
				Edges: []approval.EdgeDefinition{{Source: "nonexistent", Target: "end"}},
			},
			errContains: "unknown source node",
		},
		{
			name: "EdgeUnknownTarget",
			def: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
				Edges: []approval.EdgeDefinition{{Source: "start", Target: "nonexistent"}},
			},
			errContains: "unknown target node",
		},
		{
			name: "SubFlowMissingFlowId",
			def: &approval.FlowDefinition{
				Nodes: []approval.NodeDefinition{
					{ID: "start", Type: "start"},
					{ID: "sf", Type: "sub_flow", Data: map[string]any{"subFlowConfig": map[string]any{}}},
					{ID: "end", Type: "end"},
				},
				Edges: []approval.EdgeDefinition{
					{Source: "start", Target: "sf"},
					{Source: "sf", Target: "end"},
				},
			},
			errContains: "missing flowId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlowDefinition(tt.def)
			require.Error(t, err, "Validation should fail")
			assert.Contains(t, err.Error(), tt.errContains, "Error should contain expected message")
		})
	}
}

// TestExtractFromData tests extract from data scenarios.
func TestExtractFromData(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		key  string
		msg  string
	}{
		{
			name: "NilData",
			data: nil,
			key:  "key",
			msg:  "Nil data should return nil",
		},
		{
			name: "MissingKey",
			data: map[string]any{"other": "value"},
			key:  "key",
			msg:  "Missing key should return nil",
		},
		{
			name: "InvalidUnmarshal",
			data: map[string]any{"key": "not_an_array"},
			key:  "key",
			msg:  "Invalid data should return nil",
		},
		{
			name: "MarshalError",
			data: map[string]any{"key": make(chan int)},
			key:  "key",
			msg:  "Marshal error should return nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFromData[approval.Condition](tt.data, tt.key)
			assert.Nil(t, result, tt.msg)
		})
	}
}

// TestGetFlowGraphQueryErrors tests get flow graph query errors scenarios.
func TestGetFlowGraphQueryErrors(t *testing.T) {
	tests := []struct {
		name        string
		flowCode    string
		dropTable   string
		errContains string
	}{
		{
			name:        "NodesQueryError",
			flowCode:    "graph_nodes_err",
			dropTable:   "apv_flow_node",
			errContains: "query nodes",
		},
		{
			name:        "EdgesQueryError",
			flowCode:    "graph_edge_err",
			dropTable:   "apv_flow_edge",
			errContains: "query edges",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db, cleanup := setupTestDB(t)
			defer cleanup()

			pub := publisher.NewEventPublisher()
			svc := NewFlowService(db, pub)

			data, _ := json.Marshal(minimalFlowDefinition())

			flow, err := svc.DeployFlow(ctx, DeployFlowCmd{
				FlowCode:   tt.flowCode,
				FlowName:   "Graph Error Test",
				CategoryID: "cat1",
				Definition: string(data),
				OperatorID: "admin",
			})
			require.NoError(t, err, "Should deploy flow")

			var version approval.FlowVersion
			err = db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
				c.Equals("flow_id", flow.ID)
			}).Scan(ctx)
			require.NoError(t, err, "Should find version")

			err = svc.PublishVersion(ctx, version.ID, "admin")
			require.NoError(t, err, "Should publish version")

			_, err = db.NewRaw("DROP TABLE " + tt.dropTable).Exec(ctx)
			require.NoError(t, err, "Should drop table")

			_, err = svc.GetFlowGraph(ctx, flow.ID)
			require.Error(t, err, "Should fail with dropped table")
			assert.Contains(t, err.Error(), tt.errContains, "Error should contain expected message")
		})
	}
}

// TestDeployFlowExtendedNodeProperties tests deploy flow extended node properties scenarios.
func TestDeployFlowExtendedNodeProperties(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	pub := publisher.NewEventPublisher()
	svc := NewFlowService(db, pub)

	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Type: "start", Data: map[string]any{"label": "Start"}},
			{ID: "approval", Type: "approval", Data: map[string]any{
				"label":                   "Approval",
				"isAddAssigneeAllowed":    true,
				"isRemoveAssigneeAllowed": true,
				"passRatio":               0.5,
				"timeoutHours":            float64(48),
				"adminUserIds":            []any{"admin1"},
				"fallbackUserIds":         []any{"fallback1"},
				"addAssigneeTypes":        []any{"before", "after"},
				"assignees": []any{
					map[string]any{"kind": "user", "ids": []any{"u1"}, "sortOrder": float64(0), "formField": "assignee_field"},
				},
			}},
			{ID: "end", Type: "end", Data: map[string]any{"label": "End"}},
		},
		Edges: []approval.EdgeDefinition{
			{Source: "start", Target: "approval"},
			{Source: "approval", Target: "end"},
		},
	}

	data, _ := json.Marshal(def)
	flow, err := svc.DeployFlow(ctx, DeployFlowCmd{
		FlowCode:   "extended_props",
		FlowName:   "Extended Properties",
		CategoryID: "cat1",
		Definition: string(data),
		OperatorID: "admin",
	})
	require.NoError(t, err, "Should deploy flow")
	require.NotNil(t, flow, "Flow should not be nil")

	var version approval.FlowVersion
	err = db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flow.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should find version")

	var approvalNode approval.FlowNode
	err = db.NewSelect().Model(&approvalNode).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
		c.Equals("node_key", "approval")
	}).Scan(ctx)
	require.NoError(t, err, "Should find approval node")

	assert.True(t, approvalNode.IsAddAssigneeAllowed, "IsAddAssigneeAllowed should be true")
	assert.True(t, approvalNode.IsRemoveAssigneeAllowed, "IsRemoveAssigneeAllowed should be true")
	assert.Equal(t, 48, approvalNode.TimeoutHours, "TimeoutHours should be 48")
	assert.Equal(t, []string{"admin1"}, approvalNode.AdminUserIDs, "AdminUserIDs should match")
	assert.Equal(t, []string{"fallback1"}, approvalNode.FallbackUserIDs, "FallbackUserIDs should match")

	var assignees []approval.FlowNodeAssignee
	err = db.NewSelect().Model(&assignees).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", approvalNode.ID)
	}).Scan(ctx)
	require.NoError(t, err, "Should query assignees")
	require.Len(t, assignees, 1, "Should have one assignee config")
	assert.True(t, assignees[0].FormField.Valid, "FormField should be valid")
	assert.Equal(t, "assignee_field", assignees[0].FormField.String, "FormField should match")
}
