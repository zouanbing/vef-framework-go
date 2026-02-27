package handler

import (
	"context"

	"github.com/ilxqx/vef-framework-go/contextx"
	"github.com/ilxqx/vef-framework-go/orm"
)

// dbFromCtx returns the transaction DB from context, falling back to the provided default.
func dbFromCtx(ctx context.Context, fallback orm.DB) orm.DB {
	if db := contextx.DB(ctx); db != nil {
		return db
	}
	return fallback
}
