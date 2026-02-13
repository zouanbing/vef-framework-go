package orm

const (
	columnAll    = "*"     // columnAll is the constant for the all column
	sqlNull      = "NULL"  // sqlNull is the constant for the NULL value
	separatorAnd = " AND " // separatorAnd is the separator for the AND condition
	separatorOr  = " OR "  // separatorOr is the separator for the OR condition
)

// Placeholder key for named arguments in database queries.
const PlaceholderKeyOperator = "Operator"

// System operators for audit tracking.
const (
	OperatorSystem    = "system"
	OperatorCronJob   = "cron_job"
	OperatorAnonymous = "anonymous"
)

// SQL expression placeholders for query building.
const (
	ExprOperator     = "?Operator"
	ExprTableColumns = "?TableColumns"
	ExprColumns      = "?Columns"
	ExprTablePKs     = "?TablePKs"
	ExprPKs          = "?PKs"
	ExprTableName    = "?TableName"
	ExprTableAlias   = "?TableAlias"
)

// Database column names for audit fields.
const (
	ColumnID            = "id"
	ColumnCreatedAt     = "created_at"
	ColumnUpdatedAt     = "updated_at"
	ColumnCreatedBy     = "created_by"
	ColumnUpdatedBy     = "updated_by"
	ColumnCreatedByName = "created_by_name"
	ColumnUpdatedByName = "updated_by_name"
)

// Go struct field names corresponding to audit columns.
const (
	FieldID            = "ID"
	FieldCreatedAt     = "CreatedAt"
	FieldUpdatedAt     = "UpdatedAt"
	FieldCreatedBy     = "CreatedBy"
	FieldUpdatedBy     = "UpdatedBy"
	FieldCreatedByName = "CreatedByName"
	FieldUpdatedByName = "UpdatedByName"
)
