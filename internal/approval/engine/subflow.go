package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/orm"
)

const maxSubFlowDepth = 20

var (
	ErrNoPublishedVersion = errors.New("no published version found for sub-flow")
	ErrSubFlowCycle       = errors.New("circular sub-flow reference detected")
)

// SubFlowProcessor handles sub-flow nodes.
type SubFlowProcessor struct {
	engine *FlowEngine // Set after engine construction to break circular dependency
}

// NewSubFlowProcessor creates a new sub-flow processor.
func NewSubFlowProcessor() *SubFlowProcessor {
	return &SubFlowProcessor{}
}

// SetFlowEngine sets the flow engine reference (required due to circular dependency).
func (p *SubFlowProcessor) SetFlowEngine(engine *FlowEngine) {
	p.engine = engine
}

func (p *SubFlowProcessor) NodeKind() approval.NodeKind { return approval.NodeSubFlow }

func (p *SubFlowProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	if p.engine == nil {
		return nil, errors.New("FlowEngine not initialized in SubFlowProcessor")
	}

	config := pc.Node.SubFlowConfig
	if config == nil {
		return nil, errors.New("sub flow config is required")
	}

	flowID, _ := config["flowId"].(string)
	if flowID == "" {
		return nil, errors.New("sub flow config missing flowId")
	}

	// 1. Detect circular sub-flow references by traversing the parent chain
	if err := p.detectSubFlowCycle(ctx, pc.DB, pc.Instance, flowID); err != nil {
		return nil, err
	}

	// Find sub-flow and its published version
	var subFlow approval.Flow
	if err := pc.DB.NewSelect().Model(&subFlow).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", flowID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("find sub flow: %w", err)
	}

	var subVersion approval.FlowVersion
	if err := pc.DB.NewSelect().Model(&subVersion).Where(func(c orm.ConditionBuilder) {
		c.Equals("flow_id", subFlow.ID)
		c.Equals("status", string(approval.VersionPublished))
	}).Scan(ctx); err != nil {
		return nil, ErrNoPublishedVersion
	}

	// Prepare sub-flow form data via data mapping
	subFormData := p.prepareSubFormData(pc.FormData, config)

	// Create sub-flow instance
	subInstance := &approval.Instance{
		FlowID:           subFlow.ID,
		FlowVersionID:    subVersion.ID,
		ParentInstanceID: null.StringFrom(pc.Instance.ID),
		ParentNodeID:     null.StringFrom(pc.Node.ID),
		Title:            fmt.Sprintf("%s - SubFlow", pc.Instance.Title),
		SerialNo:         id.Generate(),
		ApplicantID:      pc.ApplicantID,
		ApplicantDeptID:  pc.Instance.ApplicantDeptID,
		Status:           string(approval.InstanceRunning),
		FormData:         subFormData.ToMap(),
	}
	subInstance.ID = id.Generate()
	subInstance.CreatedBy = pc.ApplicantID
	subInstance.UpdatedBy = pc.ApplicantID

	if _, err := pc.DB.NewInsert().Model(subInstance).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create sub-flow instance: %w", err)
	}

	// Start sub-flow process
	if err := p.engine.StartProcess(ctx, pc.DB, subInstance); err != nil {
		return nil, fmt.Errorf("start sub-flow: %w", err)
	}

	// Update sub-flow instance state after start
	if _, err := pc.DB.NewUpdate().Model(subInstance).WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("update sub-flow instance: %w", err)
	}

	// Publish sub-flow started event
	if err := p.engine.publishEvents(ctx, pc.DB,
		approval.NewSubFlowStartedEvent(pc.Instance.ID, subInstance.ID, pc.Node.ID),
	); err != nil {
		return nil, fmt.Errorf("publish sub-flow started event: %w", err)
	}

	return &ProcessResult{Action: NodeActionWait}, nil
}

// detectSubFlowCycle checks if launching a sub-flow with the given flowID would
// create a circular reference. It traverses the ParentInstanceID chain upward and
// checks if any ancestor instance belongs to the same flow.
func (p *SubFlowProcessor) detectSubFlowCycle(ctx context.Context, db orm.DB, current *approval.Instance, targetFlowID string) error {
	// Check the current instance itself
	if current.FlowID == targetFlowID {
		return fmt.Errorf("%w: flow %s", ErrSubFlowCycle, targetFlowID)
	}

	// Traverse upward through parent instances to prevent infinite loops from bad data
	parentID := current.ParentInstanceID
	for range maxSubFlowDepth {
		if !parentID.Valid {
			break
		}

		var parent approval.Instance

		if err := db.NewSelect().Model(&parent).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", parentID.String)
		}).Scan(ctx); err != nil {
			break
		}

		if parent.FlowID == targetFlowID {
			return fmt.Errorf("%w: flow %s", ErrSubFlowCycle, targetFlowID)
		}

		parentID = parent.ParentInstanceID
	}

	return nil
}

func (p *SubFlowProcessor) prepareSubFormData(parentData approval.FormData, config map[string]any) approval.FormData {
	result := approval.NewFormData(nil)

	dataMapping, ok := config["dataMapping"].([]any)
	if !ok {
		return result
	}

	for _, item := range dataMapping {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		sourceField, _ := m["sourceField"].(string)
		targetField, _ := m["targetField"].(string)

		if sourceField == "" || targetField == "" {
			continue
		}

		if value := parentData.Get(sourceField); value != nil {
			result.Set(targetField, value)
		}
	}

	return result
}
