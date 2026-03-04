package sequence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestToGoLayout(t *testing.T) {
	t.Run("yyyyMMdd", func(t *testing.T) {
		assert.Equal(t, "20060102", toGoLayout("yyyyMMdd"))
	})

	t.Run("yyyyMM", func(t *testing.T) {
		assert.Equal(t, "200601", toGoLayout("yyyyMM"))
	})

	t.Run("yyMMdd", func(t *testing.T) {
		assert.Equal(t, "060102", toGoLayout("yyMMdd"))
	})

	t.Run("yyyy-MM-dd", func(t *testing.T) {
		assert.Equal(t, "2006-01-02", toGoLayout("yyyy-MM-dd"))
	})

	t.Run("yyyyMMddHHmmss", func(t *testing.T) {
		assert.Equal(t, "20060102150405", toGoLayout("yyyyMMddHHmmss"))
	})

	t.Run("EmptyFormat", func(t *testing.T) {
		assert.Equal(t, "", toGoLayout(""))
	})

	t.Run("NoPlaceholders", func(t *testing.T) {
		assert.Equal(t, "---", toGoLayout("---"))
	})

	t.Run("MixedLiteralsAndPlaceholders", func(t *testing.T) {
		assert.Equal(t, "2006/01/02", toGoLayout("yyyy/MM/dd"))
	})

	t.Run("yyyyMMddHH", func(t *testing.T) {
		assert.Equal(t, "2006010215", toGoLayout("yyyyMMddHH"))
	})
}

func TestFormatDate(t *testing.T) {
	dt := timex.DateTime(time.Date(2024, 3, 15, 9, 30, 45, 0, time.Local))

	t.Run("yyyyMMdd", func(t *testing.T) {
		assert.Equal(t, "20240315", FormatDate(dt, "yyyyMMdd"))
	})

	t.Run("yyyy-MM-dd", func(t *testing.T) {
		assert.Equal(t, "2024-03-15", FormatDate(dt, "yyyy-MM-dd"))
	})

	t.Run("yyMMdd", func(t *testing.T) {
		assert.Equal(t, "240315", FormatDate(dt, "yyMMdd"))
	})

	t.Run("yyyyMM", func(t *testing.T) {
		assert.Equal(t, "202403", FormatDate(dt, "yyyyMM"))
	})

	t.Run("yyyyMMddHHmmss", func(t *testing.T) {
		assert.Equal(t, "20240315093045", FormatDate(dt, "yyyyMMddHHmmss"))
	})

	t.Run("EmptyFormat", func(t *testing.T) {
		assert.Equal(t, "", FormatDate(dt, ""))
	})

	t.Run("OnlyLiterals", func(t *testing.T) {
		assert.Equal(t, "---", FormatDate(dt, "---"))
	})
}
