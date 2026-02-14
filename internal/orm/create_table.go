package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

type BunCreateTableQuery struct {
	query *bun.CreateTableQuery
}

func NewCreateTableQuery(db *BunDB) *BunCreateTableQuery {
	return &BunCreateTableQuery{
		query: db.db.NewCreateTable(),
	}
}

func (q *BunCreateTableQuery) Model(model any) CreateTableQuery {
	q.query.Model(model)

	return q
}

func (q *BunCreateTableQuery) Table(tables ...string) CreateTableQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunCreateTableQuery) ColumnExpr(query string, args ...any) CreateTableQuery {
	q.query.ColumnExpr(query, args...)

	return q
}

func (q *BunCreateTableQuery) Temp() CreateTableQuery {
	q.query.Temp()

	return q
}

func (q *BunCreateTableQuery) IfNotExists() CreateTableQuery {
	q.query.IfNotExists()

	return q
}

func (q *BunCreateTableQuery) Varchar(n int) CreateTableQuery {
	q.query.Varchar(n)

	return q
}

func (q *BunCreateTableQuery) ForeignKey(query string, args ...any) CreateTableQuery {
	q.query.ForeignKey(query, args...)

	return q
}

func (q *BunCreateTableQuery) PartitionBy(query string, args ...any) CreateTableQuery {
	q.query.PartitionBy(query, args...)

	return q
}

func (q *BunCreateTableQuery) TableSpace(tablespace string) CreateTableQuery {
	q.query.TableSpace(tablespace)

	return q
}

func (q *BunCreateTableQuery) WithForeignKeys() CreateTableQuery {
	q.query.WithForeignKeys()

	return q
}

func (q *BunCreateTableQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}

func (q *BunCreateTableQuery) String() string {
	return q.query.String()
}
