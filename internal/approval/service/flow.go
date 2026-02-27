package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/result"
	"github.com/ilxqx/vef-framework-go/timex"
)

// CreateFlowCmd contains the parameters for creating a new flow.
type CreateFlowCmd struct {
	TenantID              string
	Code                  string
	Name                  string
	CategoryID            string
	Icon                  *string
	Description           *string
	BindingMode           approval.BindingMode
	BusinessTable         *string
	BusinessPkField       *string
	BusinessTitleField    *string
	BusinessStatusField   *string
	AdminUserIDs          []string
	IsAllInitiateAllowed  bool
	InstanceTitleTemplate string
	Initiators            []CreateFlowInitiatorCmd
}

// CreateFlowInitiatorCmd contains the parameters for creating a flow initiator.
type CreateFlowInitiatorCmd struct {
	Kind approval.InitiatorKind
	IDs  []string
}

// DeployFlowCmd contains the parameters for deploying a flow definition.
type DeployFlowCmd struct {
	FlowID     string
	Definition string // JSON of approval.FlowDefinition
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
	publisher *dispatcher.EventPublisher
}

// NewFlowService creates a new FlowService.
func NewFlowService(db orm.DB, pub *dispatcher.EventPublisher) *FlowService {
	return &FlowService{db: db, publisher: pub}
}

// CreateFlow creates a new flow with its initiator configurations.
func (s *FlowService) CreateFlow(ctx context.Context, cmd CreateFlowCmd) (*approval.Flow, error) {
	tenantID := cmd.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	var resultFlow *approval.Flow

	err := s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Check code uniqueness within tenant
		var existing approval.Flow

		err := tx.NewSelect().Model(&existing).Where(func(c orm.ConditionBuilder) {
			c.Equals("tenant_id", tenantID)
			c.Equals("code", cmd.Code)
		}).Scan(ctx)
		if err == nil {
			return ErrFlowCodeExists
		}

		if !result.IsRecordNotFound(err) {
			return fmt.Errorf("query flow by code: %w", err)
		}

		flow := approval.Flow{
			TenantID:              tenantID,
			CategoryID:            cmd.CategoryID,
			Code:                  cmd.Code,
			Name:                  cmd.Name,
			Icon:                  cmd.Icon,
			Description:           cmd.Description,
			BindingMode:           cmd.BindingMode,
			BusinessTable:         cmd.BusinessTable,
			BusinessPkField:       cmd.BusinessPkField,
			BusinessTitleField:    cmd.BusinessTitleField,
			BusinessStatusField:   cmd.BusinessStatusField,
			AdminUserIDs:          cmd.AdminUserIDs,
			IsAllInitiateAllowed:  cmd.IsAllInitiateAllowed,
			InstanceTitleTemplate: cmd.InstanceTitleTemplate,
			IsActive:              true,
			CurrentVersion:        0,
		}
		if _, err := tx.NewInsert().Model(&flow).Exec(ctx); err != nil {
			return fmt.Errorf("insert flow: %w", err)
		}

		// Insert initiators
		for _, init := range cmd.Initiators {
			initiator := approval.FlowInitiator{
				FlowID: flow.ID,
				Kind:   init.Kind,
				IDs:    init.IDs,
			}
			if _, err := tx.NewInsert().Model(&initiator).Exec(ctx); err != nil {
				return fmt.Errorf("insert flow initiator: %w", err)
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

// DeployFlow deploys a flow definition to an existing flow.
// It creates a new version with nodes and edges. The flow must already exist (use CreateFlow first).
func (s *FlowService) DeployFlow(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	var def approval.FlowDefinition
	if err := json.Unmarshal([]byte(cmd.Definition), &def); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFlowDesign, err)
	}

	if err := ValidateFlowDefinition(&def); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFlowDesign, err)
	}

	var resultVersion *approval.FlowVersion

	err := s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		// Load existing flow
		var flow approval.Flow

		if err := tx.NewSelect().Model(&flow).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", cmd.FlowID)
		}).Scan(ctx); err != nil {
			return ErrFlowNotFound
		}

		// Bump version
		flow.CurrentVersion++
		if _, err := tx.NewUpdate().Model(&flow).WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("update flow version: %w", err)
		}

		// Create new version
		version := approval.FlowVersion{
			FlowID:  flow.ID,
			Version: flow.CurrentVersion,
			Status:  approval.VersionDraft,
		}
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
				NodeKind:      nd.Type,
				Name:          name,
			}
			ApplyNodeData(&node, nd.Data)

			if _, err := tx.NewInsert().Model(&node).Exec(ctx); err != nil {
				return fmt.Errorf("insert node: %w", err)
			}

			nodeKeyToID[nd.ID] = node.ID

			// Extract and insert node assignees from data
			assignees := ExtractFromData[approval.AssigneeDefinition](nd.Data, "assignees")
			for _, assigneeDef := range assignees {
				assignee := approval.FlowNodeAssignee{
					NodeID:    node.ID,
					Kind:      approval.AssigneeKind(assigneeDef.Kind),
					IDs:       assigneeDef.IDs,
					SortOrder: assigneeDef.SortOrder,
				}
				if assigneeDef.FormField != "" {
					assignee.FormField = new(assigneeDef.FormField)
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
			if edgeDef.SourceHandle != "" {
				edge.SourceHandle = new(edgeDef.SourceHandle)
			}

			if _, err := tx.NewInsert().Model(&edge).Exec(ctx); err != nil {
				return fmt.Errorf("insert edge: %w", err)
			}
		}

		resultVersion = &version

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resultVersion, nil
}

// PublishVersion publishes a flow version (set status to published, archive old published).
func (s *FlowService) PublishVersion(ctx context.Context, versionID, operatorID string) error {
	return s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var version approval.FlowVersion

		version.ID = versionID
		if err := tx.NewSelect().
			Model(&version).
			WherePK().
			ForUpdate().
			Scan(ctx); err != nil {
			return ErrFlowNotFound
		}

		if version.Status != approval.VersionDraft {
			return ErrVersionNotDraft
		}

		// Archive old published versions
		if _, err := tx.NewUpdate().
			Model((*approval.FlowVersion)(nil)).
			Set("status", approval.VersionArchived).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("flow_id", version.FlowID).
					Equals("status", approval.VersionPublished)
			}).
			Exec(ctx); err != nil {
			return fmt.Errorf("archive old versions: %w", err)
		}

		// Publish this version
		version.Status = approval.VersionPublished
		version.PublishedAt = new(timex.Now())
		version.PublishedBy = new(operatorID)

		if _, err := tx.NewUpdate().
			Model(&version).
			WherePK().
			Exec(ctx); err != nil {
			return fmt.Errorf("publish version: %w", err)
		}

		return s.publisher.PublishAll(ctx, tx, []approval.DomainEvent{
			approval.NewFlowPublishedEvent(version.FlowID, versionID),
		})
	})
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
