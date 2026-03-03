package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"
)

// autoColumnHandlers manages audit fields (ID, timestamps, user tracking) on insert/update.
var autoColumnHandlers = []ColumnHandler{
	&IDHandler{},
	&CreatedAtHandler{},
	&UpdatedAtHandler{},
	&CreatedByHandler{},
	&UpdatedByHandler{},
}

// ColumnHandler provides the column name that the handler manages.
type ColumnHandler interface {
	// Name returns the database column name this handler manages (e.g., "id", "created_at").
	Name() string
}

// InsertColumnHandler manages columns automatically during insert operations.
type InsertColumnHandler interface {
	ColumnHandler
	// OnInsert sets the column value automatically when a new row is inserted.
	OnInsert(query *BunInsertQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// UpdateColumnHandler manages columns during both insert and update operations.
type UpdateColumnHandler interface {
	InsertColumnHandler
	// OnUpdate sets the column value automatically when an existing row is updated.
	OnUpdate(query *BunUpdateQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// processAutoColumns applies auto column handlers to a model before insert/update operations.
func processAutoColumns(query any, table *schema.Table, modelValue any, mv reflect.Value) {
	if !mv.IsValid() || (mv.Kind() == reflect.Ptr && mv.IsNil()) {
		return
	}

	// Handle slice values (batch operations) by processing each element
	if mv.Kind() == reflect.Slice {
		for i := range mv.Len() {
			elem := mv.Index(i)
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}

			processAutoColumns(query, table, elem.Interface(), elem)
		}

		return
	}

	for _, handler := range autoColumnHandlers {
		field, ok := table.FieldMap[handler.Name()]
		if !ok {
			continue
		}

		value := field.Value(mv)

		switch q := query.(type) {
		case *BunInsertQuery:
			if insertHandler, ok := handler.(InsertColumnHandler); ok {
				insertHandler.OnInsert(q, table, field, modelValue, value)
			}
		case *BunUpdateQuery:
			if updateHandler, ok := handler.(UpdateColumnHandler); ok {
				updateHandler.OnUpdate(q, table, field, modelValue, value)
			}
		}
	}
}
