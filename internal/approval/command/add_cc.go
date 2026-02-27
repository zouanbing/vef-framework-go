package command

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/shared"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// AddCCCmd adds CC records for an instance.
type AddCCCmd struct {
	cqrs.BaseCommand
	InstanceID string
	CCUserIDs  []string
	OperatorID string
}

// AddCCHandler handles the AddCCCmd command.
type AddCCHandler struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewAddCCHandler creates a new AddCCHandler.
func NewAddCCHandler(db orm.DB, pub *dispatcher.EventPublisher) *AddCCHandler {
	return &AddCCHandler{db: db, publisher: pub}
}

func (h *AddCCHandler) Handle(ctx context.Context, cmd AddCCCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance
	if err := db.NewSelect().Model(&instance).Where(func(c orm.ConditionBuilder) {
		c.Equals("id", cmd.InstanceID)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrInstanceNotFound
	}

	// Validate manual CC is allowed on current node
	if instance.CurrentNodeID != nil {
		var node approval.FlowNode
		if err := db.NewSelect().Model(&node).Where(func(c orm.ConditionBuilder) {
			c.Equals("id", *instance.CurrentNodeID)
		}).Scan(ctx); err == nil && !node.IsManualCCAllowed {
			return cqrs.Unit{}, shared.ErrManualCcNotAllowed
		}
	}

	// Filter out already-existing CC records to avoid duplicates
	var existingCCs []approval.CCRecord
	if err := db.NewSelect().Model(&existingCCs).Where(func(c orm.ConditionBuilder) {
		c.Equals("instance_id", cmd.InstanceID)
		c.In("cc_user_id", cmd.CCUserIDs)
	}).Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("query existing cc records: %w", err)
	}

	existingSet := make(map[string]struct{}, len(existingCCs))
	for _, cc := range existingCCs {
		existingSet[cc.CCUserID] = struct{}{}
	}

	records := make([]approval.CCRecord, 0, len(cmd.CCUserIDs))
	for _, userID := range cmd.CCUserIDs {
		if _, exists := existingSet[userID]; exists {
			continue
		}
		records = append(records, approval.CCRecord{
			InstanceID: cmd.InstanceID,
			CCUserID:   userID,
			IsManual:   true,
		})
	}

	if len(records) == 0 {
		return cqrs.Unit{}, nil
	}

	if _, err := db.NewInsert().Model(&records).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert cc records: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewCcNotifiedEvent(cmd.InstanceID, "", cmd.CCUserIDs, true),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
