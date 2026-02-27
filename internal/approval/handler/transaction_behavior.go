package handler

import (
	"context"

	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/internal/cqrs"
	"github.com/ilxqx/vef-framework-go/orm"
)

// TransactionBehavior wraps command handlers in a database transaction.
// Query handlers bypass the transaction.
type TransactionBehavior struct {
	db orm.DB
}

// NewTransactionBehavior creates a new TransactionBehavior.
func NewTransactionBehavior(db orm.DB) *TransactionBehavior {
	return &TransactionBehavior{db: db}
}

func (b *TransactionBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	var res any
	err := b.db.RunInTX(ctx, func(txCtx context.Context, tx orm.DB) error {
		txCtx = contextx.SetDB(txCtx, tx)
		var txErr error
		res, txErr = next(txCtx)
		return txErr
	})

	return res, err
}
