package tabular

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ilxqx/vef-framework-go/null"
	"github.com/ilxqx/vef-framework-go/timex"
)

// TestDefaultParser_Parse_EmptyString tests Default Parser parse_ empty string scenarios.
func TestDefaultParser_Parse_EmptyString(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		targetType reflect.Type
	}{
		{"String", reflect.TypeFor[string]()},
		{"Int", reflect.TypeFor[int]()},
		{"Float", reflect.TypeFor[float64]()},
		{"Bool", reflect.TypeFor[bool]()},
		{"NullString", reflect.TypeFor[null.String]()},
		{"NullInt", reflect.TypeFor[null.Int]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse("", tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, reflect.Zero(tt.targetType).Interface(), result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_BasicTypes tests Default Parser parse_ basic types scenarios.
func TestDefaultParser_Parse_BasicTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"String", "hello", reflect.TypeFor[string](), "hello"},
		{"Int", "42", reflect.TypeFor[int](), 42},
		{"Int8", "127", reflect.TypeFor[int8](), int8(127)},
		{"Int16", "32767", reflect.TypeFor[int16](), int16(32767)},
		{"Int32", "2147483647", reflect.TypeFor[int32](), int32(2147483647)},
		{"Int64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
		{"Uint", "42", reflect.TypeFor[uint](), uint(42)},
		{"Uint8", "255", reflect.TypeFor[uint8](), uint8(255)},
		{"Uint16", "65535", reflect.TypeFor[uint16](), uint16(65535)},
		{"Uint32", "4294967295", reflect.TypeFor[uint32](), uint32(4294967295)},
		{"Uint64", "18446744073709551615", reflect.TypeFor[uint64](), uint64(18446744073709551615)},
		{"Float32", "3.14", reflect.TypeFor[float32](), float32(3.14)},
		{"Float64", "3.14159265359", reflect.TypeFor[float64](), float64(3.14159265359)},
		{"BoolTrue", "true", reflect.TypeFor[bool](), true},
		{"BoolFalse", "false", reflect.TypeFor[bool](), false},
		{"Bool1", "1", reflect.TypeFor[bool](), true},
		{"Bool0", "0", reflect.TypeFor[bool](), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_InvalidBasicTypes tests Default Parser parse_ invalid basic types scenarios.
func TestDefaultParser_Parse_InvalidBasicTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"InvalidInt", "not_a_number", reflect.TypeFor[int]()},
		{"InvalidFloat", "abc", reflect.TypeFor[float64]()},
		{"InvalidBool", "maybe", reflect.TypeFor[bool]()},
		{"InvalidUint", "-1", reflect.TypeFor[uint]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
		})
	}
}

// TestDefaultParser_Parse_PointerTypes tests Default Parser parse_ pointer types scenarios.
func TestDefaultParser_Parse_PointerTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "StringPointer",
			cellValue:  "test",
			targetType: reflect.TypeFor[*string](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*string)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, "test", *ptr, "Should equal expected value")
			},
		},
		{
			name:       "IntPointer",
			cellValue:  "42",
			targetType: reflect.TypeFor[*int](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*int)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, 42, *ptr, "Should equal expected value")
			},
		},
		{
			name:       "BoolPointer",
			cellValue:  "true",
			targetType: reflect.TypeFor[*bool](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*bool)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, true, *ptr, "Should equal expected value")
			},
		},
		{
			name:       "Float64Pointer",
			cellValue:  "3.14",
			targetType: reflect.TypeFor[*float64](),
			validate: func(t *testing.T, result any) {
				ptr := result.(*float64)
				assert.NotNil(t, ptr, "Should not be nil")
				assert.Equal(t, 3.14, *ptr, "Should equal expected value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			require.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParser_Parse_NullTypes tests Default Parser parse_ null types scenarios.
func TestDefaultParser_Parse_NullTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"NullString", "test", typeNullString, null.StringFrom("test")},
		{"NullInt", "42", typeNullInt, null.IntFrom(42)},
		{"NullInt16", "123", typeNullInt16, null.Int16From(123)},
		{"NullInt32", "456", typeNullInt32, null.Int32From(456)},
		{"NullFloat", "3.14", typeNullFloat, null.FloatFrom(3.14)},
		{"NullBool", "true", typeNullBool, null.BoolFrom(true)},
		{"NullByte", "255", typeNullByte, null.ByteFrom(255)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_InvalidNullTypes tests Default Parser parse_ invalid null types scenarios.
func TestDefaultParser_Parse_InvalidNullTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"InvalidNullInt", "not_a_number", typeNullInt},
		{"InvalidNullFloat", "abc", typeNullFloat},
		{"InvalidNullBool", "maybe", typeNullBool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
		})
	}
}

// TestDefaultParser_Parse_TimeTypes tests Default Parser parse_ time types scenarios.
func TestDefaultParser_Parse_TimeTypes(t *testing.T) {
	parser := NewDefaultParser("")

	testTimeStr := "2024-01-15 14:30:45"
	testDateStr := "2024-01-15"
	testTimeOnlyStr := "14:30:45"

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "TimeTime",
			cellValue:  testTimeStr,
			targetType: typeTime,
			validate: func(t *testing.T, result any) {
				parsed := result.(time.Time)
				expected := time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local)
				assert.Equal(t, expected, parsed, "Should equal expected value")
			},
		},
		{
			name:       "NullDateTime",
			cellValue:  testTimeStr,
			targetType: typeNullDateTime,
			validate: func(t *testing.T, result any) {
				nullDT := result.(null.DateTime)
				assert.True(t, nullDT.Valid, "Should be valid")

				expected := timex.DateTime(time.Date(2024, 1, 15, 14, 30, 45, 0, time.Local))
				assert.Equal(t, expected, nullDT.ValueOrZero(), "Should equal expected value")
			},
		},
		{
			name:       "NullDate",
			cellValue:  testDateStr,
			targetType: typeNullDate,
			validate: func(t *testing.T, result any) {
				nullDate := result.(null.Date)
				assert.True(t, nullDate.Valid, "Should be valid")

				expected := timex.Date(time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local))
				assert.Equal(t, expected, nullDate.ValueOrZero(), "Should equal expected value")
			},
		},
		{
			name:       "NullTime",
			cellValue:  testTimeOnlyStr,
			targetType: typeNullTime,
			validate: func(t *testing.T, result any) {
				nullTime := result.(null.Time)
				assert.True(t, nullTime.Valid, "Should be valid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			require.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParser_Parse_TimeTypesWithFormat tests Default Parser parse_ time types with format scenarios.
func TestDefaultParser_Parse_TimeTypesWithFormat(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "TimeTimeCustomFormat",
			format:     "2006-01-02",
			cellValue:  "2024-01-15",
			targetType: typeTime,
			validate: func(t *testing.T, result any) {
				parsed := result.(time.Time)
				expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
				assert.Equal(t, expected, parsed, "Should equal expected value")
			},
		},
		{
			name:       "TimeTimeRFC3339",
			format:     time.RFC3339,
			cellValue:  "2024-01-15T14:30:45+08:00",
			targetType: typeTime,
			validate: func(t *testing.T, result any) {
				parsed := result.(time.Time)
				assert.Equal(t, 2024, parsed.Year(), "Should equal expected value")
				assert.Equal(t, time.January, parsed.Month(), "Should equal expected value")
				assert.Equal(t, 15, parsed.Day(), "Should equal expected value")
			},
		},
		{
			name:       "NullDateTimeCustomFormat",
			format:     "2006/01/02 15:04",
			cellValue:  "2024/01/15 14:30",
			targetType: typeNullDateTime,
			validate: func(t *testing.T, result any) {
				nullDT := result.(null.DateTime)
				assert.True(t, nullDT.Valid, "Should be valid")
				dt := nullDT.ValueOrZero()
				assert.Equal(t, 2024, time.Time(dt).Year(), "Should equal expected value")
			},
		},
		{
			name:       "NullDateCustomFormat",
			format:     "2006年01月02日",
			cellValue:  "2024年01月15日",
			targetType: typeNullDate,
			validate: func(t *testing.T, result any) {
				nullDate := result.(null.Date)
				assert.True(t, nullDate.Valid, "Should be valid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewDefaultParser(tt.format)
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			require.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParser_Parse_InvalidTimeTypes tests Default Parser parse_ invalid time types scenarios.
func TestDefaultParser_Parse_InvalidTimeTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"InvalidTimeTime", "not_a_time", typeTime},
		{"InvalidNullDateTime", "invalid", typeNullDateTime},
		{"InvalidNullDate", "2024-13-45", typeNullDate},
		{"InvalidNullTime", "25:99:99", typeNullTime},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
		})
	}
}

// TestDefaultParser_Parse_DecimalTypes tests Default Parser parse_ decimal types scenarios.
func TestDefaultParser_Parse_DecimalTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   string
	}{
		{"DecimalInteger", "100", typeDecimal, "100"},
		{"DecimalFloat", "3.14", typeDecimal, "3.14"},
		{"DecimalScientific", "1.23e+2", typeDecimal, "123"},
		{"NullDecimalValid", "99.99", typeNullDecimal, "99.99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")

			switch tt.targetType {
			case typeDecimal:
				d := result.(decimal.Decimal)
				assert.Equal(t, tt.expected, d.String(), "Should equal expected value")
			case typeNullDecimal:
				nd := result.(null.Decimal)
				assert.True(t, nd.Valid, "Should be valid")
				assert.Equal(t, tt.expected, nd.ValueOrZero().String(), "Should equal expected value")
			}
		})
	}
}

// TestDefaultParser_Parse_InvalidDecimalTypes tests Default Parser parse_ invalid decimal types scenarios.
func TestDefaultParser_Parse_InvalidDecimalTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"InvalidDecimal", "not_a_number", typeDecimal},
		{"InvalidNullDecimal", "abc", typeNullDecimal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
		})
	}
}

// TestDefaultParser_Parse_UnsupportedTypes tests Default Parser parse_ unsupported types scenarios.
func TestDefaultParser_Parse_UnsupportedTypes(t *testing.T) {
	parser := NewDefaultParser("")

	type CustomStruct struct {
		Field string
	}

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
	}{
		{"UnsupportedStruct", "data", reflect.TypeFor[CustomStruct]()},
		{"UnsupportedSlice", "[1,2,3]", reflect.TypeFor[[]int]()},
		{"UnsupportedMap", "{}", reflect.TypeFor[map[string]string]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.Error(t, err, "Should return error")
			assert.ErrorIs(t, err, ErrUnsupportedType, "Error should be ErrUnsupportedType")
		})
	}
}

// TestDefaultParser_Parse_EdgeCases tests Default Parser parse_ edge cases scenarios.
func TestDefaultParser_Parse_EdgeCases(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"ZeroInt", "0", reflect.TypeFor[int](), 0},
		{"ZeroFloat", "0.0", reflect.TypeFor[float64](), 0.0},
		{"NegativeInt", "-42", reflect.TypeFor[int](), -42},
		{"NegativeFloat", "-3.14", reflect.TypeFor[float64](), -3.14},
		{"LeadingZeros", "007", reflect.TypeFor[int](), 7},
		{"WhitespaceString", "  test  ", reflect.TypeFor[string](), "  test  "},
		{"MaxInt64", "9223372036854775807", reflect.TypeFor[int64](), int64(9223372036854775807)},
		{"MinInt64", "-9223372036854775808", reflect.TypeFor[int64](), int64(-9223372036854775808)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_UnicodeStrings tests Default Parser parse_ unicode strings scenarios.
func TestDefaultParser_Parse_UnicodeStrings(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		expected   any
	}{
		{"ChineseCharacters", "你好世界", reflect.TypeFor[string](), "你好世界"},
		{"EmojiCharacters", "👍🎉", reflect.TypeFor[string](), "👍🎉"},
		{"MixedUnicode", "Hello世界🌍", reflect.TypeFor[string](), "Hello世界🌍"},
		{"NullStringUnicode", "测试数据", typeNullString, null.StringFrom("测试数据")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_BooleanVariants tests Default Parser parse_ boolean variants scenarios.
func TestDefaultParser_Parse_BooleanVariants(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name      string
		cellValue string
		expected  bool
	}{
		{"True", "true", true},
		{"False", "false", false},
		{"One", "1", true},
		{"Zero", "0", false},
		{"TrueUpperCase", "True", true},
		{"TrueCapitalized", "TRUE", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, reflect.TypeFor[bool]())
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestDefaultParser_Parse_FloatPrecision tests Default Parser parse_ float precision scenarios.
func TestDefaultParser_Parse_FloatPrecision(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		cellValue  string
		targetType reflect.Type
		validate   func(*testing.T, any)
	}{
		{
			name:       "Float32Precision",
			cellValue:  "3.14159265359",
			targetType: reflect.TypeFor[float32](),
			validate: func(t *testing.T, result any) {
				f := result.(float32)
				assert.InDelta(t, 3.14159, f, 0.00001)
			},
		},
		{
			name:       "Float64Precision",
			cellValue:  "3.14159265359",
			targetType: reflect.TypeFor[float64](),
			validate: func(t *testing.T, result any) {
				f := result.(float64)
				assert.InDelta(t, 3.14159265359, f, 0.000000001)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.cellValue, tt.targetType)
			assert.NoError(t, err, "Should not return error")
			tt.validate(t, result)
		})
	}
}

// TestDefaultParser_NewDefaultParser tests Default Parser new default parser scenarios.
func TestDefaultParser_NewDefaultParser(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"EmptyFormat", ""},
		{"DateFormat", "2006-01-02"},
		{"TimeFormat", time.RFC3339},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewDefaultParser(tt.format)
			assert.NotNil(t, parser, "Should not be nil")
		})
	}
}

// TestDefaultParser_Parse_EmptyStringForPointer tests Default Parser parse_ empty string for pointer scenarios.
func TestDefaultParser_Parse_EmptyStringForPointer(t *testing.T) {
	parser := NewDefaultParser("")

	result, err := parser.Parse("", reflect.TypeFor[*string]())
	assert.NoError(t, err, "Should not return error")
	assert.Nil(t, result, "Should be nil")
}

// TestDefaultParser_Parse_EmptyStringForNullTypes tests Default Parser parse_ empty string for null types scenarios.
func TestDefaultParser_Parse_EmptyStringForNullTypes(t *testing.T) {
	parser := NewDefaultParser("")

	tests := []struct {
		name       string
		targetType reflect.Type
	}{
		{"NullString", typeNullString},
		{"NullInt", typeNullInt},
		{"NullFloat", typeNullFloat},
		{"NullBool", typeNullBool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse("", tt.targetType)
			assert.NoError(t, err, "Should not return error")
			assert.Equal(t, reflect.Zero(tt.targetType).Interface(), result, "Should equal expected value")
		})
	}
}
