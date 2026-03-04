package engine

import (
	"context"
	"fmt"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
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
			approval.NewCCNotifiedEvent(pc.Instance.ID, pc.Node.ID, ccUserIDs, false),
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
	var ccConfigs []approval.FlowNodeCC

	if err := pc.DB.NewSelect().
		Model(&ccConfigs).
		Select("ids").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", pc.Node.ID)
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load cc configs: %w", err)
	}

	seen := collections.NewHashSet[string]()
	var ccUserIDs []string

	for _, cfg := range ccConfigs {
		for _, id := range cfg.IDs {
			if seen.Add(id) {
				ccUserIDs = append(ccUserIDs, id)
			}
		}
	}

	if len(ccUserIDs) == 0 {
		return nil, nil
	}

	records := make([]approval.CCRecord, len(ccUserIDs))
	for i, userID := range ccUserIDs {
		records[i] = approval.CCRecord{
			InstanceID: pc.Instance.ID,
			NodeID:     &pc.Node.ID,
			CCUserID:   userID,
			IsManual:   false,
		}
	}

	if _, err := pc.DB.NewInsert().Model(&records).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert cc records: %w", err)
	}

	return ccUserIDs, nil
}
