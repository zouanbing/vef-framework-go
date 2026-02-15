package orm

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/ilxqx/vef-framework-go/page"
)

// DBAccessor provides access to the underlying DB instance.
type DBAccessor interface {
	DB() DB
}

// Executor defines the Exec method shared by all DML and DDL queries.
type Executor interface {
	Exec(ctx context.Context, dest ...any) (sql.Result, error)
}

// QueryExecutor extends Executor with Scan for reading result rows.
type QueryExecutor interface {
	Executor

	Scan(ctx context.Context, dest ...any) error
}

// CTE defines methods for creating Common Table Expressions.
type CTE[T Executor] interface {
	With(name string, builder func(query SelectQuery)) T
	WithValues(name string, model any, withOrder ...bool) T
	WithRecursive(name string, builder func(query SelectQuery)) T
}

// Selectable defines column selection methods (include/exclude).
type Selectable[T Executor] interface {
	SelectAll() T
	Select(columns ...string) T
	Exclude(columns ...string) T
	ExcludeAll() T
}

// TableSource defines methods for specifying table sources.
// Supports model-based and raw table references with optional aliases.
type TableSource[T Executor] interface {
	// Model sets the primary table with automatic name and alias resolution from model structure.
	Model(model any) T
	// ModelTable overrides the table name and alias resolved by Model. Must be called after Model.
	ModelTable(name string, alias ...string) T
	Table(name string, alias ...string) T
	// TableFrom auto-resolves table name and alias from model. The alias parameter takes precedence.
	TableFrom(model any, alias ...string) T
	TableExpr(builder func(ExprBuilder) any, alias ...string) T
	TableSubQuery(builder func(query SelectQuery), alias ...string) T
}

// JoinOperations defines all standard SQL join types (INNER, LEFT, RIGHT, FULL, CROSS)
// with model, table name, subquery, and expression source variants.
type JoinOperations[T any] interface {
	Join(model any, builder func(ConditionBuilder), alias ...string) T
	JoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	JoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	JoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	LeftJoin(model any, builder func(ConditionBuilder), alias ...string) T
	LeftJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	LeftJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	LeftJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	RightJoin(model any, builder func(ConditionBuilder), alias ...string) T
	RightJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	RightJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	RightJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	FullJoin(model any, builder func(ConditionBuilder), alias ...string) T
	FullJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	FullJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	FullJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	CrossJoin(model any, alias ...string) T
	CrossJoinTable(name string, alias ...string) T
	CrossJoinSubQuery(sqBuilder func(query SelectQuery), alias ...string) T
	CrossJoinExpr(eBuilder func(ExprBuilder) any, alias ...string) T
}

// Filterable defines WHERE clause methods including soft delete support.
type Filterable[T Executor] interface {
	Where(func(ConditionBuilder)) T
	WherePK(columns ...string) T
	WhereDeleted() T
	IncludeDeleted() T
}

// Orderable defines ORDER BY methods for query results.
type Orderable[T Executor] interface {
	OrderBy(columns ...string) T
	OrderByDesc(columns ...string) T
	OrderByExpr(func(ExprBuilder) any) T
}

// Limitable defines the LIMIT clause for result set size control.
type Limitable[T Executor] interface {
	Limit(limit int) T
}

// ColumnUpdatable defines methods for setting column values in queries.
type ColumnUpdatable[T Executor] interface {
	Column(name string, value any) T
	ColumnExpr(name string, builder func(ExprBuilder) any) T
}

// Returnable defines RETURNING clause methods for INSERT, UPDATE, and DELETE queries.
type Returnable[T Executor] interface {
	Returning(columns ...string) T
	ReturningAll() T
	ReturningNone() T
}

// ApplyFunc is a reusable query modification function.
type ApplyFunc[T any] func(T)

// Applier enables applying reusable query modifications, optionally conditioned on a boolean.
type Applier[T any] interface {
	Apply(fns ...ApplyFunc[T]) T
	ApplyIf(condition bool, fns ...ApplyFunc[T]) T
}

// DialectExprBuilder is a callback that returns a QueryAppender for dialect-specific expressions.
type DialectExprBuilder func() schema.QueryAppender

// DialectExprs maps database dialects to expression builders for cross-database compatibility.
// The Default builder is used as fallback when no dialect-specific builder is set.
type DialectExprs struct {
	Oracle    DialectExprBuilder
	SQLServer DialectExprBuilder
	Postgres  DialectExprBuilder
	MySQL     DialectExprBuilder
	SQLite    DialectExprBuilder
	Default   DialectExprBuilder
}

// DialectAction is a zero-argument callback for dialect-specific side effects.
type DialectAction func()

// DialectExecs maps database dialects to side-effect callbacks.
type DialectExecs struct {
	Oracle    DialectAction
	SQLServer DialectAction
	Postgres  DialectAction
	MySQL     DialectAction
	SQLite    DialectAction
	Default   DialectAction
}

// DialectActionErr is a callback that can return an error.
type DialectActionErr func() error

// DialectExecsWithErr maps database dialects to callbacks that may return an error.
type DialectExecsWithErr struct {
	Oracle    DialectActionErr
	SQLServer DialectActionErr
	Postgres  DialectActionErr
	MySQL     DialectActionErr
	SQLite    DialectActionErr
	Default   DialectActionErr
}

// DialectFragmentBuilder is a callback that returns a query fragment buffer.
type DialectFragmentBuilder func() ([]byte, error)

// DialectFragments maps database dialects to query fragment builders.
type DialectFragments struct {
	Oracle    DialectFragmentBuilder
	SQLServer DialectFragmentBuilder
	Postgres  DialectFragmentBuilder
	MySQL     DialectFragmentBuilder
	SQLite    DialectFragmentBuilder
	Default   DialectFragmentBuilder
}

// QueryBuilder defines the common interface for building subqueries and conditions.
type QueryBuilder interface {
	fmt.Stringer

	Dialect() schema.Dialect
	GetTable() *schema.Table
	Query() bun.Query
	ExprBuilder() ExprBuilder
	CreateSubQuery(subQuery *bun.SelectQuery) SelectQuery
	BuildSubQuery(builder func(query SelectQuery)) *bun.SelectQuery
	BuildCondition(builder func(ConditionBuilder)) interface {
		schema.QueryAppender
		ConditionBuilder
	}
}

// ExprBuilder provides a fluent API for building SQL expressions including
// aggregates, window functions, string/date/math functions, and conditional logic.
type ExprBuilder interface {
	// Column builds a column expression. If withTableAlias is false, skips automatic table alias.
	Column(column string, withTableAlias ...bool) schema.QueryAppender
	TableColumns(withTableAlias ...bool) schema.QueryAppender
	AllColumns(tableAlias ...string) schema.QueryAppender
	Null() schema.QueryAppender
	IsNull(expr any) schema.QueryAppender
	IsNotNull(expr any) schema.QueryAppender
	Literal(value any) schema.QueryAppender
	Order(func(OrderBuilder)) schema.QueryAppender
	Case(func(CaseBuilder)) schema.QueryAppender
	SubQuery(func(SelectQuery)) schema.QueryAppender
	Exists(func(SelectQuery)) schema.QueryAppender
	NotExists(func(SelectQuery)) schema.QueryAppender
	Paren(expr any) schema.QueryAppender
	Not(expr any) schema.QueryAppender
	Any(func(SelectQuery)) schema.QueryAppender
	All(func(SelectQuery)) schema.QueryAppender

	// ========== Arithmetic Operators ==========

	Add(left, right any) schema.QueryAppender
	Subtract(left, right any) schema.QueryAppender
	Multiply(left, right any) schema.QueryAppender
	Divide(left, right any) schema.QueryAppender

	// ========== Comparison Operators ==========

	Equals(left, right any) schema.QueryAppender
	NotEquals(left, right any) schema.QueryAppender
	GreaterThan(left, right any) schema.QueryAppender
	GreaterThanOrEqual(left, right any) schema.QueryAppender
	LessThan(left, right any) schema.QueryAppender
	LessThanOrEqual(left, right any) schema.QueryAppender
	Between(expr, lower, upper any) schema.QueryAppender
	NotBetween(expr, lower, upper any) schema.QueryAppender
	In(expr any, values ...any) schema.QueryAppender
	NotIn(expr any, values ...any) schema.QueryAppender
	IsTrue(expr any) schema.QueryAppender
	IsFalse(expr any) schema.QueryAppender

	// ========== Expression Building ==========

	Expr(expr string, args ...any) schema.QueryAppender
	Exprs(exprs ...any) schema.QueryAppender
	ExprsWithSep(separator any, exprs ...any) schema.QueryAppender
	// ExprByDialect selects the appropriate expression builder for the current database dialect.
	ExprByDialect(exprs DialectExprs) schema.QueryAppender
	ExecByDialect(execs DialectExecs)
	ExecByDialectWithErr(execs DialectExecsWithErr) error
	FragmentByDialect(fragments DialectFragments) ([]byte, error)

	// ========== Aggregate Functions ==========

	Count(func(CountBuilder)) schema.QueryAppender
	CountColumn(column string, distinct ...bool) schema.QueryAppender
	CountAll(distinct ...bool) schema.QueryAppender
	Sum(func(SumBuilder)) schema.QueryAppender
	SumColumn(column string, distinct ...bool) schema.QueryAppender
	Avg(func(AvgBuilder)) schema.QueryAppender
	AvgColumn(column string, distinct ...bool) schema.QueryAppender
	Min(func(MinBuilder)) schema.QueryAppender
	MinColumn(column string) schema.QueryAppender
	Max(func(MaxBuilder)) schema.QueryAppender
	MaxColumn(column string) schema.QueryAppender
	StringAgg(func(StringAggBuilder)) schema.QueryAppender
	ArrayAgg(func(ArrayAggBuilder)) schema.QueryAppender
	JSONObjectAgg(func(JSONObjectAggBuilder)) schema.QueryAppender
	JSONArrayAgg(func(JSONArrayAggBuilder)) schema.QueryAppender
	BitOr(func(BitOrBuilder)) schema.QueryAppender
	BitAnd(func(BitAndBuilder)) schema.QueryAppender
	BoolOr(func(BoolOrBuilder)) schema.QueryAppender
	BoolAnd(func(BoolAndBuilder)) schema.QueryAppender
	StdDev(func(StdDevBuilder)) schema.QueryAppender
	Variance(func(VarianceBuilder)) schema.QueryAppender

	// ========== Window Functions ==========

	RowNumber(func(RowNumberBuilder)) schema.QueryAppender
	Rank(func(RankBuilder)) schema.QueryAppender
	DenseRank(func(DenseRankBuilder)) schema.QueryAppender
	PercentRank(func(PercentRankBuilder)) schema.QueryAppender
	CumeDist(func(CumeDistBuilder)) schema.QueryAppender
	NTile(func(NTileBuilder)) schema.QueryAppender
	Lag(func(LagBuilder)) schema.QueryAppender
	Lead(func(LeadBuilder)) schema.QueryAppender
	FirstValue(func(FirstValueBuilder)) schema.QueryAppender
	LastValue(func(LastValueBuilder)) schema.QueryAppender
	NthValue(func(NthValueBuilder)) schema.QueryAppender
	WinCount(func(WindowCountBuilder)) schema.QueryAppender
	WinSum(func(WindowSumBuilder)) schema.QueryAppender
	WinAvg(func(WindowAvgBuilder)) schema.QueryAppender
	WinMin(func(WindowMinBuilder)) schema.QueryAppender
	WinMax(func(WindowMaxBuilder)) schema.QueryAppender
	WinStringAgg(func(WindowStringAggBuilder)) schema.QueryAppender
	WinArrayAgg(func(WindowArrayAggBuilder)) schema.QueryAppender
	WinStdDev(func(WindowStdDevBuilder)) schema.QueryAppender
	WinVariance(func(WindowVarianceBuilder)) schema.QueryAppender
	WinJSONObjectAgg(func(WindowJSONObjectAggBuilder)) schema.QueryAppender
	WinJSONArrayAgg(func(WindowJSONArrayAggBuilder)) schema.QueryAppender
	WinBitOr(func(WindowBitOrBuilder)) schema.QueryAppender
	WinBitAnd(func(WindowBitAndBuilder)) schema.QueryAppender
	WinBoolOr(func(WindowBoolOrBuilder)) schema.QueryAppender
	WinBoolAnd(func(WindowBoolAndBuilder)) schema.QueryAppender

	// ========== String Functions ==========

	Concat(args ...any) schema.QueryAppender
	ConcatWithSep(separator any, args ...any) schema.QueryAppender
	// SubString extracts a substring. start is 1-based; length is optional.
	SubString(expr, start any, length ...any) schema.QueryAppender
	Upper(expr any) schema.QueryAppender
	Lower(expr any) schema.QueryAppender
	Trim(expr any) schema.QueryAppender
	TrimLeft(expr any) schema.QueryAppender
	TrimRight(expr any) schema.QueryAppender
	Length(expr any) schema.QueryAppender
	CharLength(expr any) schema.QueryAppender
	// Position finds the position of substring in string (1-based, 0 if not found).
	Position(substring, str any) schema.QueryAppender
	Left(expr, length any) schema.QueryAppender
	Right(expr, length any) schema.QueryAppender
	Repeat(expr, count any) schema.QueryAppender
	Replace(expr, search, replacement any) schema.QueryAppender
	Contains(expr, substr any) schema.QueryAppender
	StartsWith(expr, prefix any) schema.QueryAppender
	EndsWith(expr, suffix any) schema.QueryAppender
	ContainsIgnoreCase(expr, substr any) schema.QueryAppender
	StartsWithIgnoreCase(expr, prefix any) schema.QueryAppender
	EndsWithIgnoreCase(expr, suffix any) schema.QueryAppender
	Reverse(expr any) schema.QueryAppender

	// ========== Date and Time Functions ==========

	CurrentDate() schema.QueryAppender
	CurrentTime() schema.QueryAppender
	CurrentTimestamp() schema.QueryAppender
	// Now is an alias for CurrentTimestamp.
	Now() schema.QueryAppender
	ExtractYear(expr any) schema.QueryAppender
	ExtractMonth(expr any) schema.QueryAppender
	ExtractDay(expr any) schema.QueryAppender
	ExtractHour(expr any) schema.QueryAppender
	ExtractMinute(expr any) schema.QueryAppender
	ExtractSecond(expr any) schema.QueryAppender
	DateTrunc(unit DateTimeUnit, expr any) schema.QueryAppender
	DateAdd(expr, interval any, unit DateTimeUnit) schema.QueryAppender
	DateSubtract(expr, interval any, unit DateTimeUnit) schema.QueryAppender
	DateDiff(start, end any, unit DateTimeUnit) schema.QueryAppender
	Age(start, end any) schema.QueryAppender

	// ========== Math Functions ==========

	Abs(expr any) schema.QueryAppender
	Ceil(expr any) schema.QueryAppender
	Floor(expr any) schema.QueryAppender
	Round(expr any, precision ...any) schema.QueryAppender
	Trunc(expr any, precision ...any) schema.QueryAppender
	Power(base, exponent any) schema.QueryAppender
	Sqrt(expr any) schema.QueryAppender
	Exp(expr any) schema.QueryAppender
	Ln(expr any) schema.QueryAppender
	// Log returns the logarithm with specified base (default base 10).
	Log(expr any, base ...any) schema.QueryAppender
	Sin(expr any) schema.QueryAppender
	Cos(expr any) schema.QueryAppender
	Tan(expr any) schema.QueryAppender
	Asin(expr any) schema.QueryAppender
	Acos(expr any) schema.QueryAppender
	Atan(expr any) schema.QueryAppender
	Pi() schema.QueryAppender
	Random() schema.QueryAppender
	// Sign returns -1, 0, or 1.
	Sign(expr any) schema.QueryAppender
	Mod(dividend, divisor any) schema.QueryAppender
	Greatest(args ...any) schema.QueryAppender
	Least(args ...any) schema.QueryAppender

	// ========== Conditional Functions ==========

	Coalesce(args ...any) schema.QueryAppender
	// NullIf returns null if the two arguments are equal, otherwise the first argument.
	NullIf(expr1, expr2 any) schema.QueryAppender
	// IfNull returns defaultValue if expr is null, otherwise expr.
	IfNull(expr, defaultValue any) schema.QueryAppender

	// ========== Type Conversion Functions ==========

	ToString(expr any) schema.QueryAppender
	ToInteger(expr any) schema.QueryAppender
	ToDecimal(expr any, precision ...any) schema.QueryAppender
	ToFloat(expr any) schema.QueryAppender
	ToBool(expr any) schema.QueryAppender
	ToDate(expr any, format ...any) schema.QueryAppender
	ToTime(expr any, format ...any) schema.QueryAppender
	ToTimestamp(expr any, format ...any) schema.QueryAppender
	ToJSON(expr any) schema.QueryAppender

	// ========== JSON Functions ==========

	JSONExtract(json, path any) schema.QueryAppender
	JSONUnquote(expr any) schema.QueryAppender
	JSONArray(args ...any) schema.QueryAppender
	JSONObject(keyValues ...any) schema.QueryAppender
	JSONContains(json, value any) schema.QueryAppender
	JSONContainsPath(json, path any) schema.QueryAppender
	JSONKeys(json any, path ...any) schema.QueryAppender
	JSONLength(json any, path ...any) schema.QueryAppender
	JSONType(json any, path ...any) schema.QueryAppender
	JSONValid(expr any) schema.QueryAppender
	// JSONSet sets value at path (insert or update).
	JSONSet(json, path, value any) schema.QueryAppender
	// JSONInsert inserts value at path only if path doesn't exist.
	JSONInsert(json, path, value any) schema.QueryAppender
	// JSONReplace replaces value at path only if path exists.
	JSONReplace(json, path, value any) schema.QueryAppender
	JSONArrayAppend(json, path, value any) schema.QueryAppender

	// ========== Utility Functions ==========

	// Decode implements Oracle-style case expression.
	// Usage: Decode(expr, search1, result1, search2, result2, ..., defaultResult)
	Decode(args ...any) schema.QueryAppender
}

// SelectQueryExecutor extends QueryExecutor with SELECT-specific operations.
type SelectQueryExecutor interface {
	QueryExecutor

	Rows(ctx context.Context) (*sql.Rows, error)
	ScanAndCount(ctx context.Context, dest ...any) (int64, error)
	Count(ctx context.Context) (int64, error)
	Exists(ctx context.Context) (bool, error)
}

// SelectQuery builds and executes SELECT queries with support for
// joins, conditions, ordering, grouping, pagination, locking, and set operations.
type SelectQuery interface {
	QueryBuilder
	SelectQueryExecutor
	DBAccessor
	CTE[SelectQuery]
	Selectable[SelectQuery]
	TableSource[SelectQuery]
	JoinOperations[SelectQuery]
	Filterable[SelectQuery]
	Orderable[SelectQuery]
	Limitable[SelectQuery]
	Applier[SelectQuery]

	SelectAs(column, alias string) SelectQuery
	// SelectModelColumns selects the model's columns. This is the default if no select methods are called.
	SelectModelColumns() SelectQuery
	SelectModelPKs() SelectQuery
	SelectExpr(builder func(ExprBuilder) any, alias ...string) SelectQuery
	Distinct() SelectQuery
	DistinctOnColumns(columns ...string) SelectQuery
	DistinctOnExpr(builder func(ExprBuilder) any) SelectQuery
	// JoinRelations applies RelationSpec configurations for declarative JOIN operations.
	JoinRelations(specs ...*RelationSpec) SelectQuery
	Relation(name string, apply ...func(query SelectQuery)) SelectQuery
	GroupBy(columns ...string) SelectQuery
	GroupByExpr(func(ExprBuilder) any) SelectQuery
	Having(func(ConditionBuilder)) SelectQuery
	Offset(offset int) SelectQuery
	Paginate(pageable page.Pageable) SelectQuery
	ForShare(tables ...string) SelectQuery
	ForShareNoWait(tables ...string) SelectQuery
	ForShareSkipLocked(tables ...string) SelectQuery
	ForUpdate(tables ...string) SelectQuery
	ForUpdateNoWait(tables ...string) SelectQuery
	ForUpdateSkipLocked(tables ...string) SelectQuery
	Union(func(query SelectQuery)) SelectQuery
	UnionAll(func(query SelectQuery)) SelectQuery
	Intersect(func(query SelectQuery)) SelectQuery
	IntersectAll(func(query SelectQuery)) SelectQuery
	Except(func(query SelectQuery)) SelectQuery
	ExceptAll(func(query SelectQuery)) SelectQuery
}

// RawQuery executes raw SQL queries with parameter binding.
type RawQuery interface {
	QueryExecutor
}

// InsertQuery builds and executes INSERT queries with conflict resolution and RETURNING support.
type InsertQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[InsertQuery]
	TableSource[InsertQuery]
	Selectable[InsertQuery]
	ColumnUpdatable[InsertQuery]
	Returnable[InsertQuery]
	Applier[InsertQuery]

	// OnConflict configures conflict handling (UPSERT).
	OnConflict(func(ConflictBuilder)) InsertQuery
}

// UpdateQuery builds and executes UPDATE queries.
// Does not inherit JoinOperations; use From methods for PostgreSQL-style FROM clause.
type UpdateQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[UpdateQuery]
	TableSource[UpdateQuery]
	Selectable[UpdateQuery]
	Filterable[UpdateQuery]
	Orderable[UpdateQuery]
	Limitable[UpdateQuery]
	ColumnUpdatable[UpdateQuery]
	Returnable[UpdateQuery]
	Applier[UpdateQuery]

	// Set is an alias for Column.
	Set(name string, value any) UpdateQuery
	// SetExpr is an alias for ColumnExpr.
	SetExpr(name string, builder func(ExprBuilder) any) UpdateQuery
	OmitZero() UpdateQuery
	Bulk() UpdateQuery
}

// DeleteQuery builds and executes DELETE queries with soft delete support.
type DeleteQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[DeleteQuery]
	TableSource[DeleteQuery]
	Filterable[DeleteQuery]
	Orderable[DeleteQuery]
	Limitable[DeleteQuery]
	Returnable[DeleteQuery]
	Applier[DeleteQuery]

	// ForceDelete bypasses soft delete and performs a hard delete.
	ForceDelete() DeleteQuery
}

// MergeQuery builds and executes MERGE (UPSERT) queries with conditional match/no-match actions.
type MergeQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[MergeQuery]
	TableSource[MergeQuery]
	Returnable[MergeQuery]
	Applier[MergeQuery]

	Using(model any, alias ...string) MergeQuery
	UsingTable(table string, alias ...string) MergeQuery
	UsingExpr(builder func(ExprBuilder) any, alias ...string) MergeQuery
	UsingSubQuery(builder func(SelectQuery), alias ...string) MergeQuery

	// On specifies the merge condition that determines matches between target and source.
	On(func(ConditionBuilder)) MergeQuery

	WhenMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder
	WhenNotMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder
	WhenNotMatchedByTarget(builder ...func(ConditionBuilder)) MergeWhenBuilder
	WhenNotMatchedBySource(builder ...func(ConditionBuilder)) MergeWhenBuilder
}

// TableTarget defines table targeting methods for DDL queries.
type TableTarget[T Executor] interface {
	Model(model any) T
	Table(tables ...string) T
}

// CreateTableQuery builds and executes CREATE TABLE queries.
type CreateTableQuery interface {
	Executor
	TableTarget[CreateTableQuery]
	fmt.Stringer

	Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) CreateTableQuery
	Temp() CreateTableQuery
	IfNotExists() CreateTableQuery
	// DefaultVarChar sets the default VARCHAR length for string columns in model-based creation.
	DefaultVarChar(n int) CreateTableQuery
	PrimaryKey(builder func(PrimaryKeyBuilder)) CreateTableQuery
	Unique(builder func(UniqueBuilder)) CreateTableQuery
	Check(builder func(CheckBuilder)) CreateTableQuery
	ForeignKey(builder func(ForeignKeyBuilder)) CreateTableQuery
	PartitionBy(strategy PartitionStrategy, columns ...string) CreateTableQuery
	TableSpace(tablespace string) CreateTableQuery
	// WithForeignKeys creates foreign keys from model relations.
	WithForeignKeys() CreateTableQuery
}

// DropTableQuery builds and executes DROP TABLE queries.
type DropTableQuery interface {
	Executor
	TableTarget[DropTableQuery]
	fmt.Stringer

	IfExists() DropTableQuery
	Cascade() DropTableQuery
	Restrict() DropTableQuery
}

// CreateIndexQuery builds and executes CREATE INDEX queries.
type CreateIndexQuery interface {
	Executor
	TableTarget[CreateIndexQuery]

	Index(name string) CreateIndexQuery
	Column(columns ...string) CreateIndexQuery
	ColumnExpr(builder func(ExprBuilder) any) CreateIndexQuery
	ExcludeColumn(columns ...string) CreateIndexQuery
	Unique() CreateIndexQuery
	// Concurrently creates the index concurrently (PostgreSQL only).
	Concurrently() CreateIndexQuery
	IfNotExists() CreateIndexQuery
	// Include adds covering columns (PostgreSQL INCLUDE clause).
	Include(columns ...string) CreateIndexQuery
	Using(method IndexMethod) CreateIndexQuery
	Where(builder func(ConditionBuilder)) CreateIndexQuery
}

// DropIndexQuery builds and executes DROP INDEX queries.
type DropIndexQuery interface {
	Executor

	Index(name string) DropIndexQuery
	IfExists() DropIndexQuery
	// Concurrently drops the index concurrently (PostgreSQL only).
	Concurrently() DropIndexQuery
	Cascade() DropIndexQuery
	Restrict() DropIndexQuery
}

// TruncateTableQuery builds and executes TRUNCATE TABLE queries.
type TruncateTableQuery interface {
	Executor
	TableTarget[TruncateTableQuery]

	ContinueIdentity() TruncateTableQuery
	Cascade() TruncateTableQuery
	Restrict() TruncateTableQuery
}

// AddColumnQuery builds and executes ALTER TABLE ADD COLUMN queries.
type AddColumnQuery interface {
	Executor
	TableTarget[AddColumnQuery]

	Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) AddColumnQuery
	IfNotExists() AddColumnQuery
}

// DropColumnQuery builds and executes ALTER TABLE DROP COLUMN queries.
type DropColumnQuery interface {
	Executor
	TableTarget[DropColumnQuery]

	Column(columns ...string) DropColumnQuery
}

// Tx extends DB with Commit and Rollback for manual transaction control.
type Tx interface {
	DB
	Commit() error
	Rollback() error
}

// DB provides factory methods for creating queries and managing transactions.
type DB interface {
	NewSelect() SelectQuery
	NewInsert() InsertQuery
	NewUpdate() UpdateQuery
	NewDelete() DeleteQuery
	NewMerge() MergeQuery
	NewRaw(query string, args ...any) RawQuery
	NewCreateTable() CreateTableQuery
	NewDropTable() DropTableQuery
	NewCreateIndex() CreateIndexQuery
	NewDropIndex() DropIndexQuery
	NewTruncateTable() TruncateTableQuery
	NewAddColumn() AddColumnQuery
	NewDropColumn() DropColumnQuery
	RunInTX(ctx context.Context, fn func(ctx context.Context, tx DB) error) error
	RunInReadOnlyTX(ctx context.Context, fn func(ctx context.Context, tx DB) error) error
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	Conn(ctx context.Context) (*sql.Conn, error)
	RegisterModel(models ...any)
	ResetModel(ctx context.Context, models ...any) error
	// ScanRows scans all rows and closes *sql.Rows when done.
	ScanRows(ctx context.Context, rows *sql.Rows, dest ...any) error
	// ScanRow scans a single row without closing *sql.Rows.
	ScanRow(ctx context.Context, rows *sql.Rows, dest ...any) error
	WithNamedArg(name string, value any) DB
	ModelPKs(model any) (map[string]any, error)
	ModelPKFields(model any) []*PKField
	TableOf(model any) *schema.Table
}
