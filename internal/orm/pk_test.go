package orm

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Test models with different PK types for PKField testing.
type stringPKModel struct {
	bun.BaseModel `bun:"table:test_string_pk"`

	ID   string `bun:"id,pk"`
	Name string `bun:"name"`
}

type intPKModel struct {
	bun.BaseModel `bun:"table:test_int_pk"`

	ID   int64  `bun:"id,pk"`
	Name string `bun:"name"`
}

type int32PKModel struct {
	bun.BaseModel `bun:"table:test_int32_pk"`

	ID   int32  `bun:"id,pk"`
	Name string `bun:"name"`
}

type floatPKModel struct {
	bun.BaseModel `bun:"table:test_float_pk"`

	ID   float64 `bun:"id,pk"`
	Name string  `bun:"name"`
}

func newTestBunDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err)

	t.Cleanup(func() { sqldb.Close() })

	return bun.NewDB(sqldb, sqlitedialect.New())
}

func newPKFieldForModel(t *testing.T, db *bun.DB, model any) *PKField {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}

	table := db.Table(modelType)
	require.True(t, len(table.PKs) > 0, "Model must have at least one primary key")

	return NewPKField(table.PKs[0])
}

func TestPKFieldValidateModel(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	t.Run("ValidPointerToStruct", func(t *testing.T) {
		model := &stringPKModel{ID: "test-id"}

		val, err := pkField.Value(model)
		assert.NoError(t, err, "Should extract value from valid pointer model")
		assert.Equal(t, "test-id", val, "Should return correct PK value")
	})

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(stringPKModel{ID: "test"})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(&s)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject pointer to non-struct")
	})

	t.Run("ReflectValuePointerToStruct", func(t *testing.T) {
		model := &stringPKModel{ID: "reflect-test"}

		val, err := pkField.Value(reflect.ValueOf(model))
		assert.NoError(t, err, "Should accept reflect.Value pointer to struct")
		assert.Equal(t, "reflect-test", val, "Should return correct PK value")
	})

	t.Run("ReflectValueNonPointerNonAddressable", func(t *testing.T) {
		model := stringPKModel{ID: "test"}

		_, err := pkField.Value(reflect.ValueOf(model))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-addressable reflect.Value")
	})

	t.Run("ReflectValuePointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(reflect.ValueOf(&s))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject reflect.Value pointer to non-struct")
	})

	t.Run("ReflectValueAddressableStruct", func(t *testing.T) {
		model := stringPKModel{ID: "addressable-test"}
		// Create an addressable reflect.Value by using reflect.New
		rv := reflect.New(reflect.TypeOf(model)).Elem()
		rv.Set(reflect.ValueOf(model))

		val, err := pkField.Value(rv)
		assert.NoError(t, err, "Should accept addressable reflect.Value struct")
		assert.Equal(t, "addressable-test", val, "Should return correct PK value")
	})
}

// TestPKFieldSetStringType verifies PKField.Set for string-typed primary keys.
func TestPKFieldSetStringType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	t.Run("SetStringValue", func(t *testing.T) {
		model := &stringPKModel{}

		err := pkField.Set(model, "new-id")
		assert.NoError(t, err, "Should set string PK value")
		assert.Equal(t, "new-id", model.ID, "Should update model ID field")
	})

	t.Run("SetIntAsString", func(t *testing.T) {
		model := &stringPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err, "Should convert int to string for string PK")
		assert.Equal(t, "42", model.ID, "Should store converted string value")
	})

	t.Run("InvalidModel", func(t *testing.T) {
		err := pkField.Set(stringPKModel{}, "value")
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})
}

// TestPKFieldSetIntTypes verifies PKField.Set for integer-typed primary keys.
func TestPKFieldSetIntTypes(t *testing.T) {
	db := newTestBunDB(t)

	t.Run("Int64PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err, "Should set int64 PK value")
		assert.Equal(t, int64(42), model.ID, "Should store correct int64 value")
	})

	t.Run("Int64PKFromString", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, "123")
		assert.NoError(t, err, "Should parse string to int64 PK")
		assert.Equal(t, int64(123), model.ID, "Should store parsed int64 value")
	})

	t.Run("Int32PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*int32PKModel)(nil))
		model := &int32PKModel{}

		err := pkField.Set(model, 99)
		assert.NoError(t, err, "Should set int32 PK value")
		assert.Equal(t, int32(99), model.ID, "Should store correct int32 value")
	})

	t.Run("Int64PKInvalidValue", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, "not-a-number")
		assert.Error(t, err, "Should reject non-numeric string for int PK")
	})
}

// TestPKFieldSetUnsupportedType verifies PKField.Set rejects unsupported PK types.
func TestPKFieldSetUnsupportedType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*floatPKModel)(nil))
	model := &floatPKModel{}

	err := pkField.Set(model, 3.14)
	assert.ErrorIs(t, err, ErrPrimaryKeyUnsupportedType, "Should reject unsupported PK type")
}

// TestPKFieldValueErrors verifies PKField.Value returns errors for invalid models.
func TestPKFieldValueErrors(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(stringPKModel{})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject non-pointer model")
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		n := 42

		_, err := pkField.Value(&n)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct, "Should reject pointer to non-struct")
	})
}

// TestNewPKField verifies NewPKField populates Field, Column, and Name correctly.
func TestNewPKField(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	assert.Equal(t, "ID", pkField.Field, "Should use struct field name")
	assert.Equal(t, "id", pkField.Column, "Should use bun column tag")
	assert.Equal(t, "id", pkField.Name, "Should use column name as display name")
}
