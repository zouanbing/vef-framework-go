package handler

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/internal/approval/dispatcher"
	"github.com/ilxqx/vef-framework-go/internal/approval/service"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// PublishVersionCmd publishes a flow version.
type PublishVersionCmd struct {
	cqrs.CommandBase
	VersionID  string
	OperatorID string
}

// PublishVersionHandler handles the PublishVersionCmd command.
type PublishVersionHandler struct {
	db        orm.DB
	publisher *dispatcher.EventPublisher
}

// NewPublishVersionHandler creates a new PublishVersionHandler.
func NewPublishVersionHandler(db orm.DB, pub *dispatcher.EventPublisher) *PublishVersionHandler {
	return &PublishVersionHandler{db: db, publisher: pub}
}

func (h *PublishVersionHandler) Handle(ctx context.Context, cmd PublishVersionCmd) (cqrs.Unit, error) {
	db := dbFromCtx(ctx, h.db)

	var version approval.FlowVersion
	version.ID = cmd.VersionID
	if err := db.NewSelect().
		Model(&version).
		WherePK().
		ForUpdate().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, service.ErrFlowNotFound
	}

	if version.Status != approval.VersionDraft {
		return cqrs.Unit{}, service.ErrVersionNotDraft
	}

	// Archive old published versions
	if _, err := db.NewUpdate().
		Model((*approval.FlowVersion)(nil)).
		Set("status", approval.VersionArchived).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", version.FlowID).
				Equals("status", approval.VersionPublished)
		}).
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("archive old versions: %w", err)
	}

	// Publish this version
	version.Status = approval.VersionPublished
	version.PublishedAt = new(timex.Now())
	version.PublishedBy = new(cmd.OperatorID)

	if _, err := db.NewUpdate().
		Model(&version).
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("publish version: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewFlowPublishedEvent(version.FlowID, cmd.VersionID),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
