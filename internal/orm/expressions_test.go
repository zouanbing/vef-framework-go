package orm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

func newTestQueryGen() schema.QueryGen {
	return schema.NewQueryGen(sqlitedialect.New())
}

func TestExpressionsAppendQueryStringSeparator(t *testing.T) {
	gen := newTestQueryGen()

	expr := newExpressions(", ", "hello", "world")

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
	assert.Contains(t, result, ", ")
}

func TestExpressionsAppendQueryEmptyExpressions(t *testing.T) {
	gen := newTestQueryGen()

	expr := newExpressions(", ")

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)
	assert.Equal(t, "NULL", string(b))
}

func TestExpressionsAppendQueryQueryAppenderSeparator(t *testing.T) {
	gen := newTestQueryGen()

	sep := bun.Safe(" AND ")
	expr := newExpressions(sep, "a", "b")

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	assert.Contains(t, result, " AND ")
}

func TestExpressionsAppendQueryDefaultSeparator(t *testing.T) {
	gen := newTestQueryGen()

	// Use an int as separator — falls through to default branch
	expr := newExpressions(42, "x", "y")

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestExpressionsAppendQuerySliceElement(t *testing.T) {
	gen := newTestQueryGen()

	inner := []any{"a", "b"}
	expr := newExpressions(", ", inner)

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	// Slice element is rendered by gen.AppendValue as JSON array
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
}

func TestExpressionsAppendQueryQueryAppenderElement(t *testing.T) {
	gen := newTestQueryGen()

	appender := bun.Safe("NOW()")
	expr := newExpressions(", ", appender)

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	assert.Contains(t, result, "NOW()")
}

// errorAppender is a QueryAppender that always returns an error.
type errorAppender struct{}

func (e errorAppender) AppendQuery(_ schema.QueryGen, _ []byte) ([]byte, error) {
	return nil, errors.New("test error")
}

func TestExpressionsAppendQuerySeparatorError(t *testing.T) {
	gen := newTestQueryGen()

	sep := errorAppender{}
	expr := newExpressions(sep, "a", "b")

	_, err := expr.AppendQuery(gen, nil)
	assert.Error(t, err, "Should propagate separator AppendQuery error")
}

func TestExpressionsAppendQueryMultipleElements(t *testing.T) {
	gen := newTestQueryGen()

	expr := newExpressions(", ", bun.Safe("a"), bun.Safe("b"), bun.Safe("c"))

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
	assert.Contains(t, result, ", ")
}

func TestExpressionsAppendQueryBytesSliceNotNested(t *testing.T) {
	gen := newTestQueryGen()

	// []byte should NOT be treated as a nested slice
	data := []byte("hello")
	expr := newExpressions(", ", data)

	b, err := expr.AppendQuery(gen, nil)
	assert.NoError(t, err)

	result := string(b)
	// []byte should be appended as a value, not wrapped in parentheses as a sub-slice
	assert.NotContains(t, result, "(")
}
