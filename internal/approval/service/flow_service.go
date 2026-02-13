package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/constants"
	"github.com/ilxqx/vef-framework-go/datetime"
	"github.com/ilxqx/vef-framework-go/decimal"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/internal/approval/publisher"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
)

// DeployFlowCmd contains the parameters for deploying a flow definition.
type DeployFlowCmd struct {
	FlowCode   string
	FlowName   string
	CategoryID string
	Definition string // JSON of approval.FlowDefinition
	OperatorID string
}

// FlowGraph contains the complete flow graph for a version.
type FlowGraph struct {
	Flow    *approval.Flow        `json:"flow"`
	Version *approval.FlowVersion `json:"version"`
	Nodes   []approval.FlowNode   `json:"nodes"`
	Edges   []approval.FlowEdge   `json:"edges"`
}

// FlowService manages flow definitions and versions.
type FlowService struct {
	db        orm.DB
	publisher *publisher.EventPublisher
}

// NewFlowService creates a new FlowService.
func NewFlowService(db orm.DB, pub *publisher.EventPublisher) *FlowService {
	return &FlowService{db: db, publisher: pub}
}

// DeployFlow deploys a flow definition (create/update flow + version + nodes + edges).
func (s *FlowService) DeployFlow(ctx context.Context, cmd DeployFlowCmd) (*approval.Flow, error) {
	var def approval.FlowDefinition
	if err := json.Unmarshal([]byte(cmd.Definition), &def); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFlowDesign, err)
	}

	if err := validateFlowDefinition(&def); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFlowDesign, err)
	}

	var resultFlow *approval.Flow

	err := s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Load or create flow
		var flow approval.Flow

		err := tx.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
			c.Equals("code", cmd.FlowCode)
		}).Scan(ctx)
		if err != nil && !result.IsRecordNotFound(err) {
			return fmt.Errorf("query flow by code: %w", err)
		}

		isNewFlow := result.IsRecordNotFound(err)

		if isNewFlow {
			flow = approval.Flow{
				TenantID:             "default",
				CategoryID:           cmd.CategoryID,
				Code:                 cmd.FlowCode,
				Name:                 cmd.FlowName,
				IsActive:             true,
				IsAllInitiateAllowed: true,
				CurrentVersion:       1,
			}
			flow.ID = id.Generate()
			flow.CreatedBy = cmd.OperatorID
			flow.UpdatedBy = cmd.OperatorID

			if _, err := tx.NewInsert().Model(&flow).Exec(ctx); err != nil {
				return fmt.Errorf("insert flow: %w", err)
			}
		} else {
			flow.CurrentVersion++
			flow.Name = cmd.FlowName
			flow.UpdatedBy = cmd.OperatorID

			if _, err := tx.NewUpdate().Model(&flow).WherePK().Exec(ctx); err != nil {
				return fmt.Errorf("update flow: %w", err)
			}
		}

		// Create new version
		version := approval.FlowVersion{
			FlowID:  flow.ID,
			Version: flow.CurrentVersion,
			Status:  approval.VersionDraft,
		}
		version.ID = id.Generate()
		version.CreatedBy = cmd.OperatorID
		version.UpdatedBy = cmd.OperatorID

		if _, err := tx.NewInsert().Model(&version).Exec(ctx); err != nil {
			return fmt.Errorf("insert version: %w", err)
		}

		// Create nodes and build nodeKey → nodeID mapping
		nodeKeyToID := make(map[string]string, len(def.Nodes))

		for _, nd := range def.Nodes {
			// Extract label from data
			var name string
			if nd.Data != nil {
				if v, ok := nd.Data["label"].(string); ok {
					name = v
				}
			}

			node := approval.FlowNode{
				FlowVersionID: version.ID,
				NodeKey:       nd.ID,
				NodeKind:      approval.NodeKind(nd.Type),
				Name:          name,
			}
			node.ID = id.Generate()
			node.CreatedBy = cmd.OperatorID
			node.UpdatedBy = cmd.OperatorID

			applyNodeData(&node, nd.Data)

			if _, err := tx.NewInsert().Model(&node).Exec(ctx); err != nil {
				return fmt.Errorf("insert node: %w", err)
			}

			nodeKeyToID[nd.ID] = node.ID

			// Extract and insert node assignees from data
			assignees := extractFromData[approval.AssigneeDefinition](nd.Data, "assignees")
			for _, assigneeDef := range assignees {
				assignee := approval.FlowNodeAssignee{
					NodeID:       node.ID,
					AssigneeKind: approval.AssigneeKind(assigneeDef.Kind),
					AssigneeIDs:  assigneeDef.IDs,
					SortOrder:    assigneeDef.SortOrder,
				}
				assignee.ID = id.Generate()

				if assigneeDef.FormField != constants.Empty {
					assignee.FormField = null.StringFrom(assigneeDef.FormField)
				}

				if _, err := tx.NewInsert().Model(&assignee).Exec(ctx); err != nil {
					return fmt.Errorf("insert node assignee: %w", err)
				}
			}
		}

		// Create edges using real node IDs
		for _, edgeDef := range def.Edges {
			sourceID, ok := nodeKeyToID[edgeDef.Source]
			if !ok {
				return fmt.Errorf("%w: unknown source node key %q", ErrInvalidFlowDesign, edgeDef.Source)
			}

			targetID, ok := nodeKeyToID[edgeDef.Target]
			if !ok {
				return fmt.Errorf("%w: unknown target node key %q", ErrInvalidFlowDesign, edgeDef.Target)
			}

			edge := approval.FlowEdge{
				FlowVersionID: version.ID,
				SourceNodeID:  sourceID,
				TargetNodeID:  targetID,
			}
			edge.ID = id.Generate()

			if edgeDef.SourceHandle != "" {
				edge.SourceHandle = null.StringFrom(edgeDef.SourceHandle)
			}

			if _, err := tx.NewInsert().Model(&edge).Exec(ctx); err != nil {
				return fmt.Errorf("insert edge: %w", err)
			}
		}

		resultFlow = &flow

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resultFlow, nil
}

// PublishVersion publishes a flow version (set status to published, archive old published).
func (s *FlowService) PublishVersion(ctx context.Context, versionID, operatorID string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var version approval.FlowVersion

		if err := tx.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", versionID)
		}).Scan(ctx); err != nil {
			return ErrFlowNotFound
		}

		if version.Status != approval.VersionDraft {
			return ErrVersionNotDraft
		}

		// Archive old published versions
		if _, err := tx.NewUpdate().Model((*approval.FlowVersion)(nil)).
			Set("status", string(approval.VersionArchived)).
			Where(func(c orm.ConditionBuilder) {
				c.Equals("flow_id", version.FlowID)
				c.Equals("status", string(approval.VersionPublished))
			}).Exec(ctx); err != nil {
			return fmt.Errorf("archive old versions: %w", err)
		}

		// Publish this version
		now := datetime.Now()
		version.Status = approval.VersionPublished
		version.PublishedAt = null.DateTimeFrom(now)
		version.PublishedBy = null.StringFrom(operatorID)

		if _, err := tx.NewUpdate().Model(&version).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("publish version: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewFlowPublishedEvent(version.FlowID, versionID),
		})
	})
}

// applyNodeData maps design-time data to FlowNode fields.
func applyNodeData(node *approval.FlowNode, data map[string]any) {
	if len(data) == 0 {
		return
	}

	if v, ok := data["approvalMethod"].(string); ok {
		node.ApprovalMethod = approval.ApprovalMethod(v)
	}

	if v, ok := data["passRule"].(string); ok {
		node.PassRule = approval.PassRule(v)
	}

	if v, ok := data["executionType"].(string); ok {
		node.ExecutionType = approval.ExecutionType(v)
	}

	if v, ok := data["emptyHandlerAction"].(string); ok {
		node.EmptyHandlerAction = approval.EmptyHandlerAction(v)
	}

	if v, ok := data["sameApplicantAction"].(string); ok {
		node.SameApplicantAction = approval.SameApplicantAction(v)
	}

	if v, ok := data["duplicateHandlerAction"].(string); ok {
		node.DuplicateHandlerAction = approval.DuplicateHandlerAction(v)
	}

	if v, ok := data["rollbackType"].(string); ok {
		node.RollbackType = approval.RollbackType(v)
	}

	if v, ok := data["rollbackDataStrategy"].(string); ok {
		node.RollbackDataStrategy = approval.RollbackDataStrategy(v)
	}

	if v, ok := data["isRollbackAllowed"].(bool); ok {
		node.IsRollbackAllowed = v
	}

	if v, ok := data["isAddAssigneeAllowed"].(bool); ok {
		node.IsAddAssigneeAllowed = v
	}

	if v, ok := data["isRemoveAssigneeAllowed"].(bool); ok {
		node.IsRemoveAssigneeAllowed = v
	}

	if v, ok := data["isTransferAllowed"].(bool); ok {
		node.IsTransferAllowed = v
	}

	if v, ok := data["isOpinionRequired"].(bool); ok {
		node.IsOpinionRequired = v
	}

	if v, ok := data["isManualCCAllowed"].(bool); ok {
		node.IsManualCCAllowed = v
	}

	if v, ok := data["passRatio"]; ok {
		switch r := v.(type) {
		case float64:
			node.PassRatio = decimal.NewFromFloat(r)
		case string:
			if d, err := decimal.NewFromString(r); err == nil {
				node.PassRatio = d
			}
		}
	}

	if v, ok := data["timeoutHours"]; ok {
		if f, ok := v.(float64); ok {
			node.TimeoutHours = int(f)
		}
	}

	if v, ok := data["adminUserIds"]; ok {
		if ids, ok := toStringSlice(v); ok {
			node.AdminUserIDs = ids
		}
	}

	if v, ok := data["fallbackUserIds"]; ok {
		if ids, ok := toStringSlice(v); ok {
			node.FallbackUserIDs = ids
		}
	}

	if v, ok := data["addAssigneeTypes"]; ok {
		if ids, ok := toStringSlice(v); ok {
			node.AddAssigneeTypes = ids
		}
	}

	if v, ok := data["subFlowConfig"].(map[string]any); ok {
		node.SubFlowConfig = v
	}

	if v, ok := data["fieldPermissions"].(map[string]any); ok {
		node.FieldPermissions = v
	}

	// Extract condition branches for condition nodes
	branches := extractFromData[approval.ConditionBranch](data, "branches")
	if len(branches) > 0 {
		node.Branches = branches
	}
}

// GetFlowGraph returns nodes and edges for a flow's published version.
func (s *FlowService) GetFlowGraph(ctx context.Context, flowID string) (*FlowGraph, error) {
	var flow approval.Flow

	if err := s.db.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", flowID)
	}).Scan(ctx); err != nil {
		return nil, ErrFlowNotFound
	}

	// Find published version
	var version approval.FlowVersion

	if err := s.db.NewSelect().Model(&version).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", flowID)
		c.Equals("status", string(approval.VersionPublished))
	}).Scan(ctx); err != nil {
		return nil, ErrNoPublishedVersion
	}

	var nodes []approval.FlowNode

	if err := s.db.NewSelect().Model(&nodes).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}

	var edges []approval.FlowEdge

	if err := s.db.NewSelect().Model(&edges).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_version_id", version.ID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}

	return &FlowGraph{
		Flow:    &flow,
		Version: &version,
		Nodes:   nodes,
		Edges:   edges,
	}, nil
}

// validNodeKinds defines the set of valid node kinds for flow validation.
var validNodeKinds = map[approval.NodeKind]struct{}{
	approval.NodeStart:     {},
	approval.NodeEnd:       {},
	approval.NodeApproval:  {},
	approval.NodeHandle:    {},
	approval.NodeCondition: {},
	approval.NodeSubFlow:   {},
}

// validateFlowDefinition validates the structural integrity of a flow definition.
func validateFlowDefinition(def *approval.FlowDefinition) error {
	if len(def.Nodes) == 0 {
		return fmt.Errorf("flow must have at least one node")
	}

	var startCount, endCount int

	nodeIDs := make(map[string]struct{}, len(def.Nodes))

	for _, nd := range def.Nodes {
		if nd.ID == constants.Empty {
			return fmt.Errorf("node ID must not be empty")
		}

		if _, dup := nodeIDs[nd.ID]; dup {
			return fmt.Errorf("duplicate node ID %q", nd.ID)
		}

		nodeIDs[nd.ID] = struct{}{}

		kind := approval.NodeKind(nd.Type)
		if _, ok := validNodeKinds[kind]; !ok {
			return fmt.Errorf("invalid node kind %q for node %q", nd.Type, nd.ID)
		}

		switch kind {
		case approval.NodeStart:
			startCount++
		case approval.NodeEnd:
			endCount++
		case approval.NodeSubFlow:
			var config map[string]any
			if nd.Data != nil {
				config, _ = nd.Data["subFlowConfig"].(map[string]any)
			}

			if config == nil {
				return fmt.Errorf("sub_flow node %q missing subFlowConfig", nd.ID)
			}

			flowID, _ := config["flowId"].(string)
			if flowID == constants.Empty {
				return fmt.Errorf("sub_flow node %q missing flowId in subFlowConfig", nd.ID)
			}
		}
	}

	if startCount != 1 {
		return fmt.Errorf("flow must have exactly 1 start node, found %d", startCount)
	}

	if endCount < 1 {
		return fmt.Errorf("flow must have at least 1 end node, found %d", endCount)
	}

	// Validate all edge references point to existing nodes
	for _, edge := range def.Edges {
		if _, ok := nodeIDs[edge.Source]; !ok {
			return fmt.Errorf("line references unknown source node %q", edge.Source)
		}

		if _, ok := nodeIDs[edge.Target]; !ok {
			return fmt.Errorf("line references unknown target node %q", edge.Target)
		}
	}

	return nil
}

// extractFromData extracts a typed slice from a map key via JSON round-trip.
func extractFromData[T any](data map[string]any, key string) []T {
	if data == nil {
		return nil
	}

	raw, ok := data[key]
	if !ok {
		return nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var result []T
	if err := json.Unmarshal(b, &result); err != nil {
		return nil
	}

	return result
}

// toStringSlice converts a JSON-decoded []any to []string.
func toStringSlice(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}

	result := make([]string, 0, len(arr))

	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}

	return result, len(result) > 0
}
