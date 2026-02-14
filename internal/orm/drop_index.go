package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

type BunDropIndexQuery struct {
	query *bun.DropIndexQuery
}

func NewDropIndexQuery(db *BunDB) *BunDropIndexQuery {
	return &BunDropIndexQuery{
		query: db.db.NewDropIndex(),
	}
}

func (q *BunDropIndexQuery) Index(name string, args ...any) DropIndexQuery {
	q.query.Index(name, args...)

	return q
}

func (q *BunDropIndexQuery) IfExists() DropIndexQuery {
	q.query.IfExists()

	return q
}

func (q *BunDropIndexQuery) Concurrently() DropIndexQuery {
	q.query.Concurrently()

	return q
}

func (q *BunDropIndexQuery) Cascade() DropIndexQuery {
	q.query.Cascade()

	return q
}

func (q *BunDropIndexQuery) Restrict() DropIndexQuery {
	q.query.Restrict()

	return q
}

func (q *BunDropIndexQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
