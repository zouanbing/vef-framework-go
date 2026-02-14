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

// ColumnHandler is the base interface for all auto column handlers.
// It provides the column name that the handler manages.
type ColumnHandler interface {
	// Name returns the name of the column this handler manages.
	Name() string
}

// InsertColumnHandler is an interface for handlers that automatically manage columns during insert operations.
// Handlers implementing this interface will be called before insert operations to set column values.
type InsertColumnHandler interface {
	ColumnHandler
	// OnInsert is called when a new record is being inserted.
	// It allows the handler to automatically set or modify column values.
	OnInsert(query *BunInsertQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// UpdateColumnHandler is an interface for handlers that manage columns during both insert and update operations.
// It extends InsertHandler to also handle update scenarios with additional context.
type UpdateColumnHandler interface {
	InsertColumnHandler
	// OnUpdate is called when an existing record is being updated.
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
