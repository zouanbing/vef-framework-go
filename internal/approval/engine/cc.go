package engine

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/orm"
)

// CCProcessor handles CC (carbon copy) notification nodes.
type CCProcessor struct{}

// NewCCProcessor creates a CCProcessor.
func NewCCProcessor() *CCProcessor { return &CCProcessor{} }

func (p *CCProcessor) NodeKind() approval.NodeKind { return approval.NodeCC }

func (p *CCProcessor) Process(ctx context.Context, pc *ProcessContext) (*ProcessResult, error) {
	ccUserIDs, err := p.createCCRecords(ctx, pc)
	if err != nil {
		return nil, err
	}

	var events []approval.DomainEvent
	if len(ccUserIDs) > 0 {
		events = []approval.DomainEvent{
			approval.NewCcNotifiedEvent(pc.Instance.ID, pc.Node.ID, ccUserIDs, false),
		}
	}

	if pc.Node.IsReadConfirmRequired {
		return &ProcessResult{Action: NodeActionWait, Events: events}, nil
	}

	return &ProcessResult{Action: NodeActionContinue, Events: events}, nil
}

// createCCRecords loads FlowNodeCC configurations and creates CC records for all CC users.
// Returns the list of CC user IDs for event publishing.
func (p *CCProcessor) createCCRecords(ctx context.Context, pc *ProcessContext) ([]string, error) {
	if pc.DB == nil {
		return nil, nil
	}

	var ccConfigs []approval.FlowNodeCC

	if err := pc.DB.NewSelect().Model(&ccConfigs).Where(func(c orm.ConditionBuilder) {
		c.Equals("node_id", pc.Node.ID)
	}).Scan(ctx); err != nil {
		return nil, fmt.Errorf("load cc configs: %w", err)
	}

	// Collect all CC user IDs
	var ccUserIDs []string
	for _, cfg := range ccConfigs {
		ccUserIDs = append(ccUserIDs, cfg.IDs...)
	}

	if len(ccUserIDs) == 0 {
		return nil, nil
	}

	// Batch create CC records
	records := make([]approval.CCRecord, 0, len(ccUserIDs))
	for _, userID := range ccUserIDs {
		record := approval.CCRecord{
			InstanceID: pc.Instance.ID,
			NodeID:     new(pc.Node.ID),
			CCUserID:   userID,
			IsManual:   false,
		}
		records = append(records, record)
	}

	if _, err := pc.DB.NewInsert().Model(&records).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert cc records: %w", err)
	}

	return ccUserIDs, nil
}
