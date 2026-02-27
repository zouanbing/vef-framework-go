package dispatcher

import (
	"context"
	"math"
	"time"

	"github.com/ilxqx/vef-framework-go/approval"
	"github.com/ilxqx/vef-framework-go/config"
	"github.com/ilxqx/vef-framework-go/orm"
	"github.com/ilxqx/vef-framework-go/timex"
)

// Relay polls the event outbox table and dispatches pending events.
type Relay struct {
	db         orm.DB
	dispatcher approval.EventDispatcher
	cfg        *config.ApprovalConfig
}

// NewRelay creates a new Relay.
func NewRelay(db orm.DB, dispatcher approval.EventDispatcher, cfg *config.ApprovalConfig) *Relay {
	return &Relay{db: db, dispatcher: dispatcher, cfg: cfg}
}

// RelayPending polls pending and retryable events from the outbox and dispatches them.
func (r *Relay) RelayPending(ctx context.Context) {
	batchSize := r.cfg.OutboxBatchSizeOrDefault()
	maxRetries := r.cfg.OutboxMaxRetriesOrDefault()

	var records []approval.EventOutbox

	if err := r.db.NewSelect().Model(&records).
		Where(func(cb orm.ConditionBuilder) {
			cb.Group(func(cb orm.ConditionBuilder) {
				cb.Equals("status", string(approval.EventOutboxPending)).
					IsNull("retry_after")
			}).OrGroup(func(cb orm.ConditionBuilder) {
				cb.Equals("status", string(approval.EventOutboxFailed)).
					LessThan("retry_count", maxRetries).
					LessThanOrEqual("retry_after", timex.Now())
			})
		}).
		OrderBy("created_at").
		Limit(batchSize).
		ForUpdateSkipLocked().
		Scan(ctx); err != nil {
		logger.Errorf("Failed to poll outbox events: %v", err)
		return
	}

	if len(records) == 0 {
		return
	}

	logger.Infof("Relaying %d outbox events", len(records))

	for i := range records {
		if err := r.dispatchOne(ctx, &records[i]); err != nil {
			logger.Errorf("Failed to dispatch event %s: %v", records[i].EventID, err)
		}
	}
}

// dispatchOne dispatches a single outbox record and updates its status.
func (r *Relay) dispatchOne(ctx context.Context, record *approval.EventOutbox) error {
	now := timex.Now()

	if err := r.dispatcher.Dispatch(ctx, *record); err != nil {
		return r.markFailed(ctx, record, err, now)
	}

	return r.markCompleted(ctx, record, now)
}

// markCompleted marks an outbox record as completed.
func (r *Relay) markCompleted(ctx context.Context, record *approval.EventOutbox, now timex.DateTime) error {
	record.Status = approval.EventOutboxCompleted
	record.ProcessedAt = &now

	_, err := r.db.NewUpdate().
		Model(record).
		WherePK().
		Select("status", "processed_at").
		Exec(ctx)

	return err
}

// markFailed marks an outbox record as failed with exponential backoff retry scheduling.
func (r *Relay) markFailed(ctx context.Context, record *approval.EventOutbox, dispatchErr error, now timex.DateTime) error {
	record.Status = approval.EventOutboxFailed
	record.RetryCount++
	record.LastError = new(dispatchErr.Error())

	maxRetries := r.cfg.OutboxMaxRetriesOrDefault()
	if record.RetryCount < maxRetries {
		backoff := time.Duration(math.Pow(2, float64(record.RetryCount))) * time.Second
		record.RetryAfter = new(now.Add(backoff))
	} else {
		record.RetryAfter = nil
	}

	_, err := r.db.NewUpdate().
		Model(record).
		WherePK().
		Select("status", "retry_count", "last_error", "retry_after").
		Exec(ctx)

	return err
}
