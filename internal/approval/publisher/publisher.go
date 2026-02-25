package publisher

import (
	"context"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/mapx"
	"github.com/ilxqx/vef-framework-go/orm"
)

// EventPublisher publishes events to EventOutbox table.
type EventPublisher struct{}

// NewEventPublisher creates a new EventPublisher.
func NewEventPublisher() *EventPublisher {
	return &EventPublisher{}
}

// PublishAll marshals each event and inserts into the event outbox table.
func (p *EventPublisher) PublishAll(ctx context.Context, db orm.DB, events []approval.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	outboxRecords := make([]approval.EventOutbox, 0, len(events))

	for _, evt := range events {
		payloadMap, err := mapx.ToMap(evt)
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", evt.EventName(), err)
		}

		outboxRecords = append(outboxRecords, approval.EventOutbox{
			EventID:   id.Generate(),
			EventType: evt.EventName(),
			Payload:   payloadMap,
			Status:    approval.EventOutboxPending,
		})
	}

	_, err := db.NewInsert().Model(&outboxRecords).Exec(ctx)

	return err
}
