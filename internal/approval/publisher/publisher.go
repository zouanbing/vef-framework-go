package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/id"
	"github.com/ilxqx/vef-framework-go/orm"
)

// EventPublisher publishes events to EventOutbox table.
type EventPublisher struct {
	db orm.DB
}

// NewEventPublisher creates a new EventPublisher.
func NewEventPublisher(db orm.DB) *EventPublisher {
	return &EventPublisher{db: db}
}

// PublishAll marshals each event and inserts into the event outbox table.
func (p *EventPublisher) PublishAll(ctx context.Context, db orm.DB, events []approval.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	outboxRecords := make([]approval.EventOutbox, 0, len(events))

	for _, evt := range events {
		payloadMap, err := toMap(evt)
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", evt.EventName(), err)
		}

		outboxRecords = append(outboxRecords, approval.EventOutbox{
			EventID:   id.Generate(),
			EventType: evt.EventName(),
			Payload:   payloadMap,
			Status:    "pending",
		})
	}

	_, err := db.NewInsert().Model(&outboxRecords).Exec(ctx)

	return err
}

// toMap converts a struct to map[string]any via JSON round-trip.
func toMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return m, nil
}
