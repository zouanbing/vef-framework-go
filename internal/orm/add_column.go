package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

type BunAddColumnQuery struct {
	query *bun.AddColumnQuery
}

func NewAddColumnQuery(db *BunDB) *BunAddColumnQuery {
	return &BunAddColumnQuery{
		query: db.db.NewAddColumn(),
	}
}

func (q *BunAddColumnQuery) Model(model any) AddColumnQuery {
	q.query.Model(model)

	return q
}

func (q *BunAddColumnQuery) Table(tables ...string) AddColumnQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunAddColumnQuery) ColumnExpr(query string, args ...any) AddColumnQuery {
	q.query.ColumnExpr(query, args...)

	return q
}

func (q *BunAddColumnQuery) IfNotExists() AddColumnQuery {
	q.query.IfNotExists()

	return q
}

func (q *BunAddColumnQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
