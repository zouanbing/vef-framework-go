package command

import (
	"context"
	"fmt"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
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
func NewAddCCHandler(db orm.DB, publisher *dispatcher.EventPublisher) *AddCCHandler {
	return &AddCCHandler{db: db, publisher: publisher}
}

func (h *AddCCHandler) Handle(ctx context.Context, cmd AddCCCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance
	instance.ID = cmd.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		Select("current_node_id").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, shared.ErrInstanceNotFound
	}

	// Validate manual CC is allowed on current node
	if instance.CurrentNodeID != nil {
		var node approval.FlowNode
		node.ID = *instance.CurrentNodeID

		if err := db.NewSelect().
			Model(&node).
			Select("is_manual_cc_allowed").
			WherePK().
			Scan(ctx); err == nil && !node.IsManualCCAllowed {
			return cqrs.Unit{}, shared.ErrManualCcNotAllowed
		}
	}

	// Filter out already-existing CC records to avoid duplicates
	var existingCCs []approval.CCRecord

	if err := db.NewSelect().
		Model(&existingCCs).
		Select("cc_user_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", cmd.InstanceID).
				In("cc_user_id", cmd.CCUserIDs)
		}).
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("query existing cc records: %w", err)
	}

	existingSet := collections.NewHashSet[string]()
	for _, cc := range existingCCs {
		existingSet.Add(cc.CCUserID)
	}

	records := make([]approval.CCRecord, 0, len(cmd.CCUserIDs))
	for _, userID := range cmd.CCUserIDs {
		if existingSet.Contains(userID) {
			continue
		}
		records = append(records, approval.CCRecord{
			InstanceID: cmd.InstanceID,
			NodeID:     instance.CurrentNodeID,
			CCUserID:   userID,
			IsManual:   true,
		})
	}

	if len(records) == 0 {
		return cqrs.Unit{}, nil
	}

	if _, err := db.NewInsert().
		Model(&records).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert cc records: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewCCNotifiedEvent(cmd.InstanceID, *instance.CurrentNodeID, cmd.CCUserIDs, true),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
