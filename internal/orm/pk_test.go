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
	ID            string `bun:"id,pk"`
	Name          string `bun:"name"`
}

type intPKModel struct {
	bun.BaseModel `bun:"table:test_int_pk"`
	ID            int64  `bun:"id,pk"`
	Name          string `bun:"name"`
}

type int32PKModel struct {
	bun.BaseModel `bun:"table:test_int32_pk"`
	ID            int32  `bun:"id,pk"`
	Name          string `bun:"name"`
}

type floatPKModel struct {
	bun.BaseModel `bun:"table:test_float_pk"`
	ID            float64 `bun:"id,pk"`
	Name          string  `bun:"name"`
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
		assert.NoError(t, err)
		assert.Equal(t, "test-id", val)
	})

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(stringPKModel{ID: "test"})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(&s)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})

	t.Run("ReflectValuePointerToStruct", func(t *testing.T) {
		model := &stringPKModel{ID: "reflect-test"}

		val, err := pkField.Value(reflect.ValueOf(model))
		assert.NoError(t, err)
		assert.Equal(t, "reflect-test", val)
	})

	t.Run("ReflectValueNonPointerNonAddressable", func(t *testing.T) {
		model := stringPKModel{ID: "test"}

		_, err := pkField.Value(reflect.ValueOf(model))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})

	t.Run("ReflectValuePointerToNonStruct", func(t *testing.T) {
		s := "not-a-struct"

		_, err := pkField.Value(reflect.ValueOf(&s))
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})

	t.Run("ReflectValueAddressableStruct", func(t *testing.T) {
		model := stringPKModel{ID: "addressable-test"}
		// Create an addressable reflect.Value by using reflect.New
		rv := reflect.New(reflect.TypeOf(model)).Elem()
		rv.Set(reflect.ValueOf(model))

		val, err := pkField.Value(rv)
		assert.NoError(t, err)
		assert.Equal(t, "addressable-test", val)
	})
}

func TestPKFieldSetStringType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	t.Run("SetStringValue", func(t *testing.T) {
		model := &stringPKModel{}

		err := pkField.Set(model, "new-id")
		assert.NoError(t, err)
		assert.Equal(t, "new-id", model.ID)
	})

	t.Run("SetIntAsString", func(t *testing.T) {
		model := &stringPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err)
		assert.Equal(t, "42", model.ID)
	})

	t.Run("InvalidModel", func(t *testing.T) {
		err := pkField.Set(stringPKModel{}, "value")
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})
}

func TestPKFieldSetIntTypes(t *testing.T) {
	db := newTestBunDB(t)

	t.Run("Int64PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, 42)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), model.ID)
	})

	t.Run("Int64PKFromString", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, "123")
		assert.NoError(t, err)
		assert.Equal(t, int64(123), model.ID)
	})

	t.Run("Int32PK", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*int32PKModel)(nil))
		model := &int32PKModel{}

		err := pkField.Set(model, 99)
		assert.NoError(t, err)
		assert.Equal(t, int32(99), model.ID)
	})

	t.Run("Int64PKInvalidValue", func(t *testing.T) {
		pkField := newPKFieldForModel(t, db, (*intPKModel)(nil))
		model := &intPKModel{}

		err := pkField.Set(model, "not-a-number")
		assert.Error(t, err)
	})
}

func TestPKFieldSetUnsupportedType(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*floatPKModel)(nil))
	model := &floatPKModel{}

	err := pkField.Set(model, 3.14)
	assert.ErrorIs(t, err, ErrPrimaryKeyUnsupportedType)
}

func TestPKFieldValueErrors(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	t.Run("NonPointerModel", func(t *testing.T) {
		_, err := pkField.Value(stringPKModel{})
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})

	t.Run("PointerToNonStruct", func(t *testing.T) {
		n := 42

		_, err := pkField.Value(&n)
		assert.ErrorIs(t, err, ErrModelMustBePointerToStruct)
	})
}

func TestNewPKField(t *testing.T) {
	db := newTestBunDB(t)
	pkField := newPKFieldForModel(t, db, (*stringPKModel)(nil))

	assert.Equal(t, "ID", pkField.Field)
	assert.Equal(t, "id", pkField.Column)
	assert.Equal(t, "id", pkField.Name)
}
